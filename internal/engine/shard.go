package engine

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/openmts/mts/internal/memtable"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/sstable"
	"github.com/openmts/mts/internal/storagefs"
)

type ShardOptions struct {
	Dir                             string
	Database                        string
	RetentionPolicy                 string
	Start                           int64
	End                             int64
	WAL                             model.WALOptions
	FlushSync                       bool
	MemTableMaxSamples              int
	MemTableDisorderFlushRatio      float64
	MemTableDisorderFlushMinSamples int
	Compaction                      model.CompactionOptions
	Compression                     model.CompressionOptions
	Memory                          *storageMemoryLimiter
	scheduler                       *compactionScheduler
	logger                          *slog.Logger
	deps                            shardDeps
}

type Shard struct {
	lifecycleMu sync.RWMutex

	opts            ShardOptions
	wal             walStore
	mem             memStore
	parts           []partReader
	manifest        sstable.Manifest
	tombstones      []model.Tombstone
	nextPart        int
	readRefs        int
	maintenanceErr  error
	recoveryReport  RecoveryReport
	compactionStats compactionStatsRecorder
	deps            shardDeps
	testHooks       shardTestHooks
	logger          *slog.Logger
}

type shardTestHooks struct {
	afterPartWriteBeforeManifest func() error
	afterManifestBeforeWALTrunc  func() error
}

var errFlushSnapshotEmpty = errors.New("flush snapshot has no columns")

