package engine

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openmts/mts/internal/memtable"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/sstable"
)

func TestFlushManifestFailureRemovesUncommittedPartAndKeepsWAL(t *testing.T) {
	mem := &fakeMemStore{
		snapshot: &fakeMemSnapshot{
			columns: []model.ColumnData{columnForMergeTest(1, 1, 0, 1)},
			samples: 1,
		},
		samples: 1,
	}
	walLog := &fakeWalStore{}
	parts := &manifestFailurePartManager{err: errors.New("manifest failed")}
	files := &fakeFileOps{}
	shard, _, err := OpenShard(ShardOptions{
		Dir:                t.TempDir(),
		Start:              0,
		End:                100,
		MemTableMaxSamples: 10,
		deps: shardDeps{
			openWAL: func(string, model.WALOptions) (walStore, error) { return walLog, nil },
			newMem:  func() memStore { return mem },
			parts:   parts,
			files:   files,
		},
	})
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}

	err = shard.Flush()
	if err == nil {
		closeErr := shard.Close()
		t.Fatalf("Flush() error = nil, want manifest failure close = %v", closeErr)
	}
	if walLog.checkpointCalled {
		closeErr := shard.Close()
		t.Fatalf("wal checkpoint called after failed manifest close = %v", closeErr)
	}
	if len(files.paths) != 1 || files.paths[0] != parts.meta.Path {
		closeErr := shard.Close()
		t.Fatalf("removed paths = %v, want %q close = %v", files.paths, parts.meta.Path, closeErr)
	}
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestCompactionManifestFailureRemovesOutputAndKeepsInputsQueryable(t *testing.T) {
	shard := openShardWithTwoFlushedParts(t)
	shard.deps.parts = writeManifestFailurePartManager{err: errors.New("manifest failed")}

	err := shard.Compact()
	if err == nil {
		closeErr := shard.Close()
		t.Fatalf("Compact() error = nil, want manifest failure close = %v", closeErr)
	}
	outputPath := filepath.Join(shard.opts.Dir, "sst-000003")
	if _, statErr := os.Stat(outputPath); !errors.Is(statErr, os.ErrNotExist) {
		closeErr := shard.Close()
		t.Fatalf("output stat = %v, want not exist close = %v", statErr, closeErr)
	}
	assertShardQuerySampleCount(t, shard, 2)
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestCompactionOldPartDeleteFailureKeepsNewManifestAndRecordsMaintenance(t *testing.T) {
	shard := openShardWithTwoFlushedParts(t)
	removeErr := errors.New("remove old part failed")
	files := &fakeFileOps{err: removeErr}
	shard.deps.files = files

	err := shard.Compact()
	if err != nil {
		closeErr := shard.Close()
		t.Fatalf("Compact() error = %v, want nil after manifest switch close = %v", err, closeErr)
	}
	if len(shard.manifest.Parts) != 1 || shard.manifest.Parts[0].ID != "sst-000003" {
		closeErr := shard.Close()
		t.Fatalf("manifest parts = %#v, want compacted output close = %v", shard.manifest.Parts, closeErr)
	}
	var issue *RecoveryIssue
	if !errors.As(shard.maintenanceErr, &issue) || issue.Kind != RecoveryIssueOrphanRemoveFailed {
		closeErr := shard.Close()
		t.Fatalf("maintenanceErr = %v, want old part cleanup issue close = %v", shard.maintenanceErr, closeErr)
	}
	if len(files.paths) == 0 {
		closeErr := shard.Close()
		t.Fatalf("RemoveAll paths = none, want old part removals close = %v", closeErr)
	}
	assertShardQuerySampleCount(t, shard, 2)
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func openShardWithTwoFlushedParts(t *testing.T) *Shard {
	t.Helper()
	shard, _, err := OpenShard(ShardOptions{
		Dir:                t.TempDir(),
		Start:              0,
		End:                int64(time.Hour),
		MemTableMaxSamples: 1,
	})
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}
	for _, point := range []model.ResolvedPoint{
		testResolvedPoint(1, 10, 1),
		testResolvedPoint(1, 20, 2),
	} {
		if err := shard.Write(point, true); err != nil {
			closeErr := shard.Close()
			t.Fatalf("Write() error = %v close = %v", err, closeErr)
		}
	}
	return shard
}

func assertShardQuerySampleCount(t *testing.T, shard *Shard, want int) {
	t.Helper()
	got, err := shard.Query(memtable.Query{Start: 0, End: 100})
	if err != nil {
		closeErr := shard.Close()
		t.Fatalf("Query() error = %v close = %v", err, closeErr)
	}
	if len(got) != 1 || len(got[0].Samples) != want {
		closeErr := shard.Close()
		t.Fatalf("query result = %#v, want one column with %d samples close = %v", got, want, closeErr)
	}
}

type manifestFailurePartManager struct {
	meta sstable.PartMeta
	err  error
}

func (m *manifestFailurePartManager) LoadManifest(string) (sstable.Manifest, error) {
	return sstable.Manifest{}, nil
}

func (m *manifestFailurePartManager) WriteManifest(string, sstable.Manifest) error {
	return m.err
}

func (m *manifestFailurePartManager) OpenPart(string) (partReader, error) {
	return fakePartReader{meta: m.meta}, nil
}

func (m *manifestFailurePartManager) OpenPartTrusted(string) (partReader, error) {
	return m.OpenPart("")
}

func (m *manifestFailurePartManager) WritePart(
	root string,
	level int,
	id string,
	columns []model.ColumnData,
	_ sstable.WriteOptions,
) (sstable.PartMeta, error) {
	m.meta = sstable.PartMeta{
		ID:          id,
		Level:       level,
		Path:        filepath.Join(root, id),
		MinTime:     columns[0].Samples[0].Timestamp,
		MaxTime:     columns[0].Samples[0].Timestamp,
		MinSeriesID: columns[0].SeriesID,
		MaxSeriesID: columns[0].SeriesID,
		RowsCount:   len(columns[0].Samples),
		SeriesCount: 1,
		BlockCount:  1,
		MaxWriteSeq: columns[0].Samples[0].WriteSeq,
	}
	return m.meta, nil
}

func (m *manifestFailurePartManager) NewWriter(root string, level int, id string, _ sstable.WriteOptions) (partWriter, error) {
	return &fakePartWriter{
		level: level,
		id:    id,
		path:  filepath.Join(root, id),
		onClose: func(meta sstable.PartMeta) {
			m.meta = meta
		},
	}, nil
}

func (m *manifestFailurePartManager) NewSeriesBatchReader(partReader, sstable.Query) (seriesBatchReader, error) {
	return fakeSeriesBatchReader{}, nil
}

type writeManifestFailurePartManager struct {
	defaultPartManager
	err error
}

func (m writeManifestFailurePartManager) WriteManifest(string, sstable.Manifest) error {
	return m.err
}
