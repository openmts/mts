package engine

import (
	"errors"
	"fmt"
	"os"
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
	Compression        model.CompressionOptions
}

type Shard struct {
	lifecycleMu sync.RWMutex

	opts           ShardOptions
	wal            *wal.Log
	mem            *memtable.MemTable
	parts          []*sstable.Part
	manifest       sstable.Manifest
	tombstones     []model.Tombstone
	nextPart       int
	maintenanceErr error
	testHooks      shardTestHooks
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
		closeErr := closeParts(shard.parts)
		if closeErr != nil {
			return nil, 0, errors.Join(err, closeErr)
		}
		return nil, 0, err
	}
	shard.maintenanceErr = shard.cleanupOrphanParts()
	log, err := wal.Open(filepath.Join(opts.Dir, "wal"), wal.Options(opts.WAL))
	if err != nil {
		closeErr := closeParts(shard.parts)
		return nil, 0, errors.Join(err, closeErr)
	}
	shard.wal = log
	replayed, err := log.ReplayRecords()
	if err != nil {
		closeErr := shard.closeLocked()
		return nil, 0, errors.Join(err, closeErr)
	}
	for _, record := range replayed {
		for _, point := range record.Points {
			if err := shard.mem.Apply(point); err != nil {
				closeErr := shard.closeLocked()
				return nil, 0, errors.Join(err, closeErr)
			}
			if point.WriteSeq > maxSeq {
				maxSeq = point.WriteSeq
			}
		}
		for _, tombstone := range record.Tombstones {
			shard.tombstones = append(shard.tombstones, tombstone)
			if tombstone.WriteSeq > maxSeq {
				maxSeq = tombstone.WriteSeq
			}
		}
	}
	return shard, maxSeq, nil
}

func (s *Shard) Write(point model.ResolvedPoint, syncWrite bool) error {
	return s.WriteBatch([]model.ResolvedPoint{point}, syncWrite)
}

func (s *Shard) WriteBatch(points []model.ResolvedPoint, syncWrite bool) error {
	if len(points) == 0 {
		return nil
	}
	if err := s.wal.Append(points, syncWrite); err != nil {
		return err
	}
	if err := s.mem.ApplyBatch(points); err != nil {
		return err
	}
	if s.mem.SampleCount() >= s.opts.MemTableMaxSamples {
		if err := s.Flush(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Shard) DeleteRange(tombstone model.Tombstone, syncWrite bool) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if tombstone.EndTime < tombstone.StartTime {
		return nil
	}
	if err := s.wal.AppendTombstones([]model.Tombstone{tombstone}, syncWrite); err != nil {
		return err
	}
	s.tombstones = append(s.tombstones, tombstone)
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
	columns := snapshot.Columns(memtable.Query{
		Start: s.opts.Start,
		End:   s.opts.End,
	})
	if len(columns) == 0 {
		s.mem.Restore(snapshot)
		return nil
	}
	meta, err := sstable.WritePartWithOptions(s.opts.Dir, 0, s.nextPartID(), columns, sstable.WriteOptions{
		Compression: s.opts.Compression,
	})
	if err != nil {
		s.mem.Restore(snapshot)
		return err
	}
	if s.testHooks.afterPartWriteBeforeManifest != nil {
		if err := s.testHooks.afterPartWriteBeforeManifest(); err != nil {
			s.mem.Restore(snapshot)
			return err
		}
	}
	part, err := sstable.OpenPart(meta.Path)
	if err != nil {
		s.mem.Restore(snapshot)
		return err
	}
	nextManifest := sstable.Manifest{Parts: append([]sstable.PartMeta{}, s.manifest.Parts...)}
	nextManifest.Parts = append(nextManifest.Parts, meta)
	if err := sstable.WriteManifest(s.opts.Dir, nextManifest); err != nil {
		s.mem.Restore(snapshot)
		return err
	}
	s.parts = append(s.parts, part)
	s.manifest = nextManifest
	if s.testHooks.afterManifestBeforeWALTrunc != nil {
		if err := s.testHooks.afterManifestBeforeWALTrunc(); err != nil {
			s.mem.Restore(snapshot)
			return err
		}
	}
	if len(s.tombstones) == 0 {
		if err := s.wal.Checkpoint(); err != nil {
			s.mem.Restore(snapshot)
			return err
		}
	}
	snapshot.Release()
	return nil
}

func (s *Shard) Query(query memtable.Query) ([]model.ColumnData, error) {
	s.lifecycleMu.RLock()
	defer s.lifecycleMu.RUnlock()
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
	return applyTombstones(mergeColumnData(columns), s.tombstones), nil
}

func (s *Shard) Close() error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	return s.closeLocked()
}