func OpenShard(opts ShardOptions) (*Shard, uint64, error) {
	if err := prepareStorageRoot(opts.Dir); err != nil {
		return nil, 0, err
	}
	logger := opts.logger
	if logger == nil {
		logger = nopLogger()
	}
	deps := normalizeShardDeps(opts.deps)
	manifest, err := deps.parts.LoadManifest(opts.Dir)
	if err != nil {
		return nil, 0, err
	}
	shard := &Shard{
		opts:     opts,
		mem:      deps.newMem(),
		manifest: manifest,
		parts:    make([]partReader, 0, len(manifest.Parts)),
		deps:     deps,
		logger:   logger,
	}
	maxSeq, err := shard.openParts()
	if err != nil {
		closeErr := closeParts(shard.parts)
		if closeErr != nil {
			return nil, 0, errors.Join(err, closeErr)
		}
		return nil, 0, err
	}
	shard.recoveryReport.Merge(shard.cleanupOrphanParts())
	shard.maintenanceErr = shard.recoveryReport.MaintenanceError()
	log, err := deps.openWAL(opts.Dir, opts.WAL)
	if err != nil {
		closeErr := closeParts(shard.parts)
		return nil, 0, errors.Join(err, closeErr)
	}
	shard.wal = log
	replayed, err := log.ReplayRecords()
	if err != nil {
		logger.Warn("wal replay failed",
			"shard", shardID(opts.Database, opts.RetentionPolicy, opts.Start),
			"error", err,
		)
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
	logger.Info("shard opened",
		"shard", shardID(opts.Database, opts.RetentionPolicy, opts.Start),
		"wal_records", len(replayed),
	)
	return shard, maxSeq, nil
}

func (s *Shard) Write(point model.ResolvedPoint, syncWrite bool) error {
	return s.WriteBatch([]model.ResolvedPoint{point}, syncWrite)
}

func (s *Shard) WriteBatch(points []model.ResolvedPoint, syncWrite bool) error {
	if len(points) == 0 {
		return nil
	}
	s.lifecycleMu.RLock()
	release, err := s.reserveWriteMemory(points)
	if err != nil {
		s.lifecycleMu.RUnlock()
		return err
	}
	if err := s.wal.Append(points, syncWrite); err != nil {
		release()
		s.lifecycleMu.RUnlock()
		return err
	}
	if err := s.applyMemAfterWAL(points); err != nil {
		release()
		s.lifecycleMu.RUnlock()
		return err
	}
	needFlush := s.shouldFlushMemTable()
	release()
	s.lifecycleMu.RUnlock()
	if needFlush {
		return s.Flush()
	}
	return nil
}

func (s *Shard) WriteTypedBatch(
	batch model.ResolvedTypedBatch,
	rows []int,
	syncWrite bool,
) error {
	if typedBatchRowCount(batch, rows) == 0 {
		return nil
	}
	s.lifecycleMu.RLock()
	release, err := s.reserveTypedWriteMemory(batch, rows)
	if err != nil {
		s.lifecycleMu.RUnlock()
		return err
	}
	if err := s.appendTypedWAL(batch, rows, syncWrite); err != nil {
		release()
		s.lifecycleMu.RUnlock()
		return err
	}
	if err := s.applyTypedMemAfterWAL(batch, rows); err != nil {
		release()
		s.lifecycleMu.RUnlock()
		return err
	}
	needFlush := s.shouldFlushMemTable()
	release()
	s.lifecycleMu.RUnlock()
	if needFlush {
		return s.Flush()
	}
	return nil
}

func (s *Shard) shouldFlushMemTable() bool {
	if s.mem == nil {
		return false
	}
	if s.opts.MemTableMaxSamples > 0 && s.mem.SampleCount() >= s.opts.MemTableMaxSamples {
		return true
	}
	ratio := s.opts.MemTableDisorderFlushRatio
	if ratio <= 0 {
		return false
	}
	minSamples := s.opts.MemTableDisorderFlushMinSamples
	if minSamples <= 0 {
		minSamples = 1024
	}
	appended := s.mem.AppendedSamples()
	if appended < uint64(minSamples) {
		return false
	}
	return s.mem.DisorderRatio() >= ratio
}

func (s *Shard) appendTypedWAL(
	batch model.ResolvedTypedBatch,
	rows []int,
	syncWrite bool,
) error {
	if typed, ok := s.wal.(typedWalStore); ok {
		return typed.AppendTyped(batch, rows, syncWrite)
	}
	return s.wal.Append(resolvedPointsFromTypedBatch(batch, rows), syncWrite)
}

func (s *Shard) applyTypedMemTable(batch model.ResolvedTypedBatch, rows []int) error {
	if typed, ok := s.mem.(typedMemStore); ok {
		return typed.ApplyTypedBatch(batch, rows)
	}
	return s.mem.ApplyBatch(resolvedPointsFromTypedBatch(batch, rows))
}

func resolvedPointsFromTypedBatch(
	batch model.ResolvedTypedBatch,
	rows []int,
) []model.ResolvedPoint {
	count := typedBatchRowCount(batch, rows)
	points := make([]model.ResolvedPoint, count)
	fieldArena := make([]model.ResolvedField, count*len(batch.Fields))
	offset := 0
	for position := range count {
		row := typedBatchRowIndex(rows, position)
		fields := fieldArena[offset : offset+len(batch.Fields)]
		offset += len(batch.Fields)
		for index, column := range batch.Fields {
			fields[index] = resolvedTypedFieldAt(column, row)
		}
		points[position] = resolvedTypedPointAt(batch, row, fields)
	}
	return points
}

func resolvedTypedPointAt(
	batch model.ResolvedTypedBatch,
	row int,
	fields []model.ResolvedField,
) model.ResolvedPoint {
	return model.ResolvedPoint{
		Database:        batch.Database,
		RetentionPolicy: batch.RetentionPolicy,
		Measurement:     batch.Measurement,
		Tags:            typedTagMapAt(batch.Tags, row),
		SeriesID:        batch.SeriesIDs[row],
		Timestamp:       batch.Timestamps[row],
		WriteSeq:        batch.WriteSeqs[row],
		Fields:          fields,
	}
}

func resolvedTypedFieldAt(column model.ResolvedTypedFieldColumn, row int) model.ResolvedField {
	return model.ResolvedField{
		FieldID:   column.FieldID,
		FieldName: column.Name,
		Type:      column.Type,
		Value:     typedFieldValueAt(column, row),
	}
}

func typedTagMapAt(tags []model.TagColumn, row int) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		out[tag.Name] = tag.Values[row]
	}
	return out
}

func typedFieldValueAt(column model.ResolvedTypedFieldColumn, row int) model.FieldValue {
	switch column.Type {
	case model.FieldFloat64:
		return model.Float64Value(column.Float64Values[row])
	case model.FieldInt64:
		return model.Int64Value(column.Int64Values[row])
	case model.FieldString:
		return model.StringValue(column.StringValues[row])
	case model.FieldBool:
		return model.BoolValue(column.BoolValues[row])
	default:
		return model.FieldValue{Type: column.Type}
	}
}

func (s *Shard) ApproxMemorySamples() int {
	s.lifecycleMu.RLock()
	defer s.lifecycleMu.RUnlock()
	return s.mem.ApproxMemorySamples()
}

func (s *Shard) ApproxMemoryBytes() int64 {
	s.lifecycleMu.RLock()
	defer s.lifecycleMu.RUnlock()
	return s.approxMemoryBytesLocked()
}

