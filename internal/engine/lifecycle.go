package engine

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"sync"
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/sstable"
	"github.com/openmts/mts/internal/storagefs"
)

const streamingCompactionSeriesBatchSize = 256

// maxCompactionSeriesSamples 限制单次写入 compact 输出的样本窗口，降低峰值 RSS。
const maxCompactionSeriesSamples = 65536

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
	shards := e.snapshotShards()
	workers := e.compactWorkerLimit()
	type shardOutcome struct {
		result CompactionResult
		err    error
	}
	outcomes := make([]shardOutcome, len(shards))
	if workers <= 1 || len(shards) <= 1 {
		for index, shard := range shards {
			if err := ctx.Err(); err != nil {
				result := CompactionResult{State: compactionTaskFailed, Error: err.Error(), Duration: time.Since(started)}
				return result, err
			}
			outcomes[index].result, outcomes[index].err = shard.CompactWithResult()
		}
	} else {
		sem := make(chan struct{}, workers)
		var wg sync.WaitGroup
		for index, shard := range shards {
			if err := ctx.Err(); err != nil {
				break
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(index int, shard *Shard) {
				defer wg.Done()
				defer func() { <-sem }()
				if err := ctx.Err(); err != nil {
					outcomes[index] = shardOutcome{
						result: CompactionResult{State: compactionTaskFailed, Error: err.Error()},
						err:    err,
					}
					return
				}
				result, err := shard.CompactWithResult()
				outcomes[index] = shardOutcome{result: result, err: err}
			}(index, shard)
		}
		wg.Wait()
	}
	result := CompactionResult{State: compactionTaskNoop}
	var firstErr error
	for _, outcome := range outcomes {
		result = mergeCompactionResult(result, outcome.result)
		if outcome.err != nil && firstErr == nil {
			firstErr = outcome.err
			result.State = compactionTaskFailed
			result.Error = outcome.err.Error()
		}
	}
	result.Duration = time.Since(started)
	if result.Shards > 0 && result.State == compactionTaskNoop {
		result.State = compactionTaskSucceeded
	}
	if firstErr != nil {
		e.logger.Warn("compaction failed",
			"duration_ms", result.Duration.Milliseconds(),
			"error", firstErr,
			"workers", workers,
		)
		return result, firstErr
	}
	if result.State == compactionTaskSucceeded {
		e.logger.Info("compaction completed",
			"duration_ms", result.Duration.Milliseconds(),
			"input_parts", result.InputParts,
			"output_parts", result.OutputParts,
			"workers", workers,
		)
	}
	return result, nil
}

func (e *Engine) snapshotShards() []*Shard {
	e.mu.Lock()
	defer e.mu.Unlock()
	shards := make([]*Shard, 0, len(e.shards))
	for _, shard := range e.shards {
		shards = append(shards, shard)
	}
	return shards
}

func (e *Engine) compactWorkerLimit() int {
	if e == nil {
		return 1
	}
	if e.opts.MaxConcurrentCompaction > 0 {
		return e.opts.MaxConcurrentCompaction
	}
	return defaultParallelCompactionLimit()
}

func (e *Engine) ApplyRetention(_ context.Context, now time.Time) error {
	if e.opts.Retention <= 0 {
		return nil
	}
	cutoff := now.UnixNano() - int64(e.opts.Retention)
	e.mu.Lock()
	defer e.mu.Unlock()
	for id, shard := range e.shards {
		if err := e.applyRetentionToShardLocked(id, shard, cutoff); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) applyRetentionToShardLocked(id string, shard *Shard, cutoff int64) error {
	if shard.opts.End < cutoff {
		return e.removeExpiredShardLocked(id, shard)
	}
	if shard.opts.Start >= cutoff {
		return nil
	}
	// 分片仍活跃，但包含过期时间范围：下发 tombstone 覆盖 [Start, cutoff)。
	tombstone := model.Tombstone{
		StartTime: shard.opts.Start,
		EndTime:   cutoff - 1,
		WriteSeq:  e.nextWriteSeq(),
	}
	if err := shard.DeleteRange(tombstone, e.opts.FlushSync); err != nil {
		e.retentionDeleteErrors++
		return err
	}
	e.logger.Info("retention range tombstoned",
		"shard", id,
		"start", tombstone.StartTime,
		"end", tombstone.EndTime,
	)
	return nil
}

func (e *Engine) removeExpiredShardLocked(id string, shard *Shard) error {
	shard.lifecycleMu.Lock()
	deletedBytes, sizeErr := directorySize(shard.opts.Dir)
	if sizeErr != nil {
		deletedBytes = 0
	}
	if shard.hasActiveReadersLocked() {
		shard.lifecycleMu.Unlock()
		e.retentionDeleteErrors++
		return ErrShardBusy
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
	e.logger.Info("retention shard removed",
		"shard", id,
		"deleted_bytes", deletedBytes,
	)
	delete(e.shards, id)
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
	// 活跃查询仍引用旧 part 句柄时，跳过会删除输入 part 的 compaction。
	if s.hasActiveReadersLocked() {
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
		reason := compactionSkipDuplicateCandidate
		if snap := s.opts.scheduler.snapshotCopy(); snap.LastSkipReason != "" {
			reason = snap.LastSkipReason
		}
		s.compactionStats.recordSkip(reason)
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
	s.logger.Warn("compaction skipped: insufficient disk space",
		"shard", shardID(s.opts.Database, s.opts.RetentionPolicy, s.opts.Start),
		"available_bytes", available,
		"required_bytes", required,
	)
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
		for _, window := range splitSeriesSampleWindows(group, maxCompactionSeriesSamples) {
			if err := o.addSeriesGroup(window); err != nil {
				return err
			}
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

func splitSeriesSampleWindows(columns []model.ColumnData, maxSamples int) [][]model.ColumnData {
	if maxSamples <= 0 || len(columns) == 0 {
		return [][]model.ColumnData{columns}
	}
	count := 0
	for _, column := range columns {
		if len(column.Samples) > count {
			count = len(column.Samples)
		}
	}
	if count <= maxSamples {
		return [][]model.ColumnData{columns}
	}
	windows := make([][]model.ColumnData, 0, (count+maxSamples-1)/maxSamples)
	for start := 0; start < count; start += maxSamples {
		end := start + maxSamples
		if end > count {
			end = count
		}
		window := make([]model.ColumnData, 0, len(columns))
		for _, column := range columns {
			if start >= len(column.Samples) {
				continue
			}
			columnEnd := end
			if columnEnd > len(column.Samples) {
				columnEnd = len(column.Samples)
			}
			part := column
			part.Samples = column.Samples[start:columnEnd]
			if len(part.Samples) == 0 {
				continue
			}
			window = append(window, part)
		}
		if len(window) > 0 {
			windows = append(windows, window)
		}
	}
	return windows
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
