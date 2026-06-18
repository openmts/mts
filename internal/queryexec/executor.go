package queryexec

import (
	"context"

	"github.com/openmts/mts/internal/model"
)

type decoratedColumnStream struct {
	raw        ColumnDataStream
	decorator  ColumnDecorator
	current    model.ColumnSeries
	hasCurrent bool
	positioned bool
}

type sliceColumnDataStream struct {
	columns []model.ColumnData
	index   int
	closed  bool
}

type errorColumnDataStream struct {
	err error
}

type contextColumnDataStream struct {
	ctx    context.Context
	inner  ColumnDataStream
	err    error
	closed bool
}

type sliceColumnSeriesStream struct {
	columns []model.ColumnSeries
	index   int
	closed  bool
}

type sliceRowStream struct {
	rows   []model.Row
	index  int
	closed bool
}

func NewSliceColumnStream(columns []model.ColumnData, decorator ColumnDecorator) ColumnStream {
	return NewDecoratedColumnStream(NewSliceColumnDataStream(columns), decorator)
}

func NewDecoratedColumnStream(raw ColumnDataStream, decorator ColumnDecorator) ColumnStream {
	return &decoratedColumnStream{
		raw:       raw,
		decorator: decorator,
	}
}

func NewSliceColumnDataStream(columns []model.ColumnData) ColumnDataStream {
	return &sliceColumnDataStream{columns: columns}
}

func NewErrorColumnDataStream(err error) ColumnDataStream {
	return &errorColumnDataStream{err: err}
}

func WithContextColumnDataStream(ctx context.Context, inner ColumnDataStream) ColumnDataStream {
	if ctx == nil {
		return inner
	}
	return &contextColumnDataStream{ctx: ctx, inner: inner}
}

func NewSliceColumnSeriesStream(columns []model.ColumnSeries) ColumnStream {
	return &sliceColumnSeriesStream{columns: columns}
}

func NewSliceRowStream(rows []model.Row) RowStream {
	return &sliceRowStream{rows: rows}
}

func (s *decoratedColumnStream) Next() bool {
	if s.raw == nil {
		return false
	}
	ok := s.raw.Next()
	s.current = model.ColumnSeries{}
	s.hasCurrent = false
	s.positioned = ok
	return ok
}

func (s *decoratedColumnStream) Column() model.ColumnSeries {
	if s.raw == nil || !s.positioned {
		return model.ColumnSeries{}
	}
	if s.hasCurrent {
		return s.current
	}
	if s.decorator == nil {
		return model.ColumnSeries{}
	}
	current, ok := s.decorator(s.raw.ColumnData())
	if !ok {
		return model.ColumnSeries{}
	}
	s.current = current
	s.hasCurrent = true
	return current
}

func (s *decoratedColumnStream) Err() error {
	if s.raw == nil {
		return nil
	}
	return s.raw.Err()
}

func (s *decoratedColumnStream) Close() error {
	if s.raw == nil {
		return nil
	}
	return s.raw.Close()
}

func (s *sliceColumnDataStream) Next() bool {
	if s.closed || s.index >= len(s.columns) {
		return false
	}
	s.index++
	return true
}

func (s *sliceColumnDataStream) ColumnData() model.ColumnData {
	if s.index == 0 || s.index > len(s.columns) {
		return model.ColumnData{}
	}
	return s.columns[s.index-1]
}

func (s *sliceColumnDataStream) Err() error {
	return nil
}

func (s *sliceColumnDataStream) Close() error {
	s.columns = nil
	s.closed = true
	return nil
}

func (s *errorColumnDataStream) Next() bool {
	return false
}

func (s *errorColumnDataStream) ColumnData() model.ColumnData {
	return model.ColumnData{}
}

func (s *errorColumnDataStream) Err() error {
	return s.err
}

func (s *errorColumnDataStream) Close() error {
	return nil
}

func (s *contextColumnDataStream) Next() bool {
	if s.err != nil || s.inner == nil {
		return false
	}
	if err := s.ctx.Err(); err != nil {
		s.err = err
		_ = s.Close()
		return false
	}
	return s.inner.Next()
}

func (s *contextColumnDataStream) ColumnData() model.ColumnData {
	if s.inner == nil {
		return model.ColumnData{}
	}
	return s.inner.ColumnData()
}

func (s *contextColumnDataStream) Err() error {
	if s.err != nil {
		return s.err
	}
	if s.ctx != nil {
		if err := s.ctx.Err(); err != nil {
			return err
		}
	}
	if s.inner == nil {
		return nil
	}
	return s.inner.Err()
}

func (s *contextColumnDataStream) Close() error {
	if s.closed || s.inner == nil {
		return nil
	}
	s.closed = true
	return s.inner.Close()
}

func (s *sliceColumnSeriesStream) Next() bool {
	if s.closed || s.index >= len(s.columns) {
		return false
	}
	s.index++
	return true
}

func (s *sliceColumnSeriesStream) Column() model.ColumnSeries {
	if s.index == 0 || s.index > len(s.columns) {
		return model.ColumnSeries{}
	}
	return s.columns[s.index-1]
}

func (s *sliceColumnSeriesStream) Err() error {
	return nil
}

func (s *sliceColumnSeriesStream) Close() error {
	s.columns = nil
	s.closed = true
	return nil
}

func (s *sliceRowStream) Next() bool {
	if s.closed || s.index >= len(s.rows) {
		return false
	}
	s.index++
	return true
}

func (s *sliceRowStream) Row() model.Row {
	if s.index == 0 || s.index > len(s.rows) {
		return model.Row{}
	}
	return s.rows[s.index-1]
}

func (s *sliceRowStream) Err() error {
	return nil
}

func (s *sliceRowStream) Close() error {
	s.rows = nil
	s.closed = true
	return nil
}
