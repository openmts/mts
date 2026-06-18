package engine

import (
	"errors"
	"testing"
	"time"

	"github.com/openmts/mts/internal/faultinject"
	"github.com/openmts/mts/internal/memtable"
	"github.com/openmts/mts/internal/storagefs"
)

func TestShardWriteWALAppendFailureDoesNotApplyMemTable(t *testing.T) {
	assertRejectedWALWriteDoesNotApplyMemTable(t, faultinject.OpWrite)
}

func TestShardSyncWriteFailureDoesNotApplyMemTable(t *testing.T) {
	assertRejectedWALWriteDoesNotApplyMemTable(t, faultinject.OpSync)
}

func assertRejectedWALWriteDoesNotApplyMemTable(t *testing.T, op faultinject.Operation) {
	t.Helper()
	shard, _, err := OpenShard(ShardOptions{
		Dir:                t.TempDir(),
		Start:              0,
		End:                int64(time.Hour),
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}
	fs := faultinject.NewFS()
	fs.FailNext(op, errors.New("wal fault"))
	restore := storagefs.SetFaultController(fs)
	err = shard.Write(testResolvedPoint(1, 10, 1), true)
	restore()
	if err == nil {
		closeErr := shard.Close()
		t.Fatalf("Write() error = nil, want WAL %s fault close = %v", op, closeErr)
	}
	got, queryErr := shard.Query(memtable.Query{Start: 0, End: 20})
	if queryErr != nil {
		closeErr := shard.Close()
		t.Fatalf("Query() error = %v close = %v", queryErr, closeErr)
	}
	if len(got) != 0 {
		closeErr := shard.Close()
		t.Fatalf("memtable query count = %d, want 0 after rejected WAL write close = %v", len(got), closeErr)
	}
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
