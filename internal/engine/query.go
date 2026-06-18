package engine

import (
	"context"
	"errors"
	"sort"

	"github.com/openmts/mts/internal/catalog"
	"github.com/openmts/mts/internal/memtable"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryexec"
)

type columnIterator struct {
	stream queryexec.ColumnStream
	stats  *model.QueryStats
}

type rowIterator struct {
	stream queryexec.RowStream
}

type queryMemoryColumnDataStream struct {
	source  queryexec.ColumnDataStream
	memory  *storageMemoryLimiter
	current model.ColumnData
	release func()
	err     error
}

func newColumnIterator(stream queryexec.ColumnStream, stats ...*model.QueryStats) *columnIterator {
	iterator := &columnIterator{stream: stream}
	if len(stats) > 0 {
		iterator.stats = stats[0]
	}
	return iterator
}

func newRowIterator(stream queryexec.RowStream) *rowIterator {
	return &rowIterator{stream: stream}
}

func (e *Engine) QueryColumns(ctx context.Context, query model.Query) ([]model.ColumnSeries, error) {
	iter, err := e.QueryColumnIterator(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = iter.Close()
	}()
	columns := make([]model.ColumnSeries, 0)
	var queryBytes int64
	for iter.Next() {
		column := iter.Column()
		queryBytes += estimateColumnSeriesBytes(column)
		if err := e.checkQueryMemoryBudget(queryBytes); err != nil {
			return nil, err
		}
		columns = append(columns, column)
	}
	return columns, iter.Err()
}

func (e *Engine) QueryWithExplain(
	ctx context.Context,
	query model.Query,
) ([]model.ColumnSeries, model.QueryExplain, model.QueryStats, error) {
	plan, err := e.BuildQueryPlan(ctx, query)
	if err != nil {
		return nil, model.QueryExplain{}, model.QueryStats{}, err
	}
	iter, err := e.queryColumnIteratorFromPlan(ctx, plan)
	if err != nil {
		return nil, plan.Explain, model.QueryStats{}, err
	}
	columns, err := e.collectQueryColumns(iter)
	return columns, plan.Explain, iter.Stats(), err
}

func (e *Engine) collectQueryColumns(iter *columnIterator) ([]model.ColumnSeries, error) {
	columns := make([]model.ColumnSeries, 0)
	var queryBytes int64
	for iter.Next() {
		column := iter.Column()
		queryBytes += estimateColumnSeriesBytes(column)
		if err := e.checkQueryMemoryBudget(queryBytes); err != nil {
			return columns, errors.Join(err, iter.Close())
		}
		columns = append(columns, column)
	}
	return columns, errors.Join(iter.Err(), iter.Close())
}

func (e *Engine) QueryColumnIterator(ctx context.Context, query model.Query) (*columnIterator, error) {
	plan, err := e.BuildQueryPlan(ctx, query)
	if err != nil {
		return nil, err
	}
	return e.queryColumnIteratorFromPlan(ctx, plan)
}

func (e *Engine) queryColumnIteratorFromPlan(ctx context.Context, plan QueryPlan) (*columnIterator, error) {
	stats := e.beginQueryStats(plan)
	query := plan.Query
	if plan.Empty {
		stream := queryexec.NewSliceColumnSeriesStream(nil)
		stream = queryexec.WithContextColumnStream(ctx, stream)
		stream = newQueryStatsColumnStream(stream, stats, e.finishQueryStats)
		return newColumnIterator(stream, stats), nil
	}
	if err := ctx.Err(); err != nil {
		e.finishQueryStats(stats, err)
		return nil, err
	}
	snapshot, err := e.metadata.Snapshot(ctx)
	if err != nil {
		e.finishQueryStats(stats, err)
		return nil, err
	}
	raw, err := e.queryColumnDataStream(ctx, plan, stats)
	if err != nil {
		e.finishQueryStats(stats, err)
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		e.finishQueryStats(stats, err)
		return nil, errors.Join(err, raw.Close())
	}
	raw = newQueryMemoryColumnDataStream(raw, e.memory, e.opts.StorageMemory.QueryBytesLimit)
	stream := queryexec.NewDecoratedColumnStream(raw, columnDecorator(snapshot))
	if len(query.Aggregates) > 0 {
		stream = queryexec.NewAggregateColumnStream(stream, query.Aggregates, query.Window)
	}
	if query.Budget.MaxSamples > 0 {
		stream = queryexec.NewBudgetColumnStream(stream, query.Budget)
	}
	if query.Limit > 0 || query.Offset > 0 {
		stream = queryexec.NewPaginatedColumnStream(stream, query.Limit, query.Offset)
	}
	stream = queryexec.WithContextColumnStream(ctx, stream)
	stream = newQueryStatsColumnStream(stream, stats, e.finishQueryStats)
	return newColumnIterator(stream, stats), nil
}

