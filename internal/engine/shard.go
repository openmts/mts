package engine

import (
	"fmt"
	"path/filepath"
	"sort"
	"sync"

	"codeberg.org/mts/mts/internal/memtable"
	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/sstable"
	"codeberg.org/mts/mts/internal/storagefs"
	"codeberg.org/mts/mts/internal/wal"
)

type ShardOptions struct {
	Dir                string
	Database           string
	RetentionPolicy    string
	Start              int64
	End                int64
	WAL                model.WALOptions
	MemTableMaxSamples int
	Compaction         model.CompactionOptions
}

type Shard struct {
	lifecycleMu sync.Mutex

	opts      ShardOptions
	wal       *wal.Log
	mem       *memtable.MemTable
	parts     []*sstable.Part
	manifest  sstable.Manifest
	nextPart  int
	testHooks shardTestHooks
}

type shardTestHooks struct {
	afterPartWriteBeforeManifest func() error
	afterManifestBeforeWALTrunc  func() error
}

func OpenShard(opts ShardOptions) (*Shard, uint64, error) {
	if err := storagefs.MkdirAll(opts.Dir); err != nil {
		return nil, 0, err
	}
	manifest, err := sstable.LoadManifest(opts.Dir)
	if err != nil {
		return nil, 0, err
	}
	shard := &Shard{
		opts:     opts,
		mem:      memtable.New(),
		manifest: manifest,
		parts:    make([]*sstable.Part, 0, len(manifest.Parts)),
	}
	maxSeq, err := shard.openParts()
	if err != nil {
		return nil, 0, err
	}
	log, err := wal.Open(filepath.Join(opts.Dir, "wal"), wal.Options(opts.WAL))
	if err != nil {
		return nil, 0, err
	}
	shard.wal = log
	replayed, err := log.Replay()
	if err != nil {
		return nil, 0, err
	}
	for _, point := range replayed {
		if err := shard.mem.Apply(point); err != nil {
			return nil, 0, err
		}
		if point.WriteSeq > maxSeq {
			maxSeq = point.WriteSeq
		}
	}
	return shard, maxSeq, nil
}

func (s *Shard) Write(point model.ResolvedPoint, syncWrite bool) error {
	if err := s.wal.Append([]model.ResolvedPoint{point}, syncWrite); err != nil {
		return err
	}
	if err := s.mem.Apply(point); err != nil {
		return err
	}
	if s.mem.SampleCount() >= s.opts.MemTableMaxSamples {
		if err := s.Flush(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Shard) Flush() error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if err := s.flushLocked(); err != nil {
		return err
	}
	return s.maybeCompactLocked()
}

func (s *Shard) flushLocked() error {
	if s.mem.SampleCount() == 0 {
		return nil
	}
	snapshot := s.mem.SnapshotAndReset()
	columns := snapshot.Query(memtable.Query{
		Start: s.opts.Start,
		End:   s.opts.End,
	})
	if len(columns) == 0 {
		return nil
	}
	meta, err := sstable.WritePart(s.opts.Dir, 0, s.nextPartID(), columns)
	if err != nil {
		return err
	}
	if s.testHooks.afterPartWriteBeforeManifest != nil {
		if err := s.testHooks.afterPartWriteBeforeManifest(); err != nil {
			return err
		}
	}
	part, err := sstable.OpenPart(meta.Path)
	if err != nil {
		return err
	}
	nextManifest := sstable.Manifest{Parts: append([]sstable.PartMeta{}, s.manifest.Parts...)}
	nextManifest.Parts = append(nextManifest.Parts, meta)
	if err := sstable.WriteManifest(s.opts.Dir, nextManifest); err != nil {
		return err
	}
	s.parts = append(s.parts, part)
	s.manifest = nextManifest
	if s.testHooks.afterManifestBeforeWALTrunc != nil {
		if err := s.testHooks.afterManifestBeforeWALTrunc(); err != nil {
			return err
		}
	}
	if err := s.wal.TruncateAll(); err != nil {
		return err
	}
	return nil
}

func (s *Shard) Query(query memtable.Query) ([]model.ColumnData, error) {
	columns := s.mem.Query(query)
	for _, part := range s.parts {
		got, err := part.Query(sstable.Query{
			SeriesIDs: query.SeriesIDs,
			FieldIDs:  query.FieldIDs,
			Start:     query.Start,
			End:       query.End,
		})
		if err != nil {
			return nil, err
		}
		columns = append(columns, got...)
	}
	return mergeColumnData(columns), nil
}

func (s *Shard) Close() error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	return s.closeLocked()
}

func (s *Shard) closeLocked() error {
	if s.wal == nil {
		return nil
	}
	return s.wal.Close()
}

func (s *Shard) openParts() (uint64, error) {
	var maxSeq uint64
	for _, meta := range s.manifest.Parts {
		part, err := sstable.OpenPart(meta.Path)
		if err != nil {
			return 0, err
		}
		s.parts = append(s.parts, part)
		if meta.MaxWriteSeq > maxSeq {
			maxSeq = meta.MaxWriteSeq
		}
		if partNumber(meta.ID) >= s.nextPart {
			s.nextPart = partNumber(meta.ID) + 1
		}
	}
	return maxSeq, nil
}

func (s *Shard) nextPartID() string {
	if s.nextPart == 0 {
		s.nextPart = 1
	}
	id := fmt.Sprintf("sst-%06d", s.nextPart)
	s.nextPart++
	return id
}

func partNumber(id string) int {
	var number int
	if _, err := fmt.Sscanf(id, "sst-%06d", &number); err != nil {
		return 0
	}
	return number
}

func mergeColumnData(columns []model.ColumnData) []model.ColumnData {
	type key struct {
		seriesID uint64
		fieldID  uint32
	}
	grouped := make(map[key][]model.VersionedSample)
	types := make(map[key]model.FieldType)
	for _, column := range columns {
		k := key{seriesID: column.SeriesID, fieldID: column.FieldID}
		grouped[k] = append(grouped[k], column.Samples...)
		types[k] = column.FieldType
	}
	merged := make([]model.ColumnData, 0, len(grouped))
	for k, samples := range grouped {
		merged = append(merged, mergeSamples(k.seriesID, k.fieldID, types[k], samples))
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].SeriesID != merged[j].SeriesID {
			return merged[i].SeriesID < merged[j].SeriesID
		}
		return merged[i].FieldID < merged[j].FieldID
	})
	return merged
}

func mergeSamples(
	seriesID uint64,
	fieldID uint32,
	fieldType model.FieldType,
	samples []model.VersionedSample,
) model.ColumnData {
	byTime := make(map[int64]model.VersionedSample)
	for _, sample := range samples {
		current, ok := byTime[sample.Timestamp]
		if !ok || sample.WriteSeq >= current.WriteSeq {
			byTime[sample.Timestamp] = sample
		}
	}
	out := model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   fieldID,
		FieldType: fieldType,
		Samples:   make([]model.VersionedSample, 0, len(byTime)),
	}
	for _, sample := range byTime {
		out.Samples = append(out.Samples, sample)
	}
	sort.Slice(out.Samples, func(i, j int) bool {
		return out.Samples[i].Timestamp < out.Samples[j].Timestamp
	})
	return out
}
