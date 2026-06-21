package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/openmts/mts/internal/faultinject"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/storagefs"
)

func TestRunDownsamplePolicyRecordsFailureWithoutAdvancingWatermark(t *testing.T) {
	ctx := context.Background()
	eng := openEngineWithRawDownsampleSamples(t)
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	if err := eng.CreateDownsamplePolicy(ctx, testDownsamplePolicy()); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	fs := faultinject.NewFS()
	fs.FailNext(faultinject.OpWrite, errors.New("injected"))
	restore := storagefs.SetFaultController(fs)
	result, err := eng.RunDownsamplePolicy(ctx, "cpu_1m", 2*time.Minute)
	restore()
	if err == nil || !strings.Contains(err.Error(), "fault write") {
		t.Fatalf("RunDownsamplePolicy() error = %v, want write fault", err)
	}
	if result.CompletedUntilUnix != 0 {
		t.Fatalf("CompletedUntilUnix = %d, want not advanced", result.CompletedUntilUnix)
	}
	watermark, ok, err := eng.metadata.DownsampleWatermark(ctx, "cpu_1m")
	if err != nil || !ok {
		t.Fatalf("DownsampleWatermark() = %#v ok=%v err=%v", watermark, ok, err)
	}
	if watermark.CompletedUntilUnix != 0 || watermark.LastError == "" {
		t.Fatalf("watermark = %#v, want preserved completion and error", watermark)
	}
	stats := eng.DownsampleStatsSnapshot()
	if stats.Failure != 1 || stats.LastError == "" {
		t.Fatalf("downsample stats = %#v, want failure", stats)
	}
}

func TestRunDownsamplePolicyMetadataFailureDoesNotAdvanceWatermark(t *testing.T) {
	ctx := context.Background()
	eng := openEngineWithRawDownsampleSamples(t)
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	if err := eng.CreateDownsamplePolicy(ctx, testDownsamplePolicy()); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	fs := faultinject.NewFS()
	fs.FailNext(faultinject.OpRename, errors.New("metadata rename failed"))
	restore := storagefs.SetFaultController(fs)
	result, err := eng.RunDownsamplePolicy(ctx, "cpu_1m", time.Minute)
	restore()
	if err == nil || !strings.Contains(err.Error(), "fault rename") {
		t.Fatalf("RunDownsamplePolicy() error = %v, want metadata rename fault", err)
	}
	watermark, ok, err := eng.metadata.DownsampleWatermark(ctx, "cpu_1m")
	if err != nil {
		t.Fatalf("DownsampleWatermark() error = %v", err)
	}
	if ok && watermark.CompletedUntilUnix != 0 {
		t.Fatalf("watermark = %#v, want not advanced after metadata failure", watermark)
	}
	if result.CompletedUntilUnix != 0 {
		t.Fatalf("result = %#v, want completion not advanced", result)
	}
}

func TestDownsampleValidationRejectsInvalidFunctionDetails(t *testing.T) {
	valid := testDownsamplePolicy()
	cases := map[string]model.DownsamplePolicy{
		"negative_delay": func() model.DownsamplePolicy {
			policy := valid
			policy.Delay = -time.Second
			return policy
		}(),
		"zero_refresh": func() model.DownsamplePolicy {
			policy := valid
			policy.RefreshInterval = 0
			return policy
		}(),
		"negative_lookback": func() model.DownsamplePolicy {
			policy := valid
			policy.Lookback = -time.Second
			return policy
		}(),
		"unsupported_function": func() model.DownsamplePolicy {
			policy := valid
			policy.Functions = []model.DownsampleFunction{{Function: "quantile", Field: "usage"}}
			return policy
		}(),
		"duplicate_output": func() model.DownsamplePolicy {
			policy := valid
			policy.Functions = []model.DownsampleFunction{
				{Function: "avg", Field: "usage", As: "x"},
				{Function: "max", Field: "usage", As: "x"},
			}
			return policy
		}(),
	}
	for name, policy := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := normalizeDownsamplePolicy(policy); err == nil {
				t.Fatal("normalizeDownsamplePolicy() error = nil, want error")
			}
		})
	}
}

func TestDownsampleColumnConversionRejectsBadColumns(t *testing.T) {
	policy := mustNormalizeDownsamplePolicyForTest(t, testDownsamplePolicy())
	if _, err := downsampleColumnsToPoints(policy, 0, int64(time.Minute), []model.ColumnSeries{{
		FieldName:  "unknown(usage)",
		Timestamps: []int64{0},
		Values:     []model.FieldValue{model.Float64Value(1)},
	}}); err == nil {
		t.Fatal("downsampleColumnsToPoints(unknown) error = nil, want error")
	}
	if _, err := downsampleColumnsToPoints(policy, 0, int64(time.Minute), []model.ColumnSeries{{
		FieldName:  "avg(usage)",
		Timestamps: []int64{0},
	}}); err == nil {
		t.Fatal("downsampleColumnsToPoints(mismatch) error = nil, want error")
	}
}
