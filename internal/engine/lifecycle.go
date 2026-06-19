package engine

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/sstable"
	"github.com/openmts/mts/internal/storagefs"
)

const streamingCompactionSeriesBatchSize = 256

const maxCachedCompactionIndexRows = 65536

var ErrCompactionDiskSpaceExceeded = errors.New("compaction disk space exceeded")

func (e *Engine) Compact(ctx context.Context) error {
	_, err := e.CompactWithResult(ctx)
	return err
}

func (e *Engine) CompactWithResult(ctx context.Context) (CompactionResult, error) {
	if err := ctx.Err(); err != nil {
		return CompactionResult{State: compactionTaskFailed, Error: err.Error()}, err
	}
	started := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()
	result := CompactionResult{State: compactionTaskNoop}
	for _, shard := range e.shards {
		shardResult, err := shard.CompactWithResult()
		result = mergeCompactionResult(result, shardResult)
		if err != nil {
			result.State = compactionTaskFailed
			result.Duration = time.Since(started)
			result.Error = err.Error()
			return result, err
		}
	}
	result.Duration = time.Since(started)
	if result.Shards > 0 && result.State == compactionTaskNoop {
		result.State = compactionTaskSucceeded
	}
	return result, nil
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
		deletedBytes, sizeErr := directorySize(shard.opts.Dir)
		if sizeErr != nil {
			deletedBytes = 0
		}
		if err := shard.closeLocked(); err != nil {
			e.retentionDeleteErrors++
			shard.lifecycleMu.Unlock()
			return err
		}
		if err := shard.deps.files.RemoveAll(shard.opts.Dir); err != nil {
			e.retentionDeleteErrors++
			shard.lifecycleMu.Unlock()
			return err
		}
		e.retentionExpired += uint64(len(shard.manifest.Parts))
		e.retentionDeletedBytes += uint64(deletedBytes)
		shard.lifecycleMu.Unlock()
		delete(e.shards, id)
	}
	return nil
}

func (s *Shard) Compact() error {
	_, err := s.CompactWithResult()
	return err
}

func (s *Shard) CompactWithResult() (CompactionResult, error) {
	started := time.Now()
	before := s.CompactionStatsSnapshot()
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if err := s.flushLocked(); err != nil {
		return failedCompactionResult(started, before, s.CompactionStatsSnapshot(), err)
	}
	if len(s.parts) <= 1 && len(s.tombstones) == 0 {
		return noopCompactionResult(started), nil
	}
	if s.opts.Compaction.Enabled {
		if err := s.runCompactionCascadeLocked(); err != nil {
			return failedCompactionResult(started, before, s.CompactionStatsSnapshot(), err)
		}
	}
	plan := fullCompactionPlan(s.manifest.Parts, s.tombstones, s.opts.Compaction)
	if plan == nil {
		return resultFromCompactionStats(started, before, s.CompactionStatsSnapshot(), nil), nil
	}
	err := s.compactPartsLocked(*plan)
	if err != nil {
		return failedCompactionResult(started, before, s.CompactionStatsSnapshot(), err)
	}
	return resultFromCompactionStats(started, before, s.CompactionStatsSnapshot(), nil), nil
}

func mergeCompactionResult(left CompactionResult, right CompactionResult) CompactionResult {
	if right.State == compactionTaskNoop {
		return left
	}
	left.Shards++
	left.InputParts += right.InputParts
	left.OutputParts += right.OutputParts
	left.InputBytes += right.InputBytes
	left.OutputBytes += right.OutputBytes
	left.DroppedRows += right.DroppedRows
	left.LastTask = right.LastTask
	if right.State == compactionTaskFailed {
		left.State = compactionTaskFailed
		left.Error = right.Error
		return left
	}
	if left.State == compactionTaskNoop {
		left.State = compactionTaskSucceeded
	}
	return left
}

func noopCompactionResult(started time.Time) CompactionResult {
	return CompactionResult{State: compactionTaskNoop, Duration: time.Since(started)}
}

func failedCompactionResult(
	started time.Time,
	before CompactionStats,
	after CompactionStats,
	err error,
) (CompactionResult, error) {
	result := resultFromCompactionStats(started, before, after, err)
	return result, err
}

func resultFromCompactionStats(
	started time.Time,
	before CompactionStats,
	after CompactionStats,
	err error,
) CompactionResult {
	result := CompactionResult{
		State:       compactionTaskNoop,
		Duration:    time.Since(started),
		InputParts:  after.InputParts - before.InputParts,
		OutputParts: after.OutputParts - before.OutputParts,
		InputBytes:  after.InputBytes - before.InputBytes,
		OutputBytes: after.OutputBytes - before.OutputBytes,
		DroppedRows: after.DroppedRows - before.DroppedRows,
		LastTask:    after.LastTask,
	}
	if err != nil {
		result.State = compactionTaskFailed
		result.Error = err.Error()
		return result
	}
	if after.Total > before.Total {
		result.State = compactionTaskSucceeded
	}
	return result
}

