package engine

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"time"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/sstable"
)

const streamingCompactionSeriesBatchSize = 256

const maxCachedCompactionIndexRows = 65536

func (e *Engine) Compact(_ context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, shard := range e.shards {
		if err := shard.Compact(); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) ApplyRetention(_ context.Context, now time.Time) error {
	if e.opts.Retention <= 0 {
		return nil
	}
	cutoff := now.UnixNano() - int64(e.opts.Retention)
	e.mu.Lock()
	defer e.mu.Unlock()
	for id, shard := range e.shards {
		if shard.opts.End >= cutoff {
			continue
		}
		shard.lifecycleMu.Lock()
		if err := shard.closeLocked(); err != nil {
			shard.lifecycleMu.Unlock()
			return err
		}
		if err := shard.deps.files.RemoveAll(shard.opts.Dir); err != nil {
			shard.lifecycleMu.Unlock()
			return err
		}
		shard.lifecycleMu.Unlock()
		delete(e.shards, id)
	}
	return nil
}

func (s *Shard) Compact() error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if err := s.flushLocked(); err != nil {
		return err
	}
	if len(s.parts) <= 1 && len(s.tombstones) == 0 {
		return nil
	}
	return s.compactPartsLocked(s.manifest.Parts, 1)
}

func (s *Shard) maybeCompactLocked() error {
	if !s.opts.Compaction.Enabled {
		return nil
	}
	candidates, err := s.level0CompactionCandidates()
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}
	return s.compactPartsLocked(candidates, 1)
}

func (s *Shard) compactPartsLocked(candidates []sstable.PartMeta, outputLevel int) error {
	hadTombstones := len(s.tombstones) > 0
	candidateParts := s.selectedParts(candidates)
	if len(candidateParts) == 0 {
		return nil
	}
	newParts, newMeta, err := s.writeStreamingCompactionOutputsLocked(outputLevel, candidateParts)
	if err != nil {
		return err
	}
	keptParts, keptMeta := s.keepUnselectedParts(candidates)
	nextManifest := sstable.Manifest{Parts: append(keptMeta, newMeta...)}
	if err := s.deps.parts.WriteManifest(s.opts.Dir, nextManifest); err != nil {
		closeErr := closeParts(newParts)
		return errors.Join(err, closeErr)
	}
	oldParts := s.parts
	s.parts = append(keptParts, newParts...)
	s.manifest = nextManifest
	if err := closeSelectedParts(oldParts, candidates); err != nil {
		return err
	}
	if err := s.removeOldParts(candidates); err != nil {
		return err
	}
	if hadTombstones {
		s.tombstones = nil
		return s.wal.Checkpoint()
	}
	return nil
}

func (s *Shard) selectedParts(candidates []sstable.PartMeta) []partReader {
	selected := partIDSet(candidates)
	parts := make([]partReader, 0, len(candidates))
	for _, part := range s.parts {
		if _, ok := selected[part.Meta().ID]; ok {
			parts = append(parts, part)
		}
	}
	return parts
}

func (s *Shard) writeStreamingCompactionOutputsLocked(
	outputLevel int,
	candidateParts []partReader,
) ([]partReader, []sstable.PartMeta, error) {
	inputs, err := newCompactionInputs(s.deps.parts, candidateParts, s.opts.Start, s.opts.End)
	if err != nil {
		return nil, nil, err
	}
	seriesIDs, err := compactionSeriesIDs(inputs)
	if err != nil {
		return nil, nil, err
	}
	if len(seriesIDs) == 0 {
		return nil, nil, nil
	}
	output := newCompactionOutput(s, outputLevel)
	for _, seriesID := range seriesIDs {
		columns, err := s.queryCompactionSeries(inputs, seriesID)
		if err != nil {
			abortErr := output.abort()
			return nil, nil, errors.Join(err, abortErr)
		}
		if err := output.addSeries(columns); err != nil {
			abortErr := output.abort()
			return nil, nil, errors.Join(err, abortErr)
		}
	}
	parts, metas, err := output.close()
	if err != nil {
		abortErr := output.abort()
		return nil, nil, errors.Join(err, abortErr)
	}
	return parts, metas, nil
}

type compactionInput struct {
	part   partReader
	query  sstable.Query
	reader seriesBatchReader
}

