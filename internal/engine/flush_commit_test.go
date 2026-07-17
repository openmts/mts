package engine

import (
	"errors"
	"testing"
	"time"

	"github.com/openmts/mts/internal/memtable"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/wal"
)

func TestFlushCheckpointHookFailureKeepsCommittedPartWithoutRestoringMem(t *testing.T) {
	shard := openShardForFlushCommitTest(t, 1)
	shard.testHooks.afterManifestBeforeWALTrunc = func() error {
		return errors.New("injected checkpoint failure")
	}

	err := shard.WriteBatch([]model.ResolvedPoint{testResolvedPoint(1, 10, 1)}, false)
	if err == nil {
		closeErr := shard.Close()
		t.Fatalf("WriteBatch() error = nil, want injected checkpoint failure close = %v", closeErr)
	}
	assertFlushCommitKeepsPartDropsMem(t, shard, 1)
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestFlushWALCheckpointFailureKeepsCommittedPartWithoutRestoringMem(t *testing.T) {
	walLog := &checkpointFailureWAL{err: errors.New("wal checkpoint failed")}
	mem := memtable.New()
	shard, _, err := OpenShard(ShardOptions{
		Dir:                t.TempDir(),
		Start:              0,
		End:                int64(time.Hour),
		MemTableMaxSamples: 10,
		deps: shardDeps{
			openWAL: func(string, model.WALOptions) (walStore, error) { return walLog, nil },
			newMem:  func() memStore { return memTableStore{inner: mem} },
		},
	})
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}

	if err := shard.WriteBatch([]model.ResolvedPoint{testResolvedPoint(1, 10, 1)}, false); err != nil {
		closeErr := shard.Close()
		t.Fatalf("WriteBatch() error = %v close = %v", err, closeErr)
	}
	err = shard.Flush()
	if err == nil {
		closeErr := shard.Close()
		t.Fatalf("Flush() error = nil, want wal checkpoint failure close = %v", closeErr)
	}
	if !walLog.checkpointCalled {
		closeErr := shard.Close()
		t.Fatalf("wal checkpoint called = false, want true close = %v", closeErr)
	}
	assertFlushCommitKeepsPartDropsMem(t, shard, 1)
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func openShardForFlushCommitTest(t *testing.T, maxSamples int) *Shard {
	t.Helper()
	shard, _, err := OpenShard(ShardOptions{
		Dir:                t.TempDir(),
		Start:              0,
		End:                int64(time.Hour),
		MemTableMaxSamples: maxSamples,
	})
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}
	return shard
}

func assertFlushCommitKeepsPartDropsMem(t *testing.T, shard *Shard, wantSamples int) {
	t.Helper()
	if got := shard.mem.SampleCount(); got != 0 {
		t.Fatalf("mem SampleCount() = %d, want 0 after committed flush failure", got)
	}
	if len(shard.manifest.Parts) != 1 {
		t.Fatalf("manifest parts = %d, want 1 committed part", len(shard.manifest.Parts))
	}
	if len(shard.parts) != 1 {
		t.Fatalf("open parts = %d, want 1 committed part", len(shard.parts))
	}
	got, queryErr := shard.Query(memtable.Query{Start: 0, End: 20})
	if queryErr != nil {
		t.Fatalf("Query() error = %v", queryErr)
	}
	total := 0
	for _, column := range got {
		total += len(column.Samples)
	}
	if total != wantSamples {
		t.Fatalf("query sample count = %d, want %d (no double materialization)", total, wantSamples)
	}
	report := shard.RecoveryReport()
	if len(report.Issues) == 0 {
		t.Fatal("RecoveryReport issues empty, want checkpoint failure recorded")
	}
}

type checkpointFailureWAL struct {
	points           []model.ResolvedPoint
	tombstones       []model.Tombstone
	checkpointCalled bool
	err              error
}

func (w *checkpointFailureWAL) Append(points []model.ResolvedPoint, _ bool) error {
	w.points = append(w.points, points...)
	return nil
}

func (w *checkpointFailureWAL) AppendTombstones(tombstones []model.Tombstone, _ bool) error {
	w.tombstones = append(w.tombstones, tombstones...)
	return nil
}

func (w *checkpointFailureWAL) ReplayRecords() ([]wal.Record, error) {
	return nil, nil
}

func (w *checkpointFailureWAL) ApproxMemoryBytes() int64 {
	return int64(len(w.points)*64 + len(w.tombstones)*32)
}

func (w *checkpointFailureWAL) Checkpoint() error {
	w.checkpointCalled = true
	return w.err
}

func (w *checkpointFailureWAL) Close() error {
	return nil
}