func newQueryMemoryColumnDataStream(
	source queryexec.ColumnDataStream,
	memory *storageMemoryLimiter,
	limit int64,
) queryexec.ColumnDataStream {
	if limit <= 0 && memory == nil {
		return source
	}
	return &queryMemoryColumnDataStream{source: source, memory: memory}
}

func (e *Engine) QueryRows(ctx context.Context, query model.Query) ([]model.Row, error) {
	iter, err := e.QueryRowIterator(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = iter.Close()
	}()
	rows := make([]model.Row, 0)
	for iter.Next() {
		rows = append(rows, iter.Row())
	}
	return rows, iter.Err()
}

func (e *Engine) QueryRowIterator(ctx context.Context, query model.Query) (*rowIterator, error) {
	columnQuery := query
	columnQuery.Limit = 0
	columnQuery.Offset = 0
	columns, err := e.QueryColumnIterator(ctx, columnQuery)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		_ = columns.Close()
		return nil, err
	}
	stream := queryexec.NewRowMergeStream(columns, query)
	return newRowIterator(queryexec.WithContextRowStream(ctx, stream)), nil
}

func (e *Engine) queryColumnDataStream(
	ctx context.Context,
	plan QueryPlan,
	stats *model.QueryStats,
) (queryexec.ColumnDataStream, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query := plan.Query
	if query.Budget.MaxShards > 0 && len(plan.Shards) > query.Budget.MaxShards {
		return nil, queryexec.NewReadBudgetError("shards", len(plan.Shards), query.Budget.MaxShards)
	}
	streams := make([]queryexec.ColumnDataStream, 0, len(plan.Shards))
	for _, shard := range plan.Shards {
		stream, err := shard.ScanColumns(memtable.Query{
			Context:   ctx,
			Budget:    query.Budget,
			Stats:     stats,
			Boundary:  queryBoundaryMode(query),
			SeriesIDs: plan.SeriesIDs,
			FieldIDs:  plan.FieldIDs,
			Start:     query.StartTime,
			End:       query.EndTime,
		})
		if err != nil {
			return nil, errors.Join(err, closeColumnDataStreams(streams))
		}
		streams = append(streams, stream)
	}
	return queryexec.MergeColumnDataStreams(streams...), nil
}

func (e *Engine) queryShardsWithCandidates(query model.Query) ([]*Shard, int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	shards := make([]*Shard, 0, len(e.shards))
	candidates := 0
	for _, shard := range e.shards {
		if shard.opts.Database == query.Database && shard.opts.RetentionPolicy == query.RetentionPolicy {
			candidates++
		}
		if shardMatches(shard, query) {
			shards = append(shards, shard)
		}
	}
	return shards, candidates
}

func decorateColumns(columns []model.ColumnData, snapshot catalog.Snapshot) []model.ColumnSeries {
	out := make([]model.ColumnSeries, 0, len(columns))
	for _, column := range columns {
		if !canDecorateColumn(column, snapshot) {
			continue
		}
		out = append(out, decorateColumnData(column, snapshot))
	}
	return out
}

func canDecorateColumn(column model.ColumnData, snapshot catalog.Snapshot) bool {
	if _, ok := snapshot.Series[column.SeriesID]; !ok {
		return false
	}
	if _, ok := snapshot.Fields[column.FieldID]; !ok {
		return false
	}
	return true
}