func newCompactionInputs(
	manager partManager,
	parts []partReader,
	start int64,
	end int64,
) ([]compactionInput, error) {
	cacheReaders := shouldCacheCompactionReaders(parts)
	inputs := make([]compactionInput, 0, len(parts))
	for _, part := range parts {
		query := sstable.Query{Start: start, End: end}
		input := compactionInput{part: part, query: query}
		if cacheReaders {
			reader, err := manager.NewSeriesBatchReader(part, query)
			if err != nil {
				return nil, err
			}
			input.reader = reader
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}

func shouldCacheCompactionReaders(parts []partReader) bool {
	total := 0
	for _, part := range parts {
		total += part.Meta().SeriesCount
		if total > maxCachedCompactionIndexRows {
			return false
		}
	}
	return true
}

func compactionSeriesIDs(inputs []compactionInput) ([]uint64, error) {
	total := 0
	for _, input := range inputs {
		if input.reader != nil {
			total += input.reader.SeriesCount()
			continue
		}
		total += input.part.Meta().SeriesCount
	}
	ids := make([]uint64, 0, total)
	for _, input := range inputs {
		if input.reader != nil {
			ids = input.reader.AppendSeriesIDs(ids)
			continue
		}
		partIDs, err := input.part.SeriesIDs(input.query)
		if err != nil {
			return nil, err
		}
		ids = append(ids, partIDs...)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return compactSortedSeriesIDs(ids), nil
}

func compactSortedSeriesIDs(ids []uint64) []uint64 {
	if len(ids) <= 1 {
		return ids
	}
	write := 1
	for read := 1; read < len(ids); read++ {
		if ids[read] == ids[write-1] {
			continue
		}
		ids[write] = ids[read]
		write++
	}
	return ids[:write]
}

func (s *Shard) queryCompactionSeries(
	inputs []compactionInput,
	seriesID uint64,
) ([]model.ColumnData, error) {
	columns := make([]model.ColumnData, 0)
	for _, input := range inputs {
		var got []model.ColumnData
		var err error
		if input.reader != nil {
			got, err = input.reader.QuerySeriesID(seriesID)
		} else {
			got, err = input.part.QuerySeriesIDs(input.query, []uint64{seriesID})
		}
		if err != nil {
			return nil, err
		}
		columns = append(columns, got...)
	}
	merged := mergeColumnData(columns)
	return applyTombstones(merged, s.tombstones), nil
}

func forEachCompactionSeriesGroup(
	columns []model.ColumnData,
	visit func([]model.ColumnData) error,
) error {
	if len(columns) == 0 {
		return nil
	}
	sortColumnData(columns)
	for start := 0; start < len(columns); {
		end := start + 1
		for end < len(columns) && columns[end].SeriesID == columns[start].SeriesID {
			end++
		}
		if err := visit(columns[start:end]); err != nil {
			return err
		}
		start = end
	}
	return nil
}

type compactionOutput struct {
	shard        *Shard
	outputLevel  int
	writer       partWriter
	currentBytes int64
	parts        []partReader
	metas        []sstable.PartMeta
}

func newCompactionOutput(shard *Shard, outputLevel int) *compactionOutput {
	return &compactionOutput{
		shard:       shard,
		outputLevel: outputLevel,
		parts:       make([]partReader, 0),
		metas:       make([]sstable.PartMeta, 0),
	}
}

func (o *compactionOutput) addSeries(columns []model.ColumnData) error {
	if len(columns) == 0 {
		return nil
	}
	for _, group := range splitLargeSeriesColumns(columns, o.shard.opts.Compaction.MaxOutputPartBytes) {
		if err := o.addSeriesGroup(group); err != nil {
			return err
		}
	}
	return nil
}

func (o *compactionOutput) addSeriesGroup(columns []model.ColumnData) error {
	columnsBytes := estimateColumnsBytes(columns)
	if o.shouldRoll(columnsBytes) {
		if err := o.closeCurrent(); err != nil {
			return err
		}
	}
	if o.writer == nil {
		writer, err := o.shard.deps.parts.NewWriter(o.shard.opts.Dir, o.outputLevel, o.shard.nextPartID(), sstable.WriteOptions{
			Compression: o.shard.opts.Compression,
		})
		if err != nil {
			return err
		}
		o.writer = writer
	}
	if err := o.writer.AddSeries(columns); err != nil {
		return err
	}
	o.currentBytes += columnsBytes
	return nil
}

func (o *compactionOutput) shouldRoll(seriesBytes int64) bool {
	target := o.shard.opts.Compaction.MaxOutputPartBytes
	return target > 0 && o.writer != nil && o.currentBytes > 0 && o.currentBytes+seriesBytes > target
}

func (o *compactionOutput) close() ([]partReader, []sstable.PartMeta, error) {
	if err := o.closeCurrent(); err != nil {
		return nil, nil, err
	}
	return o.parts, o.metas, nil
}

func (o *compactionOutput) closeCurrent() error {
	if o.writer == nil {
		return nil
	}
	meta, err := o.writer.Close()
	o.writer = nil
	o.currentBytes = 0
	if err != nil {
		return err
	}
	part, err := o.shard.deps.parts.OpenPart(meta.Path)
	if err != nil {
		return err
	}
	o.parts = append(o.parts, part)
	o.metas = append(o.metas, meta)
	return nil
}

func (o *compactionOutput) abort() error {
	var err error
	if o.writer != nil {
		err = errors.Join(err, o.writer.Abort())
		o.writer = nil
	}
	err = errors.Join(err, closeParts(o.parts))
	for _, meta := range o.metas {
		err = errors.Join(err, o.shard.deps.files.RemoveAll(meta.Path))
	}
	o.parts = nil
	o.metas = nil
	return err
}

func estimateColumnsBytes(columns []model.ColumnData) int64 {
	var total int64
	for _, column := range columns {
		total += estimateColumnBytes(column)
	}
	return total
}

func splitLargeSeriesColumns(columns []model.ColumnData, targetBytes int64) [][]model.ColumnData {
	if targetBytes <= 0 || len(columns) <= 1 || estimateColumnsBytes(columns) <= targetBytes {
		return [][]model.ColumnData{columns}
	}
	groups := make([][]model.ColumnData, 0, len(columns))
	start := 0
	var size int64
	for index, column := range columns {
		columnBytes := estimateColumnBytes(column)
		if index > start && size+columnBytes > targetBytes {
			groups = append(groups, columns[start:index])
			start = index
			size = 0
		}
		size += columnBytes
	}
	return append(groups, columns[start:])
}

func estimateColumnBytes(column model.ColumnData) int64 {
	var bytes int64 = 32
	for _, sample := range column.Samples {
		bytes += 16
		switch sample.Value.Type {
		case model.FieldFloat64:
			bytes += 8
		case model.FieldInt64:
			bytes += 8
		case model.FieldBool:
			bytes++
		case model.FieldString:
			bytes += int64(len(sample.Value.String)) + 4
		}
	}
	return bytes
}

func (s *Shard) level0CompactionCandidates() ([]sstable.PartMeta, error) {
	candidates := make([]sstable.PartMeta, 0)
	var size int64
	for _, part := range s.manifest.Parts {
		if part.Level != 0 {
			continue
		}
		candidates = append(candidates, part)
		if s.opts.Compaction.Level0SizeLimit > 0 {
			partBytes, err := directorySize(part.Path)
			if err != nil {
				return nil, err
			}
			size += partBytes
		}
	}
	limit := s.opts.Compaction.Level0PartLimit
	if limit <= 0 {
		limit = 4
	}
	if len(candidates) > limit {
		return candidates, nil
	}
	if s.opts.Compaction.Level0SizeLimit > 0 && size > s.opts.Compaction.Level0SizeLimit {
		return candidates, nil
	}
	return nil, nil
}

func (s *Shard) keepUnselectedParts(candidates []sstable.PartMeta) ([]partReader, []sstable.PartMeta) {
	selected := partIDSet(candidates)
	keptParts := make([]partReader, 0, len(s.parts))
	keptMeta := make([]sstable.PartMeta, 0, len(s.manifest.Parts))
	for _, part := range s.parts {
		if _, ok := selected[part.Meta().ID]; !ok {
			keptParts = append(keptParts, part)
		}
	}
	for _, meta := range s.manifest.Parts {
		if _, ok := selected[meta.ID]; !ok {
			keptMeta = append(keptMeta, meta)
		}
	}
	return keptParts, keptMeta
}

func closeSelectedParts(parts []partReader, selectedMeta []sstable.PartMeta) error {
	selected := partIDSet(selectedMeta)
	var err error
	for _, part := range parts {
		if _, ok := selected[part.Meta().ID]; ok {
			err = errors.Join(err, part.Close())
		}
	}
	return err
}

func partIDSet(parts []sstable.PartMeta) map[string]struct{} {
	out := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		out[part.ID] = struct{}{}
	}
	return out
}

func directorySize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Shard) removeOldParts(parts []sstable.PartMeta) error {
	for _, part := range parts {
		if err := s.deps.files.RemoveAll(part.Path); err != nil {
			return err
		}
	}
	return nil
}
