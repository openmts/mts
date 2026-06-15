package engine

import (
	"context"
	"sort"

	"codeberg.org/mts/mts/internal/memtable"
	"codeberg.org/mts/mts/internal/model"
)

func (e *Engine) QueryColumns(_ context.Context, query model.Query) ([]model.ColumnSeries, error) {
	query = normalizeQuery(e.opts, query)
	seriesIDs := e.catalog.MatchSeries(query.Measurement, query.Tags)
	if len(seriesIDs) == 0 {
		return []model.ColumnSeries{}, nil
	}
	fieldIDs := e.catalog.FieldIDs(query.Measurement, query.Fields)
	if len(query.Fields) > 0 && len(fieldIDs) == 0 {
		return []model.ColumnSeries{}, nil
	}
	raw, err := e.queryColumnData(query, idSet(seriesIDs), fieldIDs)
	if err != nil {
		return nil, err
	}
	return e.decorateColumns(raw), nil
}

func (e *Engine) QueryRows(ctx context.Context, query model.Query) ([]model.Row, error) {
	columns, err := e.QueryColumns(ctx, query)
	if err != nil {
		return nil, err
	}
	return columnsToRows(columns), nil
}

func (e *Engine) queryColumnData(
	query model.Query,
	seriesIDs map[uint64]struct{},
	fieldIDs map[uint32]struct{},
) ([]model.ColumnData, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	columns := make([]model.ColumnData, 0)
	for _, shard := range e.shards {
		if !shardMatches(shard, query) {
			continue
		}
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

func (e *Engine) decorateColumns(columns []model.ColumnData) []model.ColumnSeries {
	out := make([]model.ColumnSeries, 0, len(columns))
	for _, column := range columns {
		series, ok := e.catalog.Series(column.SeriesID)
		if !ok {
			continue
		}
		field, ok := e.catalog.Field(column.FieldID)
		if !ok {
			continue
		}
		out = append(out, decorateColumn(column, series.Measurement, series.Tags, field.Name))
	}
	return out
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
