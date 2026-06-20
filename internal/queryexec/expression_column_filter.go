package queryexec

import "github.com/openmts/mts/internal/model"

type exprFilteredColumnStream struct {
	source       ColumnStream
	expr         model.QueryExpr
	lookahead    model.ColumnSeries
	hasLookahead bool
	pending      []model.ColumnSeries
	current      model.ColumnSeries
	index        int
	err          error
	closed       bool
}

func NewExprFilteredColumnStream(source ColumnStream, expr model.QueryExpr) ColumnStream {
	if expr.Kind == model.QueryExprNone {
		return source
	}
	return &exprFilteredColumnStream{source: source, expr: expr}
}

func (s *exprFilteredColumnStream) Next() bool {
	if s.closed || s.err != nil {
		return false
	}
	for {
		if s.nextPending() {
			return true
		}
		if !s.loadNextSeries() {
			return false
		}
	}
}

func (s *exprFilteredColumnStream) Column() model.ColumnSeries {
	return s.current
}

func (s *exprFilteredColumnStream) Err() error {
	return s.err
}

func (s *exprFilteredColumnStream) Close() error {
	s.closed = true
	s.pending = nil
	if s.source == nil {
		return nil
	}
	return s.source.Close()
}

func (s *exprFilteredColumnStream) nextPending() bool {
	if s.index >= len(s.pending) {
		return false
	}
	s.current = s.pending[s.index]
	s.index++
	return true
}

func (s *exprFilteredColumnStream) loadNextSeries() bool {
	columns, ok := s.readSeriesColumns()
	if !ok {
		return false
	}
	s.pending, s.err = filterSeriesColumnsByExpr(columns, s.expr)
	s.index = 0
	return s.err == nil
}

func (s *exprFilteredColumnStream) readSeriesColumns() ([]model.ColumnSeries, bool) {
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

func (s *exprFilteredColumnStream) nextColumn() (model.ColumnSeries, bool) {
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

func filterSeriesColumnsByExpr(
	columns []model.ColumnSeries,
	expr model.QueryExpr,
) ([]model.ColumnSeries, error) {
	rows, err := rowsFromSeriesColumns(columns)
	if err != nil {
		return nil, err
	}
	write := 0
	for _, row := range rows {
		if rowMatchesExpr(row, expr) {
			rows[write] = row
			write++
		}
	}
	return columnsFromFilteredRows(columns, rows[:write]), nil
}

func columnsFromFilteredRows(
	templates []model.ColumnSeries,
	rows []model.Row,
) []model.ColumnSeries {
	if len(rows) == 0 {
		return nil
	}
	out := make([]model.ColumnSeries, 0, len(templates))
	for _, template := range templates {
		column := template
		column.Timestamps = column.Timestamps[:0]
		column.Values = column.Values[:0]
		for _, row := range rows {
			value, ok := row.Fields[template.FieldName]
			if !ok {
				continue
			}
			column.Timestamps = append(column.Timestamps, row.Timestamp)
			column.Values = append(column.Values, value)
		}
		if len(column.Values) > 0 {
			out = append(out, column)
		}
	}
	return out
}
