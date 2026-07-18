package engine

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openmts/mts/internal/memtable"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryexec"
	"github.com/openmts/mts/internal/wal"
)

func TestWriteBatchMemApplyFailureAfterWALRetriesAndSurfacesError(t *testing.T) {
	walLog := &recordingWAL{}
	mem := &failOnceMemStore{failTimes: 1}
	shard, _, err := OpenShard(ShardOptions{
		Dir:                t.TempDir(),
		Start:              0,
		End:                int64(time.Hour),
		MemTableMaxSamples: 100,
		deps: shardDeps{
			openWAL: func(string, model.WALOptions) (walStore, error) { return walLog, nil },
			newMem:  func() memStore { return mem },
		},
	})
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}

	err = shard.WriteBatch([]model.ResolvedPoint{testResolvedPoint(1, 10, 1)}, false)
	if err != nil {
		closeErr := shard.Close()
		t.Fatalf("WriteBatch() error = %v, want success after single retry close = %v", err, closeErr)
	}
	if walLog.appendCalls != 1 {
		closeErr := shard.Close()
		t.Fatalf("wal append calls = %d, want 1 close = %v", walLog.appendCalls, closeErr)
	}
	if mem.applyCalls != 2 {
		closeErr := shard.Close()
		t.Fatalf("mem apply calls = %d, want 2 (fail once + retry) close = %v", mem.applyCalls, closeErr)
	}
	if mem.samples != 1 {
		closeErr := shard.Close()
		t.Fatalf("mem samples = %d, want 1 visible after retry close = %v", mem.samples, closeErr)
	}
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestWriteBatchMemApplyPersistentFailureKeepsWALDurable(t *testing.T) {
	walLog := &recordingWAL{}
	mem := &failOnceMemStore{failTimes: 100}
	shard, _, err := OpenShard(ShardOptions{
		Dir:                t.TempDir(),
		Start:              0,
		End:                int64(time.Hour),
		MemTableMaxSamples: 100,
		deps: shardDeps{
			openWAL: func(string, model.WALOptions) (walStore, error) { return walLog, nil },
			newMem:  func() memStore { return mem },
		},
	})
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}

	err = shard.WriteBatch([]model.ResolvedPoint{testResolvedPoint(1, 10, 1)}, false)
	if err == nil {
		closeErr := shard.Close()
		t.Fatalf("WriteBatch() error = nil, want mem apply failure close = %v", closeErr)
	}
	if walLog.appendCalls != 1 {
		closeErr := shard.Close()
		t.Fatalf("wal append calls = %d, want 1 durable append close = %v", walLog.appendCalls, closeErr)
	}
	if len(walLog.points) != 1 {
		closeErr := shard.Close()
		t.Fatalf("wal points = %d, want 1 close = %v", len(walLog.points), closeErr)
	}
	if mem.applyCalls < 2 {
		closeErr := shard.Close()
		t.Fatalf("mem apply calls = %d, want >=2 retries close = %v", mem.applyCalls, closeErr)
	}
	report := shard.RecoveryReport()
	if len(report.Issues) == 0 {
		closeErr := shard.Close()
		t.Fatalf("RecoveryReport empty, want mem apply failure recorded close = %v", closeErr)
	}
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

type recordingWAL struct {
	points      []model.ResolvedPoint
	appendCalls int
}

func (w *recordingWAL) Append(points []model.ResolvedPoint, _ bool) error {
	w.appendCalls++
	w.points = append(w.points, points...)
	return nil
}

func (w *recordingWAL) AppendTombstones([]model.Tombstone, bool) error { return nil }

func (w *recordingWAL) ReplayRecords() ([]wal.Record, error) { return nil, nil }

func (w *recordingWAL) ApproxMemoryBytes() int64 { return int64(len(w.points) * 64) }

func (w *recordingWAL) Checkpoint() error { return nil }

func (w *recordingWAL) Close() error { return nil }

type failOnceMemStore struct {
	failTimes  int32
	applyCalls int32
	samples    int
}

func (m *failOnceMemStore) Apply(point model.ResolvedPoint) error {
	return m.ApplyBatch([]model.ResolvedPoint{point})
}

func (m *failOnceMemStore) ApplyBatch(points []model.ResolvedPoint) error {
	call := atomic.AddInt32(&m.applyCalls, 1)
	if call <= m.failTimes {
		return errors.New("injected mem apply failure")
	}
	for _, point := range points {
		m.samples += len(point.Fields)
	}
	return nil
}

func (m *failOnceMemStore) SampleCount() int { return m.samples }

func (m *failOnceMemStore) ApproxMemorySamples() int { return m.samples }

func (m *failOnceMemStore) ApproxMemoryBytes() int64 { return int64(m.samples * 32) }

func (m *failOnceMemStore) DisorderRatio() float64 { return 0 }

func (m *failOnceMemStore) AppendedSamples() uint64 { return uint64(m.samples) }

func (m *failOnceMemStore) SnapshotAndReset() memSnapshot {
	return &fakeMemSnapshot{}
}

func (m *failOnceMemStore) Snapshot() memSnapshot { return &fakeMemSnapshot{} }

func (m *failOnceMemStore) Query(memtable.Query) []model.ColumnData {
	return nil
}

func (m *failOnceMemStore) ScanColumns(memtable.Query) queryexec.ColumnDataStream {
	return queryexec.NewSliceColumnDataStream(nil)
}

func (m *failOnceMemStore) Restore(memSnapshot) {}