func columnDecorator(snapshot catalog.Snapshot) queryexec.ColumnDecorator {
	return func(column model.ColumnData) (model.ColumnSeries, bool) {
		if !canDecorateColumn(column, snapshot) {
			return model.ColumnSeries{}, false
		}
		return decorateColumnData(column, snapshot), true
	}
}

var decorateColumnDataHook func()

func decorateColumnData(column model.ColumnData, snapshot catalog.Snapshot) model.ColumnSeries {
	if decorateColumnDataHook != nil {
		decorateColumnDataHook()
	}
	series, ok := snapshot.Series[column.SeriesID]
	if !ok {
		return model.ColumnSeries{}
	}
	field, ok := snapshot.Fields[column.FieldID]
	if !ok {
		return model.ColumnSeries{}
	}
	return decorateColumn(column, series.Measurement, series.Tags, field.Name)
}

func (i *columnIterator) Next() bool {
	if i.stream == nil {
		return false
	}
	return i.stream.Next()
}

func (i *columnIterator) Column() model.ColumnSeries {
	if i.stream == nil {
		return model.ColumnSeries{}
	}
	return i.stream.Column()
}

func (i *columnIterator) Err() error {
	if i.stream == nil {
		return nil
	}
	return i.stream.Err()
}

func (i *columnIterator) Stats() model.QueryStats {
	if i.stats == nil {
		return model.QueryStats{}
	}
	return *i.stats
}

func (i *columnIterator) Close() error {
	if i.stream == nil {
		return nil
	}
	return i.stream.Close()
}

func (s *queryMemoryColumnDataStream) Next() bool {
	s.releaseCurrent()
	if s.err != nil || s.source == nil || !s.source.Next() {
		return false
	}
	column := s.source.ColumnData()
	release, err := s.reserve(column)
	if err != nil {
		s.err = err
		return false
	}
	s.current = column
	s.release = release
	return true
}

func (s *queryMemoryColumnDataStream) ColumnData() model.ColumnData {
	return s.current
}

func (s *queryMemoryColumnDataStream) Err() error {
	if s.err != nil {
		return s.err
	}
	if s.source == nil {
		return nil
	}
	return s.source.Err()
}

func (s *queryMemoryColumnDataStream) Close() error {
	s.releaseCurrent()
	if s.source == nil {
		return nil
	}
	return s.source.Close()
}

func (s *queryMemoryColumnDataStream) reserve(column model.ColumnData) (func(), error) {
	bytes := estimateColumnBytes(column)
	if s.memory == nil {
		return func() {}, nil
	}
	return s.memory.Reserve(storageMemoryQuery, 0, bytes)
}

func (s *queryMemoryColumnDataStream) releaseCurrent() {
	if s.release == nil {
		return
	}
	s.release()
	s.release = nil
	s.current = model.ColumnData{}
}

func (i *rowIterator) Next() bool {
	if i.stream == nil {
		return false
	}
	return i.stream.Next()
}

func (i *rowIterator) Row() model.Row {
	if i.stream == nil {
		return model.Row{}
	}
	return i.stream.Row()
}

func (i *rowIterator) Err() error {
	if i.stream == nil {
		return nil
	}
	return i.stream.Err()
}

func (i *rowIterator) Close() error {
	if i.stream == nil {
		return nil
	}
	return i.stream.Close()
}

func decorateColumn(
	column model.ColumnData,
	measurement string,
	tags map[string]string,
	fieldName string,
) model.ColumnSeries {
	series := model.ColumnSeries{
		SeriesID:    column.SeriesID,
		Measurement: measurement,
		Tags:        cloneTags(tags),
		FieldID:     column.FieldID,
		FieldName:   fieldName,
		FieldType:   column.FieldType,
		Timestamps:  make([]int64, 0, len(column.Samples)),
		Values:      make([]model.FieldValue, 0, len(column.Samples)),
	}
	for _, sample := range column.Samples {
		series.Timestamps = append(series.Timestamps, sample.Timestamp)
		series.Values = append(series.Values, sample.Value)
	}
	return series
}

func (e *Engine) checkQueryMemoryBudget(queryBytes int64) error {
	limit := e.opts.StorageMemory.QueryBytesLimit
	if limit <= 0 || queryBytes <= limit {
		return nil
	}
	return storageMemoryLimitError(storageMemoryQuery, queryBytes, limit)
}

