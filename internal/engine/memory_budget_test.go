package engine

import (
	"errors"
	"testing"

	"codeberg.org/mts/mts/internal/model"
)

func TestNormalizeStorageMemoryOptionsBytes(t *testing.T) {
	got := normalizeStorageMemoryOptions(model.StorageMemoryOptions{
		SoftSampleLimit:       -1,
		HardSampleLimit:       -2,
		SoftBytesLimit:        300,
		HardBytesLimit:        200,
		QueryBytesLimit:       -3,
		FlushBytesLimit:       -4,
		CompactionBytesLimit:  -5,
		CompressionBytesLimit: -6,
	})
	if got.SoftSampleLimit != 0 || got.HardSampleLimit != 0 {
		t.Fatalf("sample limits = soft %d hard %d, want zero", got.SoftSampleLimit, got.HardSampleLimit)
	}
	if got.SoftBytesLimit != 200 || got.HardBytesLimit != 200 {
		t.Fatalf("byte limits = soft %d hard %d, want both 200", got.SoftBytesLimit, got.HardBytesLimit)
	}
	if got.QueryBytesLimit != 0 || got.FlushBytesLimit != 0 ||
		got.CompactionBytesLimit != 0 || got.CompressionBytesLimit != 0 {
		t.Fatalf("operation byte limits = query %d flush %d compaction %d compression %d, want zero",
			got.QueryBytesLimit,
			got.FlushBytesLimit,
			got.CompactionBytesLimit,
			got.CompressionBytesLimit,
		)
	}
}

func TestStorageMemoryLimiterReserveReleaseAndErrors(t *testing.T) {
	limiter := newStorageMemoryLimiter(model.StorageMemoryOptions{
		HardBytesLimit:  100,
		QueryBytesLimit: 40,
	})
	release, err := limiter.Reserve(storageMemoryQuery, 10, 30)
	if err != nil {
		t.Fatalf("Reserve(query 30) error = %v", err)
	}
	snapshot := limiter.Snapshot(storageMemoryActive{MemTableBytes: 10})
	if snapshot.ReservationBytes != 30 || snapshot.CurrentBytes != 40 || snapshot.PeakBytes != 40 {
		t.Fatalf("snapshot after reserve = %#v, want reservation 30 current 40 peak 40", snapshot)
	}
	if _, err := limiter.Reserve(storageMemoryQuery, 10, 11); !errors.Is(err, ErrStorageMemoryLimitExceeded) {
		t.Fatalf("Reserve(query over operation limit) error = %v, want ErrStorageMemoryLimitExceeded", err)
	}
	release()
	snapshot = limiter.Snapshot(storageMemoryActive{MemTableBytes: 10})
	if snapshot.ReservationBytes != 0 || snapshot.CurrentBytes != 10 {
		t.Fatalf("snapshot after release = %#v, want reservation 0 current 10", snapshot)
	}
	if _, err := limiter.Reserve(storageMemoryFlush, 10, 91); !errors.Is(err, ErrStorageMemoryLimitExceeded) {
		t.Fatalf("Reserve(flush over hard limit) error = %v, want ErrStorageMemoryLimitExceeded", err)
	}
}

func TestStorageMemoryLimiterSnapshotBreaksDownSources(t *testing.T) {
	limiter := newStorageMemoryLimiter(model.StorageMemoryOptions{
		HardBytesLimit:        1024,
		CompressionBytesLimit: 128,
	})
	writeRelease, err := limiter.Reserve(storageMemoryWrite, 100, 30)
	if err != nil {
		t.Fatalf("Reserve(write) error = %v", err)
	}
	compressionRelease, err := limiter.Reserve(storageMemoryCompression, 100, 40)
	if err != nil {
		t.Fatalf("Reserve(compression) error = %v", err)
	}
	snapshot := limiter.Snapshot(storageMemoryActive{
		MemTableBytes: 100,
		WALBytes:      20,
	})
	if snapshot.ActiveBytes != 120 || snapshot.MemTableBytes != 100 || snapshot.WALBytes != 20 {
		t.Fatalf("active breakdown = %#v, want active 120 memtable 100 wal 20", snapshot)
	}
	if snapshot.WriteBytes != 30 || snapshot.CompressionBytes != 40 || snapshot.ReservationBytes != 70 {
		t.Fatalf("reservation breakdown = %#v, want write 30 compression 40 reservation 70", snapshot)
	}
	if snapshot.CurrentBytes != 190 || snapshot.PeakBytes != 190 {
		t.Fatalf("snapshot current/peak = %#v, want 190", snapshot)
	}
	compressionRelease()
	writeRelease()
	snapshot = limiter.Snapshot(storageMemoryActive{MemTableBytes: 100, WALBytes: 20})
	if snapshot.ReservationBytes != 0 || snapshot.CurrentBytes != 120 {
		t.Fatalf("snapshot after release = %#v, want reservation 0 current 120", snapshot)
	}
}

func TestStorageMemoryLimiterRejectsCompressionBudget(t *testing.T) {
	limiter := newStorageMemoryLimiter(model.StorageMemoryOptions{
		HardBytesLimit:        1024,
		CompressionBytesLimit: 8,
	})
	if _, err := limiter.Reserve(storageMemoryCompression, 0, 9); !errors.Is(err, ErrStorageMemoryLimitExceeded) {
		t.Fatalf("Reserve(compression) error = %v, want ErrStorageMemoryLimitExceeded", err)
	}
	snapshot := limiter.Snapshot(storageMemoryActive{})
	if snapshot.RejectedWrites != 0 || snapshot.RejectedReservations == 0 {
		t.Fatalf("snapshot rejects = %#v, want reservation reject without write reject", snapshot)
	}
}
