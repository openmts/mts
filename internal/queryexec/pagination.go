package queryexec

import "github.com/openmts/mts/internal/model"

type paginatedColumnStream struct {
	source    ColumnStream
	current   model.ColumnSeries
	limit     int
	offset    int
	returned  int
	skipped   int
	closed    bool
	exhausted bool
}

func NewPaginatedColumnStream(source ColumnStream, limit int, offset int) ColumnStream {
	return &paginatedColumnStream{
		source: source,
		limit:  limit,
		offset: offset,
	}
}

func (s *paginatedColumnStream) Next() bool {
	if s.closed || s.exhausted {
		return false
	}
	for s.source.Next() {
		column := s.pageColumn(s.source.Column())
		if len(column.Values) == 0 {
			continue
		}
		s.current = column
		return true
	}
	return false
}

func (s *paginatedColumnStream) Column() model.ColumnSeries {
	return s.current
}

func (s *paginatedColumnStream) Err() error {
	if s.source == nil {
		return nil
	}
	return s.source.Err()
}

func (s *paginatedColumnStream) Close() error {
	s.closed = true
	if s.source == nil {
		return nil
	}
	return s.source.Close()
}

func (s *paginatedColumnStream) pageColumn(column model.ColumnSeries) model.ColumnSeries {
	start := 0
	for start < len(column.Values) && s.skipped < s.offset {
		start++
		s.skipped++
	}
	end := len(column.Values)
	if s.limitSet() {
		remaining := s.limit - s.returned
		if remaining <= 0 {
			s.exhausted = true
			return model.ColumnSeries{}
		}
		if start+remaining < end {
			end = start + remaining
			s.exhausted = true
		}
	}
	s.returned += end - start
	column.Timestamps = append([]int64(nil), column.Timestamps[start:end]...)
	column.Values = append([]model.FieldValue(nil), column.Values[start:end]...)
	return column
}

func (s *paginatedColumnStream) limitSet() bool {
	return s.limit > 0
}
