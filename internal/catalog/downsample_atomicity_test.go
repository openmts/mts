package catalog

import (
	"errors"
	"testing"
	"time"

	"github.com/openmts/mts/internal/faultinject"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/storagefs"
)

func TestDownsampleUpsertFailureKeepsCommittedState(t *testing.T) {
	for _, fault := range downsampleFaultCases() {
		t.Run(fault.name, func(t *testing.T) {
			cat, dir := openAtomicityCatalog(t)
			err := runDownsampleFault(fault, func() error {
				return cat.UpsertDownsamplePolicy(atomicityPolicy())
			})
			if err == nil {
				t.Fatal("UpsertDownsamplePolicy() error = nil, want fault")
			}
			assertDownsampleState(t, cat, 0, 0, false)
			assertReopenedDownsampleState(t, cat, dir, 0, 0, false)
		})
	}
}

func TestDownsampleDropFailureKeepsCommittedState(t *testing.T) {
	for _, fault := range downsampleFaultCases() {
		t.Run(fault.name, func(t *testing.T) {
			cat, dir := openAtomicityCatalog(t)
			seedAtomicityDownsample(t, cat)
			err := runDownsampleFault(fault, func() error {
				return cat.DropDownsamplePolicy("cpu_1m")
			})
			if err == nil {
				t.Fatal("DropDownsamplePolicy() error = nil, want fault")
			}
			assertDownsampleState(t, cat, 1, 10, true)
			assertReopenedDownsampleState(t, cat, dir, 1, 10, true)
		})
	}
}

func TestDownsampleWatermarkFailureKeepsCommittedState(t *testing.T) {
	for _, fault := range downsampleFaultCases() {
		t.Run(fault.name, func(t *testing.T) {
			cat, dir := openAtomicityCatalog(t)
			seedAtomicityDownsample(t, cat)
			err := runDownsampleFault(fault, func() error {
				return cat.UpdateDownsampleWatermark(model.DownsampleWatermark{
					PolicyName: "cpu_1m", CompletedUntilUnix: 20,
				})
			})
			if err == nil {
				t.Fatal("UpdateDownsampleWatermark() error = nil, want fault")
			}
			assertDownsampleState(t, cat, 1, 10, true)
			assertReopenedDownsampleState(t, cat, dir, 1, 10, true)
		})
	}
}

type downsampleFaultCase struct {
	name      string
	configure func(*faultinject.FS)
}

func downsampleFaultCases() []downsampleFaultCase {
	return []downsampleFaultCase{
		{name: "write", configure: failNextDownsampleOperation(faultinject.OpWrite)},
		{name: "sync_file", configure: failNextDownsampleOperation(faultinject.OpSync)},
		{name: "rename", configure: failNextDownsampleOperation(faultinject.OpRename)},
		{name: "sync_dir", configure: func(fs *faultinject.FS) {
			fs.FailNext(faultinject.OpSync, nil)
			fs.FailNext(faultinject.OpSync, errors.New("injected metadata failure"))
		}},
	}
}

func failNextDownsampleOperation(operation faultinject.Operation) func(*faultinject.FS) {
	return func(fs *faultinject.FS) {
		fs.FailNext(operation, errors.New("injected metadata failure"))
	}
}

func openAtomicityCatalog(t *testing.T) (*Catalog, string) {
	t.Helper()
	dir := t.TempDir()
	cat, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	return cat, dir
}

func runDownsampleFault(fault downsampleFaultCase, mutate func() error) error {
	fs := faultinject.NewFS()
	fault.configure(fs)
	restore := storagefs.SetFaultController(fs)
	err := mutate()
	restore()
	return err
}

func seedAtomicityDownsample(t *testing.T, cat *Catalog) {
	t.Helper()
	if err := cat.UpsertDownsamplePolicy(atomicityPolicy()); err != nil {
		t.Fatalf("UpsertDownsamplePolicy(seed) error = %v", err)
	}
	if err := cat.UpdateDownsampleWatermark(model.DownsampleWatermark{
		PolicyName: "cpu_1m", CompletedUntilUnix: 10,
	}); err != nil {
		t.Fatalf("UpdateDownsampleWatermark(seed) error = %v", err)
	}
}

func atomicityPolicy() model.DownsamplePolicy {
	return model.DownsamplePolicy{Name: "cpu_1m", Interval: time.Minute, Enabled: true}
}

func assertDownsampleState(
	t *testing.T,
	cat *Catalog,
	policyCount int,
	completed int64,
	wantWatermark bool,
) {
	t.Helper()
	policies, err := cat.ListDownsamplePolicies()
	if err != nil {
		t.Fatalf("ListDownsamplePolicies() error = %v", err)
	}
	if len(policies) != policyCount {
		t.Fatalf("policy count = %d, want %d", len(policies), policyCount)
	}
	watermark, ok := cat.DownsampleWatermark("cpu_1m")
	if ok != wantWatermark || ok && watermark.CompletedUntilUnix != completed {
		t.Fatalf("watermark = %#v ok=%v, want completed=%d ok=%v", watermark, ok, completed, wantWatermark)
	}
}

func assertReopenedDownsampleState(
	t *testing.T,
	cat *Catalog,
	dir string,
	policyCount int,
	completed int64,
	wantWatermark bool,
) {
	t.Helper()
	if err := cat.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(reopened) error = %v", err)
	}
	t.Cleanup(func() {
		if err := reopened.Close(); err != nil {
			t.Fatalf("Close(reopened) error = %v", err)
		}
	})
	assertDownsampleState(t, reopened, policyCount, completed, wantWatermark)
}