func (s *Shard) ApproxMemTableMemoryBytes() int64 {
	s.lifecycleMu.RLock()
	defer s.lifecycleMu.RUnlock()
	return s.mem.ApproxMemoryBytes()
}

func (s *Shard) ApproxWALMemoryBytes() int64 {
	s.lifecycleMu.RLock()
	defer s.lifecycleMu.RUnlock()
	return s.approxWALMemoryBytesLocked()
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
	release, err := s.reserveFlushMemory(snapshot)
	if err != nil {
		s.mem.Restore(snapshot)
		return err
	}
	meta, err := s.writeSnapshotPart(snapshot)
	if errors.Is(err, errFlushSnapshotEmpty) {
		release()
		s.mem.Restore(snapshot)
		return nil
	}
	if err != nil {
		release()
		s.mem.Restore(snapshot)
		return err
	}
	if s.testHooks.afterPartWriteBeforeManifest != nil {
		if err := s.testHooks.afterPartWriteBeforeManifest(); err != nil {
			cleanupErr := s.cleanupUncommittedPart(meta, nil)
			release()
			s.mem.Restore(snapshot)
			return errors.Join(err, cleanupErr)
		}
	}
	part, err := s.deps.parts.OpenPartTrusted(meta.Path)
	if err != nil {
		cleanupErr := s.cleanupUncommittedPart(meta, nil)
		release()
		s.mem.Restore(snapshot)
		return errors.Join(err, cleanupErr)
	}
	nextManifest := s.nextManifest(append([]sstable.PartMeta{}, s.manifest.Parts...))
	nextManifest.Parts = append(nextManifest.Parts, meta)
	if err := s.deps.parts.WriteManifest(s.opts.Dir, nextManifest); err != nil {
		cleanupErr := s.cleanupUncommittedPart(meta, part)
		release()
		s.mem.Restore(snapshot)
		return errors.Join(err, cleanupErr)
	}
	s.parts = append(s.parts, part)
	s.manifest = nextManifest
	return s.completeCommittedFlush(snapshot, release)
}

func (s *Shard) writeSnapshotPart(snapshot memSnapshot) (sstable.PartMeta, error) {
	var writer partWriter
	wroteSeries := false
	query := memtable.Query{
		Start: s.opts.Start,
		End:   s.opts.End,
	}
	err := snapshot.ForEachSeries(query, func(_ uint64, columns []model.ColumnData) error {
		if len(columns) == 0 {
			return nil
		}
		if writer == nil {
			nextWriter, writerErr := s.deps.parts.NewWriter(s.opts.Dir, 0, s.nextPartID(), s.flushWriteOptions())
			if writerErr != nil {
				return writerErr
			}
			writer = nextWriter
		}
		if err := writer.AddSeries(columns); err != nil {
			return err
		}
		wroteSeries = true
		return nil
	})
	if err != nil {
		return sstable.PartMeta{}, errors.Join(err, abortPartWriter(writer))
	}
	if writer == nil || !wroteSeries {
		return sstable.PartMeta{}, errFlushSnapshotEmpty
	}
	meta, err := writer.Close()
	if err != nil {
		return sstable.PartMeta{}, errors.Join(err, abortPartWriter(writer))
	}
	return meta, nil
}

func (s *Shard) flushWriteOptions() sstable.WriteOptions {
	// flush 产物落在 L0：未显式指定 Algorithm 时使用分层默认（snappy）。
	compression := defaultLevelCompression(0, s.opts.Compression)
	return sstable.WriteOptions{
		Compression:  compression,
		MemoryBudget: storageCompressionBudget{memory: s.opts.Memory},
		Sync:         s.opts.FlushSync,
	}
}

func abortPartWriter(writer partWriter) error {
	if writer == nil {
		return nil
	}
	return writer.Abort()
}

func (s *Shard) nextManifest(parts []sstable.PartMeta) sstable.Manifest {
	return sstable.Manifest{
		Sequence: s.manifest.Sequence + 1,
		Parts:    parts,
	}
}

func (s *Shard) reserveFlushMemory(snapshot memSnapshot) (func(), error) {
	if s.opts.Memory == nil || snapshot == nil {
		return func() {}, nil
	}
	return s.opts.Memory.Reserve(storageMemoryFlush, 0, snapshot.ApproxMemoryBytes())
}

