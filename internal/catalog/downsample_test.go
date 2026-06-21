package catalog

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestCatalogDownsamplePoliciesPersistAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	cat, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	policy := model.DownsamplePolicy{
		Name:              "cpu_1m",
		SourceDatabase:    "metrics",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "metrics",
		TargetRetention:   "rp_1m",
		TargetMeasurement: "cpu",
		Interval:          time.Minute,
		Functions: []model.DownsampleFunction{
			{Function: "avg", Field: "usage", As: "avg_usage"},
		},
		Delay:           time.Minute,
		RefreshInterval: time.Minute,
		Lookback:        3 * time.Minute,
		Enabled:         true,
	}
	if err := cat.UpsertDownsamplePolicy(policy); err != nil {
		t.Fatalf("UpsertDownsamplePolicy() error = %v", err)
	}
	if err := cat.UpdateDownsampleWatermark(model.DownsampleWatermark{
		PolicyName:         "cpu_1m",
		CompletedUntilUnix: int64(time.Minute),
	}); err != nil {
		t.Fatalf("UpdateDownsampleWatermark() error = %v", err)
	}
	if err := cat.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(reopen) error = %v", err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Fatalf("Close(reopened) error = %v", err)
		}
	}()
	policies, err := reopened.ListDownsamplePolicies()
	if err != nil {
		t.Fatalf("ListDownsamplePolicies() error = %v", err)
	}
	if len(policies) != 1 || policies[0].Name != "cpu_1m" {
		t.Fatalf("policies = %#v, want cpu_1m", policies)
	}
	watermark, ok := reopened.DownsampleWatermark("cpu_1m")
	if !ok || watermark.CompletedUntilUnix != int64(time.Minute) {
		t.Fatalf("watermark = %#v ok=%v, want persisted watermark", watermark, ok)
	}
}

func TestCatalogDropDownsamplePolicyRemovesWatermark(t *testing.T) {
	cat, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	policy := model.DownsamplePolicy{
		Name:              "cpu_1m",
		SourceDatabase:    "metrics",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "metrics",
		TargetRetention:   "rp_1m",
		TargetMeasurement: "cpu",
		Interval:          time.Minute,
		Functions: []model.DownsampleFunction{{
			Function: "avg",
			Field:    "usage",
			As:       "avg_usage",
		}},
		RefreshInterval: time.Minute,
		Enabled:         true,
	}
	if err := cat.UpsertDownsamplePolicy(policy); err != nil {
		t.Fatalf("UpsertDownsamplePolicy() error = %v", err)
	}
	if err := cat.UpdateDownsampleWatermark(model.DownsampleWatermark{
		PolicyName:         "cpu_1m",
		CompletedUntilUnix: int64(time.Minute),
	}); err != nil {
		t.Fatalf("UpdateDownsampleWatermark() error = %v", err)
	}
	if err := cat.DropDownsamplePolicy("cpu_1m"); err != nil {
		t.Fatalf("DropDownsamplePolicy() error = %v", err)
	}
	policies, err := cat.ListDownsamplePolicies()
	if err != nil {
		t.Fatalf("ListDownsamplePolicies() error = %v", err)
	}
	if len(policies) != 0 {
		t.Fatalf("policies = %#v, want empty", policies)
	}
	if watermark, ok := cat.DownsampleWatermark("cpu_1m"); ok {
		t.Fatalf("watermark = %#v ok=true, want removed", watermark)
	}
}

func TestDecodeDownsampleMetadataRejectsCorruptPayload(t *testing.T) {
	cases := map[string][]byte{
		"bad_magic": []byte("bad"),
		"truncated": encodeDownsampleMetadata(map[string]model.DownsamplePolicy{
			"cpu_1m": {
				Name:              "cpu_1m",
				SourceDatabase:    "metrics",
				SourceRetention:   "autogen",
				SourceMeasurement: "cpu",
				TargetDatabase:    "metrics",
				TargetRetention:   "rp_1m",
				TargetMeasurement: "cpu",
				Interval:          time.Minute,
				Functions: []model.DownsampleFunction{{
					Function: "avg",
					Field:    "usage",
					As:       "avg_usage",
				}},
				RefreshInterval: time.Minute,
			},
		}, nil)[:12],
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := decodeDownsampleMetadata(data); err == nil {
				t.Fatal("decodeDownsampleMetadata() error = nil, want error")
			}
		})
	}
}

func TestDownsampleMetadataReadersRejectTruncatedInput(t *testing.T) {
	if _, _, err := readDownsamplePayload(newPayloadReader(nil)); err == nil {
		t.Fatal("readDownsamplePayload(empty) error = nil, want error")
	}
	if _, err := readDownsamplePolicy(newPayloadReader(nil)); err == nil {
		t.Fatal("readDownsamplePolicy(empty) error = nil, want error")
	}
	if _, err := readDownsampleFunctions(newPayloadReader(nil)); err == nil {
		t.Fatal("readDownsampleFunctions(empty) error = nil, want error")
	}
	if _, err := readDownsampleWatermark(newPayloadReader(nil)); err == nil {
		t.Fatal("readDownsampleWatermark(empty) error = nil, want error")
	}
	if _, err := readStringList(newPayloadReader(nil), "tags"); err == nil {
		t.Fatal("readStringList(empty) error = nil, want error")
	}
}

func TestLoadDownsampleMetadataRejectsUnreadablePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "downsample.bin"), 0700); err != nil {
		t.Fatalf("Mkdir(downsample.bin) error = %v", err)
	}
	cat := newCatalog(dir)
	if err := cat.loadDownsampleMetadata(); err == nil {
		t.Fatal("loadDownsampleMetadata(directory) error = nil, want error")
	}
}
