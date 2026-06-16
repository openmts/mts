package engine

import (
	"context"
	"sort"

	"codeberg.org/mts/mts/internal/catalog"
	"codeberg.org/mts/mts/internal/memtable"
	"codeberg.org/mts/mts/internal/model"
)

type columnIterator struct {
	columns []model.ColumnSeries
	index   int
}

type rowIterator struct {
	rows  []model.Row
	index int
}

func (e *Engine) QueryColumns(_ context.Context, query model.Query) ([]model.ColumnSeries, error) {
	iter, err := e.QueryColumnIterator(context.Background(), query)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = iter.Close()
	}()
	columns := make([]model.ColumnSeries, 0)
	for iter.Next() {
		columns = append(columns, iter.Column())
	}
	return columns, iter.Err()
}

func (e *Engine) QueryColumnIterator(_ context.Context, query model.Query) (*columnIterator, error) {
	query = normalizeQuery(e.opts, query)
	seriesIDs := e.catalog.MatchSeries(query.Measurement, query.Tags)
	if len(seriesIDs) == 0 {
		return &columnIterator{}, nil
	}
	fieldIDs := e.catalog.FieldIDs(query.Measurement, query.Fields)
	if len(query.Fields) > 0 && len(fieldIDs) == 0 {
		return &columnIterator{}, nil
	}
	snapshot := e.catalog.Snapshot()
	raw, err := e.queryColumnData(query, idSet(seriesIDs), fieldIDs)
	if err != nil {
		return nil, err
	}
	return &columnIterator{columns: decorateColumns(raw, snapshot)}, nil
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
	columns, err := e.QueryColumns(ctx, query)
	if err != nil {
		return nil, err
	}
	return &rowIterator{rows: columnsToRows(columns)}, nil
}

func (e *Engine) queryColumnData(
	query model.Query,
	seriesIDs map[uint64]struct{},
	fieldIDs map[uint32]struct{},
) ([]model.ColumnData, error) {
	shards := e.queryShards(query)
	columns := make([]model.ColumnData, 0)
	for _, shard := range shards {
		got, err := shard.Query(memtable.Query{
			SeriesIDs: seriesIDs,
			FieldIDs:  fieldIDs,
			Start:     query.StartTime,
			End:       query.EndTime,
		})
		if err != nil {
			return nil, err
		}
		columns = append(columns, got...)
	}
	return mergeColumnData(columns), nil
}

func (e *Engine) queryShards(query model.Query) []*Shard {
	e.mu.Lock()
	defer e.mu.Unlock()
	shards := make([]*Shard, 0, len(e.shards))
	for _, shard := range e.shards {
		if shardMatches(shard, query) {
			shards = append(shards, shard)
		}
	}
	return shards
}

func decorateColumns(columns []model.ColumnData, snapshot catalog.Snapshot) []model.ColumnSeries {
	out := make([]model.ColumnSeries, 0, len(columns))
	for _, column := range columns {
		series, ok := snapshot.Series[column.SeriesID]
		if !ok {
			continue
		}
		field, ok := snapshot.Fields[column.FieldID]
		if !ok {
			continue
		}
		out = append(out, decorateColumn(column, series.Measurement, series.Tags, field.Name))
	}
	return out
}

func (i *columnIterator) Next() bool {
	if i.index >= len(i.columns) {
		return false
	}
	i.index++
	return true
}

func (i *columnIterator) Column() model.ColumnSeries {
	if i.index == 0 || i.index > len(i.columns) {
		return model.ColumnSeries{}
	}
	return i.columns[i.index-1]
}

func (i *columnIterator) Err() error {
	return nil
}

func (i *columnIterator) Close() error {
	i.columns = nil
	return nil
}

func (i *rowIterator) Next() bool {
	if i.index >= len(i.rows) {
		return false
	}
	i.index++
	return true
}

func (i *rowIterator) Row() model.Row {
	if i.index == 0 || i.index > len(i.rows) {
		return model.Row{}
	}
	return i.rows[i.index-1]
}

func (i *rowIterator) Err() error {
	return nil
}

func (i *rowIterator) Close() error {
	i.rows = nil
	return nil
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