func (s *Shard) closeLocked() error {
	partErr := closeParts(s.parts)
	if s.wal == nil {
		return partErr
	}
	walErr := s.wal.Close()
	s.wal = nil
	return errors.Join(partErr, walErr)
}

func closeParts(parts []*sstable.Part) error {
	var err error
	for _, part := range parts {
		err = errors.Join(err, part.Close())
	}
	return err
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

func (s *Shard) cleanupOrphanParts() error {
	referenced := make(map[string]struct{}, len(s.manifest.Parts))
	for _, meta := range s.manifest.Parts {
		referenced[filepath.Base(meta.Path)] = struct{}{}
		referenced[meta.ID] = struct{}{}
	}
	entries, err := os.ReadDir(s.opts.Dir)
	if err != nil {
		return fmt.Errorf("read shard dir for orphan cleanup: %w", err)
	}
	var cleanupErr error
	for _, entry := range entries {
		if !entry.IsDir() || !isPartDirName(entry.Name()) {
			continue
		}
		if _, ok := referenced[entry.Name()]; ok {
			continue
		}
		err := storagefs.RemoveAll(filepath.Join(s.opts.Dir, entry.Name()))
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove orphan part %s: %w", entry.Name(), err))
		}
	}
	return cleanupErr
}

func isPartDirName(name string) bool {
	return len(name) == len("sst-000000") && name[:4] == "sst-"
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
	if len(columns) == 0 {
		return []model.ColumnData{}
	}
	sortColumnData(columns)
	merged := make([]model.ColumnData, 0, len(columns))
	for start := 0; start < len(columns); {
		end := start + 1
		for end < len(columns) && sameColumn(columns[start], columns[end]) {
			end++
		}
		merged = append(merged, mergeColumnGroup(columns[start:end]))
		start = end
	}
	return merged
}

func sortColumnData(columns []model.ColumnData) {
	sort.Slice(columns, func(i, j int) bool {
		left := columns[i]
		right := columns[j]
		if left.SeriesID != right.SeriesID {
			return left.SeriesID < right.SeriesID
		}
		return left.FieldID < right.FieldID
	})
}

func sameColumn(left model.ColumnData, right model.ColumnData) bool {
	return left.SeriesID == right.SeriesID && left.FieldID == right.FieldID
}

func mergeColumnGroup(columns []model.ColumnData) model.ColumnData {
	first := columns[0]
	if len(columns) == 1 && samplesStrictlyIncreasing(first.Samples) {
		return first
	}
	if columnsSamplesOrdered(columns) {
		return model.ColumnData{
			SeriesID:  first.SeriesID,
			FieldID:   first.FieldID,
			FieldType: first.FieldType,
			Samples:   mergeOrderedSamples(columns),
		}
	}
	return mergeSamplesFallback(first.SeriesID, first.FieldID, first.FieldType, columns)
}

func columnsSamplesOrdered(columns []model.ColumnData) bool {
	for _, column := range columns {
		if !samplesOrdered(column.Samples) {
			return false
		}
	}
	return true
}

func samplesOrdered(samples []model.VersionedSample) bool {
	for index := 1; index < len(samples); index++ {
		if samples[index-1].Timestamp > samples[index].Timestamp {
			return false
		}
	}
	return true
}

func samplesStrictlyIncreasing(samples []model.VersionedSample) bool {
	for index := 1; index < len(samples); index++ {
		if samples[index-1].Timestamp >= samples[index].Timestamp {
			return false
		}
	}
	return true
}