func (s *Shard) maybeCompactLocked() error {
	if !s.opts.Compaction.Enabled {
		return nil
	}
	return s.runCompactionCascadeLocked()
}

func (s *Shard) runCompactionCascadeLocked() error {
	for step := 0; step < s.opts.Compaction.MaxCascadeSteps; step++ {
		plan, err := nextCompactionPlan(s.manifest.Parts, s.tombstones, s.opts.Compaction)
		if err != nil {
			return err
		}
		if plan == nil {
			return nil
		}
		if !s.startCompactionPlan(plan.candidateSignature) {
			return nil
		}
		if err := s.compactPartsLocked(*plan); err != nil {
			s.finishCompactionPlan(plan.candidateSignature)
			return err
		}
		s.finishCompactionPlan(plan.candidateSignature)
	}
	return nil
}

func (s *Shard) startCompactionPlan(signature string) bool {
	if s.opts.scheduler == nil {
		return true
	}
	started := s.opts.scheduler.start(signature)
	if !started {
		s.compactionStats.recordSkip(compactionSkipDuplicateCandidate)
	}
	return started
}

func (s *Shard) finishCompactionPlan(signature string) {
	if s.opts.scheduler == nil {
		return
	}
	s.opts.scheduler.finish(signature)
}

func (s *Shard) compactPartsLocked(plan compactionPlan) error {
	plan.output = s.resolveCompactionOutputOptions(plan.output)
	hadTombstones := len(s.tombstones) > 0
	candidateParts := s.selectedParts(plan.candidates)
	if len(candidateParts) == 0 {
		return nil
	}
	attempt := s.compactionStats.beginPlan(plan)
	if err := s.preflightCompactionDiskSpace(plan); err != nil {
		attempt.finishWithRows(nil, 0, err)
		return err
	}
	newParts, newMeta, droppedRows, err := s.writeStreamingCompactionOutputsLocked(
		plan.outputLevel,
		plan.output,
		candidateParts,
	)
	if err != nil {
		attempt.finishWithRows(nil, droppedRows, err)
		return err
	}
	keptParts, keptMeta := s.keepUnselectedParts(plan.candidates)
	nextManifest := s.nextManifest(append(keptMeta, newMeta...))
	if err := s.deps.parts.WriteManifest(s.opts.Dir, nextManifest); err != nil {
		closeErr := s.cleanupCompactionOutputs(newParts, newMeta)
		joined := errors.Join(err, closeErr)
		attempt.finishWithRows(nil, droppedRows, joined)
		return joined
	}
	oldParts := s.parts
	s.parts = append(keptParts, newParts...)
	s.manifest = nextManifest
	if err := closeSelectedParts(oldParts, plan.candidates); err != nil {
		attempt.finishWithRows(nil, droppedRows, err)
		return err
	}
	removeErr := s.removeOldParts(plan.candidates)
	if removeErr != nil {
		attempt.finishWithRows(newMeta, droppedRows, removeErr)
	} else {
		s.compactionStats.recordSafeDeleteParts(len(plan.candidates))
		attempt.finishWithRows(newMeta, droppedRows, nil)
	}
	if hadTombstones {
		s.tombstones = nil
		if err := s.wal.Checkpoint(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Shard) preflightCompactionDiskSpace(plan compactionPlan) error {
	if s.deps.files == nil {
		return nil
	}
	required := plan.outputEstimateBytes + s.opts.Compaction.DiskSpaceReserveBytes + s.opts.Compaction.MinFreeBytes
	if required <= 0 {
		return nil
	}
	available, err := s.deps.files.AvailableBytes(s.opts.Dir)
	if err != nil {
		return fmt.Errorf("check compaction disk space: %w", err)
	}
	if available >= required {
		return nil
	}
	return fmt.Errorf(
		"%w: available_bytes=%d required_bytes=%d",
		ErrCompactionDiskSpaceExceeded,
		available,
		required,
	)
}

func (s *Shard) cleanupCompactionOutputs(parts []partReader, metas []sstable.PartMeta) error {
	err := closeParts(parts)
	for _, meta := range metas {
		err = errors.Join(err, s.removeUnreferencedPart(meta.Path, "remove compaction output failed"))
	}
	return err
}

func (s *Shard) removeOldParts(parts []sstable.PartMeta) error {
	var joined error
	for _, part := range parts {
		if err := s.deps.files.RemoveAll(part.Path); err != nil {
			issue := RecoveryIssue{
				Kind:    RecoveryIssueOrphanRemoveFailed,
				Path:    part.Path,
				Message: "remove compacted input part failed",
				Err:     err,
			}
			s.recordRecoveryIssue(issue)
			joined = errors.Join(joined, &issue)
		}
	}
	return joined
}

func (s *Shard) resolveCompactionOutputOptions(
	output compactionOutputOptions,
) compactionOutputOptions {
	if !compressionConfigured(output.compression) {
		output.compression = s.opts.Compression
	}
	if output.maxOutputPartBytes <= 0 {
		output.maxOutputPartBytes = s.opts.Compaction.MaxOutputPartBytes
	}
	return output
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
	outputOpts compactionOutputOptions,
	candidateParts []partReader,
) ([]partReader, []sstable.PartMeta, int, error) {
	inputs, err := newCompactionInputs(s.deps.parts, candidateParts, s.opts.Start, s.opts.End)
	if err != nil {
		return nil, nil, 0, err
	}
	seriesIDs, err := compactionSeriesIDs(inputs)
	if err != nil {
		return nil, nil, 0, err
	}
	if len(seriesIDs) == 0 {
		return nil, nil, 0, nil
	}
	output := newCompactionOutput(s, outputLevel, outputOpts)
	droppedRows := 0
	for _, seriesID := range seriesIDs {
		columns, inputRows, outputRows, err := s.queryCompactionSeriesWithStats(inputs, seriesID)
		if err != nil {
			abortErr := output.abort()
			return nil, nil, droppedRows, errors.Join(err, abortErr)
		}
		droppedRows += inputRows - outputRows
		if err := output.addSeries(columns); err != nil {
			abortErr := output.abort()
			return nil, nil, droppedRows, errors.Join(err, abortErr)
		}
	}
	parts, metas, err := output.close()
	if err != nil {
		abortErr := output.abort()
		return nil, nil, droppedRows, errors.Join(err, abortErr)
	}
	return parts, metas, droppedRows, nil
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
	columns, _, _, err := s.queryCompactionSeriesWithStats(inputs, seriesID)
	return columns, err
}

func (s *Shard) queryCompactionSeriesWithStats(
	inputs []compactionInput,
	seriesID uint64,
) ([]model.ColumnData, int, int, error) {
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
			return nil, 0, 0, err
		}
		columns = append(columns, got...)
	}
	inputRows := countColumnSamples(columns)
	merged := mergeColumnData(columns)
	output := applyTombstones(merged, s.tombstones)
	return output, inputRows, countColumnSamples(output), nil
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
	outputOpts   compactionOutputOptions
	writer       partWriter
	currentBytes int64
	parts        []partReader
	metas        []sstable.PartMeta
}

