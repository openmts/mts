package engine

import (
	"fmt"
	"path/filepath"

	"github.com/openmts/mts/internal/memtable"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryexec"
	"github.com/openmts/mts/internal/sstable"
	"github.com/openmts/mts/internal/storagefs"
	"github.com/openmts/mts/internal/wal"
)

type walStore interface {
	Append(records []model.ResolvedPoint, syncWrite bool) error
	AppendTombstones(tombstones []model.Tombstone, syncWrite bool) error
	ReplayRecords() ([]wal.Record, error)
	ApproxMemoryBytes() int64
	Checkpoint() error
	Close() error
}

type typedWalStore interface {
	AppendTyped(batch model.ResolvedTypedBatch, rows []int, syncWrite bool) error
}

type walMetricsProvider interface {
	MetricsSnapshot() wal.Metrics
}

type memStore interface {
	Apply(point model.ResolvedPoint) error
	ApplyBatch(points []model.ResolvedPoint) error
	SampleCount() int
	ApproxMemorySamples() int
	ApproxMemoryBytes() int64
	SnapshotAndReset() memSnapshot
	Snapshot() memSnapshot
	Query(query memtable.Query) []model.ColumnData
	ScanColumns(query memtable.Query) queryexec.ColumnDataStream
	Restore(snapshot memSnapshot)
}

type typedMemStore interface {
	ApplyTypedBatch(batch model.ResolvedTypedBatch, rows []int) error
}

type memSnapshot interface {
	Columns(query memtable.Query) []model.ColumnData
	ForEachSeries(query memtable.Query, fn func(uint64, []model.ColumnData) error) error
	Query(query memtable.Query) []model.ColumnData
	SampleCount() int
	ApproxMemoryBytes() int64
	Release()
}

type partReader interface {
	Close() error
	Meta() sstable.PartMeta
	Query(query sstable.Query) ([]model.ColumnData, error)
	ScanColumns(query sstable.Query) (queryexec.ColumnDataStream, error)
	QuerySeriesIDs(query sstable.Query, seriesIDs []uint64) ([]model.ColumnData, error)
	SeriesIDs(query sstable.Query) ([]uint64, error)
}

type partWriter interface {
	AddSeries(columns []model.ColumnData) error
	Close() (sstable.PartMeta, error)
	Abort() error
}

type seriesBatchReader interface {
	SeriesIDs() []uint64
	SeriesCount() int
	AppendSeriesIDs(dst []uint64) []uint64
	QuerySeriesIDs(seriesIDs []uint64) ([]model.ColumnData, error)
	QuerySeriesID(seriesID uint64) ([]model.ColumnData, error)
}

type partManager interface {
	LoadManifest(dir string) (sstable.Manifest, error)
	WriteManifest(dir string, manifest sstable.Manifest) error
	OpenPart(path string) (partReader, error)
	OpenPartTrusted(path string) (partReader, error)
	WritePart(
		root string,
		level int,
		id string,
		columns []model.ColumnData,
		opts sstable.WriteOptions,
	) (sstable.PartMeta, error)
	NewWriter(root string, level int, id string, opts sstable.WriteOptions) (partWriter, error)
	NewSeriesBatchReader(part partReader, query sstable.Query) (seriesBatchReader, error)
}

type fileOps interface {
	RemoveAll(path string) error
	AvailableBytes(path string) (int64, error)
}

type shardDeps struct {
	openWAL func(dir string, opts model.WALOptions) (walStore, error)
	newMem  func() memStore
	parts   partManager
	files   fileOps
}

type defaultPartManager struct{}

type defaultFileOps struct{}

type memTableStore struct {
	inner *memtable.MemTable
}

func normalizeShardDeps(deps shardDeps) shardDeps {
	if deps.openWAL == nil {
		deps.openWAL = func(dir string, opts model.WALOptions) (walStore, error) {
			return wal.Open(filepath.Join(dir, "wal"), wal.Options(opts))
		}
	}
	if deps.newMem == nil {
		deps.newMem = func() memStore {
			return memTableStore{inner: memtable.New()}
		}
	}
	if deps.parts == nil {
		deps.parts = defaultPartManager{}
	}
	if deps.files == nil {
		deps.files = defaultFileOps{}
	}
	return deps
}

func (defaultPartManager) LoadManifest(dir string) (sstable.Manifest, error) {
	return sstable.LoadManifest(dir)
}

func (defaultPartManager) WriteManifest(dir string, manifest sstable.Manifest) error {
	return sstable.WriteManifest(dir, manifest)
}

func (defaultPartManager) OpenPart(path string) (partReader, error) {
	return sstable.OpenPart(path)
}

func (defaultPartManager) OpenPartTrusted(path string) (partReader, error) {
	return sstable.OpenPartTrusted(path)
}

func (defaultPartManager) WritePart(
	root string,
	level int,
	id string,
	columns []model.ColumnData,
	opts sstable.WriteOptions,
) (sstable.PartMeta, error) {
	return sstable.WritePartWithOptions(root, level, id, columns, opts)
}

func (defaultPartManager) NewWriter(
	root string,
	level int,
	id string,
	opts sstable.WriteOptions,
) (partWriter, error) {
	return sstable.NewPartWriter(root, level, id, opts)
}

func (defaultPartManager) NewSeriesBatchReader(
	reader partReader,
	query sstable.Query,
) (seriesBatchReader, error) {
	part, ok := reader.(*sstable.Part)
	if !ok {
		return nil, fmt.Errorf("part reader does not support batch reader")
	}
	return sstable.NewSeriesBatchReader(part, query)
}

func (defaultFileOps) RemoveAll(path string) error {
	return storagefs.RemoveAll(path)
}

func (defaultFileOps) AvailableBytes(path string) (int64, error) {
	return availableBytes(path)
}

func (m memTableStore) Apply(point model.ResolvedPoint) error {
	return m.inner.Apply(point)
}

func (m memTableStore) ApplyBatch(points []model.ResolvedPoint) error {
	return m.inner.ApplyBatch(points)
}

func (m memTableStore) ApplyTypedBatch(batch model.ResolvedTypedBatch, rows []int) error {
	return m.inner.ApplyTypedBatch(batch, rows)
}

func (m memTableStore) SampleCount() int {
	return m.inner.SampleCount()
}

func (m memTableStore) ApproxMemorySamples() int {
	return m.inner.SampleCount()
}

func (m memTableStore) ApproxMemoryBytes() int64 {
	return m.inner.ApproxMemoryBytes()
}

func (m memTableStore) StatsSnapshot() memtable.Stats {
	return m.inner.StatsSnapshot()
}

func (m memTableStore) SnapshotAndReset() memSnapshot {
	return m.inner.SnapshotAndReset()
}

func (m memTableStore) Snapshot() memSnapshot {
	return m.inner.Snapshot()
}

func (m memTableStore) Query(query memtable.Query) []model.ColumnData {
	return m.inner.Query(query)
}

func (m memTableStore) ScanColumns(query memtable.Query) queryexec.ColumnDataStream {
	return m.inner.ScanColumns(query)
}

func (m memTableStore) Restore(snapshot memSnapshot) {
	memSnapshot, ok := snapshot.(*memtable.Snapshot)
	if !ok {
		return
	}
	m.inner.Restore(memSnapshot)
}
