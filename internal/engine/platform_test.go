package engine

import "testing"

func TestDefaultFileOpsAvailableBytes(t *testing.T) {
	bytes, err := defaultFileOps{}.AvailableBytes(t.TempDir())
	if err != nil {
		t.Fatalf("AvailableBytes() error = %v", err)
	}
	if bytes <= 0 {
		t.Fatalf("AvailableBytes() = %d, want positive", bytes)
	}
}

func TestRuntimeStorageMemorySnapshotHasRuntimeMemory(t *testing.T) {
	snapshot := runtimeStorageMemorySnapshot(0)
	if snapshot.RuntimeHeapAllocBytes <= 0 {
		t.Fatalf("RuntimeHeapAllocBytes = %d, want positive", snapshot.RuntimeHeapAllocBytes)
	}
	if snapshot.RuntimeRSSBytes <= 0 {
		t.Fatalf("RuntimeRSSBytes = %d, want positive fallback", snapshot.RuntimeRSSBytes)
	}
}