func (s *Shard) reserveWriteMemory(points []model.ResolvedPoint) (func(), error) {
	if s.opts.Memory == nil {
		return func() {}, nil
	}
	return s.opts.Memory.Reserve(storageMemoryWrite, 0, estimateWALFrameBytes(points))
}

func (s *Shard) reserveTypedWriteMemory(
	batch model.ResolvedTypedBatch,
	rows []int,
) (func(), error) {
	if s.opts.Memory == nil {
		return func() {}, nil
	}
	return s.opts.Memory.Reserve(storageMemoryWrite, 0, estimateTypedWALFrameBytes(batch, rows))
}

func (s *Shard) approxMemoryBytesLocked() int64 {
	return s.mem.ApproxMemoryBytes() + s.approxWALMemoryBytesLocked()
}

func (s *Shard) approxWALMemoryBytesLocked() int64 {
	if s.wal == nil {
		return 0
	}
	return s.wal.ApproxMemoryBytes()
}

func (s *Shard) estimateQueryPartStats(start, end int64) (candidate int, matched int, estimatedRows int64) {
	s.lifecycleMu.RLock()
	defer s.lifecycleMu.RUnlock()
	parts := s.manifest.Parts
	candidate = len(parts)
	for _, meta := range parts {
		if !partTimeOverlaps(meta.MinTime, meta.MaxTime, start, end) {
			continue
		}
		matched++
		estimatedRows += proportionalPartRows(meta, start, end)
	}
	// MemTable 中尚未落盘的样本也计入扫描潜力。
	if s.mem != nil {
		memSamples := s.mem.SampleCount()
		if memSamples > 0 {
			estimatedRows += int64(memSamples)
		}
	}
	return candidate, matched, estimatedRows
}

func partTimeOverlaps(minTime, maxTime, start, end int64) bool {
	return maxTime >= start && minTime <= end
}

func proportionalPartRows(meta sstable.PartMeta, start, end int64) int64 {
	rows := int64(meta.RowsCount)
	if rows <= 0 {
		return 0
	}
	partSpan := meta.MaxTime - meta.MinTime
	if partSpan <= 0 {
		return rows
	}
	// 查询窗与 part 时间范围交集长度 / part 全长 * rows。
	overlapStart := meta.MinTime
	if start > overlapStart {
		overlapStart = start
	}
	overlapEnd := meta.MaxTime
	if end < overlapEnd {
		overlapEnd = end
	}
	if overlapEnd < overlapStart {
		return 0
	}
	overlap := overlapEnd - overlapStart
	if overlap >= partSpan {
		return rows
	}
	scaled := rows * overlap / partSpan
	if scaled <= 0 {
		return 1
	}
	return scaled
}

func (s *Shard) Query(query memtable.Query) ([]model.ColumnData, error) {
	stream, err := s.ScanColumns(query)
	if err != nil {
		return nil, err
	}
	return collectColumnDataStream(stream)
}

func (s *Shard) Close() error {
	return s.closeWithBusyCheck(true)
}

func (s *Shard) closeForced() error {
	return s.closeWithBusyCheck(false)
}

func (s *Shard) closeWithBusyCheck(rejectBusy bool) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if rejectBusy && s.hasActiveReadersLocked() {
		return ErrShardBusy
	}
	return s.closeLocked()
}

func (s *Shard) CompactionStatsSnapshot() CompactionStats {
	return s.compactionStats.snapshot()
}

func (s *Shard) compactionBacklogSnapshot(fallback model.CompactionOptions) (compactionBacklogSnapshot, error) {
	s.lifecycleMu.RLock()
	defer s.lifecycleMu.RUnlock()
	opts := s.opts.Compaction
	if compactionOptionsEmpty(opts) {
		opts = fallback
	}
	return buildCompactionBacklog(s.manifest.Parts, s.tombstones, opts)
}

func compactionOptionsEmpty(opts model.CompactionOptions) bool {
	return opts.Level0PartLimit == 0 && opts.Level0SizeLimit == 0 &&
		opts.MaxOutputPartBytes == 0 && len(opts.Levels) == 0
}

