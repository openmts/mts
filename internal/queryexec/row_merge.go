package queryexec

import (
	"fmt"
	"sort"

	"codeberg.org/mts/mts/internal/model"
)

type rowMergeStream struct {
	source       ColumnStream
	query        model.Query
	lookahead    model.ColumnSeries
	hasLookahead bool
	pending      []model.Row
	current      model.Row
	err          error
	pendingAt    int
	skipped      int
	returned     int
	closed       bool
	sourceClosed bool
	exhausted    bool
}

func NewRowMergeStream(source ColumnStream, query model.Query) RowStream {
	return &rowMergeStream{source: source, query: query}
}

func (s *rowMergeStream) Next() bool {
	if s.closed || s.err != nil || s.exhausted {
		return false
	}
	for {
		if s.nextPendingRow() {
			return true
		}
		if !s.loadNextSeriesRows() {
			return false
		}
	}
}

func (s *rowMergeStream) Row() model.Row {
	return cloneRow(s.current)
}

func (s *rowMergeStream) Err() error {
	return s.err
}

func (s *rowMergeStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	s.pending = nil
	return s.closeSource()
}

func (s *rowMergeStream) nextPendingRow() bool {
	for s.pendingAt < len(s.pending) {
		row := s.pending[s.pendingAt]
		s.pendingAt++
		if s.skipped < s.query.Offset {
			s.skipped++
			continue
		}
		if s.limitReached() {
			s.exhausted = true
			_ = s.closeSource()
			return false
		}
		s.returned++
		s.current = row
		if s.limitReached() {
			s.exhausted = true
			_ = s.closeSource()
		}
		return true
	}
	return false
}

func (s *rowMergeStream) loadNextSeriesRows() bool {
	columns, ok := s.readSeriesColumns()
	if !ok {
		return false
	}
	rows, err := rowsFromSeriesColumns(columns)
	if err != nil {
		s.err = err
		return false
	}
	s.pending = rows
	s.pendingAt = 0
	return true
}

func (s *rowMergeStream) readSeriesColumns() ([]model.ColumnSeries, bool) {
	first, ok := s.nextColumn()
	if !ok {
		return nil, false
	}
	columns := []model.ColumnSeries{first}
	for {
		column, ok := s.nextColumn()
		if !ok {
			return columns, s.err == nil
		}
		if column.SeriesID != first.SeriesID {
			s.lookahead = column
			s.hasLookahead = true
			return columns, true
		}
		columns = append(columns, column)
	}
}

func (s *rowMergeStream) nextColumn() (model.ColumnSeries, bool) {
	if s.hasLookahead {
		s.hasLookahead = false
		return s.lookahead, true
	}
	if s.source == nil || !s.source.Next() {
		if s.source != nil {
			s.err = s.source.Err()
		}
		return model.ColumnSeries{}, false
	}
	return s.source.Column(), true
}

func (s *rowMergeStream) limitReached() bool {
	return s.query.Limit > 0 && s.returned >= s.query.Limit
}

func (s *rowMergeStream) closeSource() error {
	if s.sourceClosed || s.source == nil {
		return nil
	}
	s.sourceClosed = true
	return s.source.Close()
}

func rowsFromSeriesColumns(columns []model.ColumnSeries) ([]model.Row, error) {
	if len(columns) == 0 {
		return []model.Row{}, nil
	}
	if err := validateColumnLengths(columns); err != nil {
		return nil, err
	}
	if alignedSeriesColumns(columns) {
		return alignedRowsFromSeriesColumns(columns), nil
	}
	return mappedRowsFromSeriesColumns(columns), nil
}

func validateColumnLengths(columns []model.ColumnSeries) error {
	for _, column := range columns {
		if len(column.Timestamps) != len(column.Values) {
			return fmt.Errorf("column %s has %d timestamps and %d values", column.FieldName, len(column.Timestamps), len(column.Values))
		}
	}
	return nil
}

func alignedSeriesColumns(columns []model.ColumnSeries) bool {
	first := columns[0]
	for _, column := range columns[1:] {
		if len(column.Timestamps) != len(first.Timestamps) {
			return false
		}
		for index, timestamp := range column.Timestamps {
			if timestamp != first.Timestamps[index] {
				return false
			}
		}
	}
	return true
}

func alignedRowsFromSeriesColumns(columns []model.ColumnSeries) []model.Row {
	first := columns[0]
	rows := make([]model.Row, 0, len(first.Timestamps))
	for index, timestamp := range first.Timestamps {
		row := newRowFromColumn(first, timestamp)
		for _, column := range columns {
			row.Fields[column.FieldName] = column.Values[index]
		}
		rows = append(rows, row)
	}
	return rows
}

func mappedRowsFromSeriesColumns(columns []model.ColumnSeries) []model.Row {
	rows := make([]model.Row, 0)
	indexByTimestamp := make(map[int64]int)
	for _, column := range columns {
		appendColumnRows(&rows, indexByTimestamp, column)
	}
	sort.Slice(rows, func(i int, j int) bool {
		return rows[i].Timestamp < rows[j].Timestamp
	})
	return rows
}

func appendColumnRows(rows *[]model.Row, indexByTimestamp map[int64]int, column model.ColumnSeries) {
	for index, timestamp := range column.Timestamps {
		rowIndex, ok := indexByTimestamp[timestamp]
		if !ok {
			*rows = append(*rows, newRowFromColumn(column, timestamp))
			rowIndex = len(*rows) - 1
			indexByTimestamp[timestamp] = rowIndex
		}
		(*rows)[rowIndex].Fields[column.FieldName] = column.Values[index]
	}
}

func newRowFromColumn(column model.ColumnSeries, timestamp int64) model.Row {
	return model.Row{
		SeriesID:    column.SeriesID,
		Measurement: column.Measurement,
		Tags:        cloneStringMap(column.Tags),
		Timestamp:   timestamp,
		Fields:      make(map[string]model.FieldValue),
	}
}

func cloneRow(row model.Row) model.Row {
	row.Tags = cloneStringMap(row.Tags)
	row.Fields = cloneFieldMap(row.Fields)
	return row
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func cloneFieldMap(values map[string]model.FieldValue) map[string]model.FieldValue {
	if values == nil {
		return nil
	}
	out := make(map[string]model.FieldValue, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
