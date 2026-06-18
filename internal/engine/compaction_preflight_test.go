package engine

import (
	"errors"
	"testing"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/sstable"
)

func TestCompactionRejectsWhenDiskSpaceInsufficient(t *testing.T) {
	shard := openShardWithTwoFlushedParts(t)
	shard.opts.Compaction.MinFreeBytes = 1
	shard.deps.files = &fakeFileOps{availableBytes: 0, availableSet: true}
	err := shard.Compact()
	if !errors.Is(err, ErrCompactionDiskSpaceExceeded) {
		closeErr := shard.Close()
		t.Fatalf("Compact() error = %v, want ErrCompactionDiskSpaceExceeded close = %v", err, closeErr)
	}
	assertShardQuerySampleCount(t, shard, 2)
	if len(shard.manifest.Parts) != 2 {
		closeErr := shard.Close()
		t.Fatalf("manifest part count = %d, want unchanged 2 close = %v", len(shard.manifest.Parts), closeErr)
	}
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestCompactionUsesTargetLevelCompressionAndSplitLimit(t *testing.T) {
	manager := &recordingPartManager{}
	shard, _, err := OpenShard(ShardOptions{
		Dir: t.TempDir(),
		deps: shardDeps{
			parts: manager,
		},
	})
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}
	output := newCompactionOutput(shard, 2, compactionOutputOptions{
		maxOutputPartBytes: 80,
		compression: model.CompressionOptions{
			Enabled:       true,
			Algorithm:     "zstd",
			MinPageValues: 1,
		},
	})
	if err := output.addSeries([]model.ColumnData{
		wideColumnForCompactionOutputTest(1, 1, "alpha"),
		wideColumnForCompactionOutputTest(1, 2, "beta"),
	}); err != nil {
		closeErr := shard.Close()
		t.Fatalf("addSeries() error = %v close = %v", err, closeErr)
	}
	parts, metas, err := output.close()
	if err != nil {
		closeErr := shard.Close()
		t.Fatalf("close() error = %v close = %v", err, closeErr)
	}
	if len(metas) < 2 {
		closeErr := errors.Join(closeParts(parts), shard.Close())
		t.Fatalf("output metas = %d, want split output close = %v", len(metas), closeErr)
	}
	for _, opts := range manager.writeOptions {
		if !opts.Compression.Enabled || opts.Compression.Algorithm != "zstd" {
			closeErr := errors.Join(closeParts(parts), shard.Close())
			t.Fatalf("writer compression = %#v, want zstd close = %v", opts.Compression, closeErr)
		}
	}
	if err := errors.Join(closeParts(parts), shard.Close()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestCompactionRecordsSafeDeleteAfterManifestCommit(t *testing.T) {
	shard := openShardWithTwoFlushedParts(t)
	if err := shard.Compact(); err != nil {
		closeErr := shard.Close()
		t.Fatalf("Compact() error = %v close = %v", err, closeErr)
	}
	stats := shard.CompactionStatsSnapshot()
	if stats.SafeDeleteParts != 2 {
		closeErr := shard.Close()
		t.Fatalf("SafeDeleteParts = %d, want 2 close = %v", stats.SafeDeleteParts, closeErr)
	}
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

type recordingPartManager struct {
	defaultPartManager
	writeOptions []sstable.WriteOptions
	nextMeta     sstable.PartMeta
}

func (m *recordingPartManager) NewWriter(
	root string,
	level int,
	id string,
	opts sstable.WriteOptions,
) (partWriter, error) {
	m.writeOptions = append(m.writeOptions, opts)
	m.nextMeta = sstable.PartMeta{ID: id, Level: level, Path: root + "/" + id, BlockCount: 1}
	return &recordingPartWriter{meta: m.nextMeta}, nil
}

func (m *recordingPartManager) OpenPart(string) (partReader, error) {
	return fakePartReader{meta: m.nextMeta}, nil
}

type recordingPartWriter struct {
	meta sstable.PartMeta
	rows int
}

func (w *recordingPartWriter) AddSeries(columns []model.ColumnData) error {
	w.rows += countColumnSamples(columns)
	return nil
}

func (w *recordingPartWriter) Close() (sstable.PartMeta, error) {
	w.meta.RowsCount = w.rows
	return w.meta, nil
}

func (w *recordingPartWriter) Abort() error {
	return nil
}