func (s *Shard) RecoveryReport() RecoveryReport {
	s.lifecycleMu.RLock()
	defer s.lifecycleMu.RUnlock()
	return s.recoveryReport.Clone()
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

func closeParts(parts []partReader) error {
	var err error
	for _, part := range parts {
		err = errors.Join(err, part.Close())
	}
	return err
}

func (s *Shard) openParts() (uint64, error) {
	var maxSeq uint64
	for _, meta := range s.manifest.Parts {
		part, err := s.deps.parts.OpenPart(meta.Path)
		if err != nil {
			issue := partOpenRecoveryIssue(meta, err)
			s.logger.Warn("part open recovery issue",
				"shard", shardID(s.opts.Database, s.opts.RetentionPolicy, s.opts.Start),
				"part", meta.ID,
				"kind", string(issue.Kind),
				"error", err,
			)
			s.recoveryReport.Add(issue)
			return 0, s.recoveryReport.FatalError()
		}
		if issue, ok := partMetadataMismatchIssue(meta, part.Meta()); ok {
			s.logger.Warn("part metadata mismatch",
				"shard", shardID(s.opts.Database, s.opts.RetentionPolicy, s.opts.Start),
				"part", meta.ID,
				"message", issue.Message,
			)
			closeErr := part.Close()
			s.recoveryReport.Add(issue)
			return 0, errors.Join(s.recoveryReport.FatalError(), closeErr)
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

func (s *Shard) cleanupOrphanParts() RecoveryReport {
	referenced := make(map[string]struct{}, len(s.manifest.Parts))
	for _, meta := range s.manifest.Parts {
		referenced[filepath.Base(meta.Path)] = struct{}{}
		referenced[meta.ID] = struct{}{}
	}
	entries, err := storagefs.ReadDir(s.opts.Dir)
	if err != nil {
		return RecoveryReport{Issues: []RecoveryIssue{{
			Kind:    RecoveryIssueCleanupScanFailed,
			Path:    s.opts.Dir,
			Message: "scan shard dir for cleanup failed",
			Err:     err,
		}}}
	}
	var report RecoveryReport
	for _, entry := range entries {
		if !isCleanupCandidate(entry, referenced) {
			continue
		}
		report.Add(s.removeCleanupCandidate(entry))
	}
	return report
}

func (s *Shard) cleanupUncommittedPart(meta sstable.PartMeta, part partReader) error {
	closeErr := closePartReader(part)
	removeErr := s.removeUnreferencedPart(meta.Path, "remove uncommitted part failed")
	return errors.Join(closeErr, removeErr)
}

func closePartReader(part partReader) error {
	if part == nil {
		return nil
	}
	return part.Close()
}

func (s *Shard) removeUnreferencedPart(path string, message string) error {
	if path == "" {
		return nil
	}
	if err := s.deps.files.RemoveAll(path); err != nil {
		s.recordRecoveryIssue(RecoveryIssue{
			Kind:    RecoveryIssueOrphanRemoveFailed,
			Path:    path,
			Message: message,
			Err:     err,
		})
		return err
	}
	return nil
}

func (s *Shard) recordRecoveryIssue(issue RecoveryIssue) {
	s.recoveryReport.Add(issue)
	s.maintenanceErr = s.recoveryReport.MaintenanceError()
}

func isPartDirName(name string) bool {
	return len(name) == len("sst-000000") && name[:4] == "sst-"
}

func isCleanupCandidate(entry os.DirEntry, referenced map[string]struct{}) bool {
	name := entry.Name()
	if entry.IsDir() && isPartDirName(name) {
		_, ok := referenced[name]
		return !ok
	}
	return !entry.IsDir() && strings.HasPrefix(name, ".tmp-")
}

func (s *Shard) removeCleanupCandidate(entry os.DirEntry) RecoveryIssue {
	name := entry.Name()
	path := filepath.Join(s.opts.Dir, name)
	kind := cleanupRemovedKind(entry)
	failKind := cleanupRemoveFailedKind(entry)
	message := cleanupRemovedMessage(entry)
	if err := s.deps.files.RemoveAll(path); err != nil {
		return RecoveryIssue{Kind: failKind, Path: path, Message: message + " failed", Err: err}
	}
	return RecoveryIssue{Kind: kind, Path: path, Message: message}
}

func cleanupRemovedKind(entry os.DirEntry) RecoveryIssueKind {
	if entry.IsDir() {
		return RecoveryIssueOrphanPartRemoved
	}
	return RecoveryIssueTempRemoved
}

func cleanupRemoveFailedKind(entry os.DirEntry) RecoveryIssueKind {
	if entry.IsDir() {
		return RecoveryIssueOrphanRemoveFailed
	}
	return RecoveryIssueTempRemoveFailed
}

func cleanupRemovedMessage(entry os.DirEntry) string {
	if entry.IsDir() {
		return "removed orphan part"
	}
	return "removed temp file"
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