func mergeOrderedSamples(columns []model.ColumnData) []model.VersionedSample {
	switch len(columns) {
	case 1:
		return dedupeOrderedSamples(columns[0].Samples)
	case 2:
		return mergeTwoOrderedSamples(columns[0].Samples, columns[1].Samples)
	}
	positions := make([]int, len(columns))
	out := make([]model.VersionedSample, 0, totalSampleCount(columns))
	for {
		minTime, ok := nextMinTimestamp(columns, positions)
		if !ok {
			return out
		}
		var latest model.VersionedSample
		hasLatest := false
		for columnIndex, column := range columns {
			for positions[columnIndex] < len(column.Samples) {
				sample := column.Samples[positions[columnIndex]]
				if sample.Timestamp != minTime {
					break
				}
				if !hasLatest || sample.WriteSeq >= latest.WriteSeq {
					latest = sample
					hasLatest = true
				}
				positions[columnIndex]++
			}
		}
		out = append(out, latest)
	}
}

func dedupeOrderedSamples(samples []model.VersionedSample) []model.VersionedSample {
	out := make([]model.VersionedSample, 0, len(samples))
	for index := 0; index < len(samples); {
		timestamp := samples[index].Timestamp
		latest := samples[index]
		index++
		for index < len(samples) && samples[index].Timestamp == timestamp {
			if samples[index].WriteSeq >= latest.WriteSeq {
				latest = samples[index]
			}
			index++
		}
		out = append(out, latest)
	}
	return out
}

func mergeTwoOrderedSamples(
	left []model.VersionedSample,
	right []model.VersionedSample,
) []model.VersionedSample {
	out := make([]model.VersionedSample, 0, len(left)+len(right))
	leftIndex := 0
	rightIndex := 0
	for leftIndex < len(left) || rightIndex < len(right) {
		timestamp := nextTwoTimestamp(left, leftIndex, right, rightIndex)
		var latest model.VersionedSample
		hasLatest := false
		leftIndex, latest, hasLatest = consumeTimestamp(left, leftIndex, timestamp, latest, hasLatest)
		rightIndex, latest, _ = consumeTimestamp(right, rightIndex, timestamp, latest, hasLatest)
		out = append(out, latest)
	}
	return out
}

func nextTwoTimestamp(
	left []model.VersionedSample,
	leftIndex int,
	right []model.VersionedSample,
	rightIndex int,
) int64 {
	if leftIndex >= len(left) {
		return right[rightIndex].Timestamp
	}
	if rightIndex >= len(right) {
		return left[leftIndex].Timestamp
	}
	if left[leftIndex].Timestamp < right[rightIndex].Timestamp {
		return left[leftIndex].Timestamp
	}
	return right[rightIndex].Timestamp
}

func consumeTimestamp(
	samples []model.VersionedSample,
	index int,
	timestamp int64,
	latest model.VersionedSample,
	hasLatest bool,
) (int, model.VersionedSample, bool) {
	for index < len(samples) && samples[index].Timestamp == timestamp {
		if !hasLatest || samples[index].WriteSeq >= latest.WriteSeq {
			latest = samples[index]
			hasLatest = true
		}
		index++
	}
	return index, latest, hasLatest
}

func nextMinTimestamp(columns []model.ColumnData, positions []int) (int64, bool) {
	var minTime int64
	found := false
	for index, column := range columns {
		if positions[index] >= len(column.Samples) {
			continue
		}
		timestamp := column.Samples[positions[index]].Timestamp
		if !found || timestamp < minTime {
			minTime = timestamp
			found = true
		}
	}
	return minTime, found
}

func totalSampleCount(columns []model.ColumnData) int {
	total := 0
	for _, column := range columns {
		total += len(column.Samples)
	}
	return total
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

func mergeSamplesFallback(
	seriesID uint64,
	fieldID uint32,
	fieldType model.FieldType,
	columns []model.ColumnData,
) model.ColumnData {
	samples := make([]model.VersionedSample, 0, totalSampleCount(columns))
	for _, column := range columns {
		samples = append(samples, column.Samples...)
	}
	return mergeSamples(seriesID, fieldID, fieldType, samples)
}