func newCompactionOutput(
	shard *Shard,
	outputLevel int,
	outputOpts compactionOutputOptions,
) *compactionOutput {
	return &compactionOutput{
		shard:       shard,
		outputLevel: outputLevel,
		outputOpts:  outputOpts,
		parts:       make([]partReader, 0),
		metas:       make([]sstable.PartMeta, 0),
	}
}

func (o *compactionOutput) addSeries(columns []model.ColumnData) error {
	if len(columns) == 0 {
		return nil
	}
	for _, group := range splitLargeSeriesColumns(columns, o.outputOpts.maxOutputPartBytes) {
		if err := o.addSeriesGroup(group); err != nil {
			return err
		}
	}
	return nil
}

func (o *compactionOutput) addSeriesGroup(columns []model.ColumnData) error {
	columnsBytes := estimateColumnsBytes(columns)
	release, err := o.reserveCompactionMemory(columnsBytes)
	if err != nil {
		return err
	}
	defer release()
	if o.shouldRoll(columnsBytes) {
		if err := o.closeCurrent(); err != nil {
			return err
		}
	}
	if o.writer == nil {
		writer, err := o.shard.deps.parts.NewWriter(o.shard.opts.Dir, o.outputLevel, o.shard.nextPartID(), sstable.WriteOptions{
			Compression:  o.outputOpts.compression,
			MemoryBudget: storageCompressionBudget{memory: o.shard.opts.Memory},
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

func (o *compactionOutput) reserveCompactionMemory(bytes int64) (func(), error) {
	if o.shard == nil || o.shard.opts.Memory == nil {
		return func() {}, nil
	}
	return o.shard.opts.Memory.Reserve(storageMemoryCompaction, 0, bytes)
}

func (o *compactionOutput) shouldRoll(seriesBytes int64) bool {
	target := o.outputOpts.maxOutputPartBytes
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
	part, err := o.shard.deps.parts.OpenPartTrusted(meta.Path)
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

func countColumnSamples(columns []model.ColumnData) int {
	total := 0
	for _, column := range columns {
		total += len(column.Samples)
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
	err := storagefs.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
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