func estimateColumnSeriesBytes(column model.ColumnSeries) int64 {
	total := int64(64 + len(column.Measurement) + len(column.FieldName))
	for key, value := range column.Tags {
		total += int64(len(key) + len(value) + 32)
	}
	total += int64(cap(column.Timestamps)) * 8
	for _, value := range column.Values {
		total += estimateFieldValueBytes(value)
	}
	return total
}

func estimateFieldValueBytes(value model.FieldValue) int64 {
	switch value.Type {
	case model.FieldFloat64, model.FieldInt64:
		return 8
	case model.FieldString:
		return 16 + int64(len(value.String))
	case model.FieldBool:
		return 1
	default:
		return 0
	}
}

func columnsToRows(columns []model.ColumnSeries) []model.Row {
	if rows, ok := columnsToRowsAligned(columns); ok {
		return rows
	}
	type rowKey struct {
		seriesID  uint64
		timestamp int64
	}
	rowsByKey := make(map[rowKey]model.Row)
	for _, column := range columns {
		for index, timestamp := range column.Timestamps {
			key := rowKey{seriesID: column.SeriesID, timestamp: timestamp}
			row := rowsByKey[key]
			if row.Fields == nil {
				row = newRow(column, timestamp)
			}
			row.Fields[column.FieldName] = column.Values[index]
			rowsByKey[key] = row
		}
	}
	rows := make([]model.Row, 0, len(rowsByKey))
	for _, row := range rowsByKey {
		rows = append(rows, row)
	}
	sortRows(rows)
	return rows
}

func columnsToRowsAligned(columns []model.ColumnSeries) ([]model.Row, bool) {
	if len(columns) == 0 {
		return []model.Row{}, true
	}
	rows := make([]model.Row, 0)
	for start := 0; start < len(columns); {
		end := start + 1
		for end < len(columns) && columns[end].SeriesID == columns[start].SeriesID {
			end++
		}
		groupRows, ok := alignedSeriesRows(columns[start:end])
		if !ok {
			return nil, false
		}
		rows = append(rows, groupRows...)
		start = end
	}
	return rows, true
}

func alignedSeriesRows(columns []model.ColumnSeries) ([]model.Row, bool) {
	if len(columns) == 0 {
		return []model.Row{}, true
	}
	first := columns[0]
	if len(first.Timestamps) != len(first.Values) {
		return nil, false
	}
	for _, column := range columns[1:] {
		if len(column.Timestamps) != len(first.Timestamps) || len(column.Values) != len(column.Timestamps) {
			return nil, false
		}
		for index, timestamp := range column.Timestamps {
			if timestamp != first.Timestamps[index] {
				return nil, false
			}
		}
	}
	rows := make([]model.Row, 0, len(first.Timestamps))
	for index, timestamp := range first.Timestamps {
		row := newRow(first, timestamp)
		for _, column := range columns {
			row.Fields[column.FieldName] = column.Values[index]
		}
		rows = append(rows, row)
	}
	return rows, true
}

func newRow(column model.ColumnSeries, timestamp int64) model.Row {
	return model.Row{
		SeriesID:    column.SeriesID,
		Measurement: column.Measurement,
		Tags:        cloneTags(column.Tags),
		Timestamp:   timestamp,
		Fields:      make(map[string]model.FieldValue),
	}
}

func sortRows(rows []model.Row) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].SeriesID != rows[j].SeriesID {
			return rows[i].SeriesID < rows[j].SeriesID
		}
		return rows[i].Timestamp < rows[j].Timestamp
	})
}

func shardMatches(shard *Shard, query model.Query) bool {
	if shard.opts.Database != query.Database || shard.opts.RetentionPolicy != query.RetentionPolicy {
		return false
	}
	return shard.opts.End >= query.StartTime && shard.opts.Start <= query.EndTime
}

func idSet(ids []uint64) map[uint64]struct{} {
	out := make(map[uint64]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

func cloneTags(tags map[string]string) map[string]string {
	out := make(map[string]string, len(tags))
	for key, value := range tags {
		out[key] = value
	}
	return out
}
