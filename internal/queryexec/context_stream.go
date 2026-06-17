package queryexec

import (
	"context"

	"codeberg.org/mts/mts/internal/model"
)

type contextColumnStream struct {
	ctx    context.Context
	inner  ColumnStream
	err    error
	closed bool
}

type contextRowStream struct {
	ctx    context.Context
	inner  RowStream
	err    error
	closed bool
}

func WithContextColumnStream(ctx context.Context, inner ColumnStream) ColumnStream {
	if ctx == nil {
		return inner
	}
	return &contextColumnStream{ctx: ctx, inner: inner}
}

func WithContextRowStream(ctx context.Context, inner RowStream) RowStream {
	if ctx == nil {
		return inner
	}
	return &contextRowStream{ctx: ctx, inner: inner}
}

func (s *contextColumnStream) Next() bool {
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

func (s *contextColumnStream) Column() model.ColumnSeries {
	if s.inner == nil {
		return model.ColumnSeries{}
	}
	return s.inner.Column()
}

func (s *contextColumnStream) Err() error {
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

func (s *contextColumnStream) Close() error {
	if s.closed || s.inner == nil {
		return nil
	}
	s.closed = true
	return s.inner.Close()
}

func (s *contextRowStream) Next() bool {
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

func (s *contextRowStream) Row() model.Row {
	if s.inner == nil {
		return model.Row{}
	}
	return s.inner.Row()
}

func (s *contextRowStream) Err() error {
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

func (s *contextRowStream) Close() error {
	if s.closed || s.inner == nil {
		return nil
	}
	s.closed = true
	return s.inner.Close()
}
