package queryservice

import (
	"context"
	"sync/atomic"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryexec"
)

type StreamingExecutor interface {
	QueryStream(ctx context.Context, query model.Query) (StreamResult, error)
}

type StreamResult struct {
	Rows              queryexec.RowStream
	Columns           queryexec.ColumnStream
	Profile           queryexec.Profile
	Explain           model.QueryExplain
	Stats             model.QueryStats
	LogicalPlanRoot   string
	PhysicalOperators []string
	Pushdowns         []string
}

type releaseRowStream struct {
	inner   queryexec.RowStream
	release func()
	closed  atomic.Bool
}

type releaseColumnStream struct {
	inner   queryexec.ColumnStream
	release func()
	closed  atomic.Bool
}

func withStreamRelease(result StreamResult, release func()) StreamResult {
	if result.Rows != nil {
		result.Rows = &releaseRowStream{inner: result.Rows, release: release}
		return result
	}
	if result.Columns != nil {
		result.Columns = &releaseColumnStream{inner: result.Columns, release: release}
		return result
	}
	release()
	return result
}

func (s *releaseRowStream) Next() bool {
	if s.inner == nil {
		s.releaseOnce()
		return false
	}
	if s.inner.Next() {
		return true
	}
	_ = s.Close()
	return false
}

func (s *releaseRowStream) Row() model.Row {
	if s.inner == nil {
		return model.Row{}
	}
	return s.inner.Row()
}

func (s *releaseRowStream) Err() error {
	if s.inner == nil {
		return nil
	}
	return s.inner.Err()
}

func (s *releaseRowStream) Close() error {
	if s.inner == nil {
		s.releaseOnce()
		return nil
	}
	err := s.inner.Close()
	s.releaseOnce()
	return err
}

func (s *releaseRowStream) releaseOnce() {
	if s.release != nil && s.closed.CompareAndSwap(false, true) {
		s.release()
	}
}

func (s *releaseColumnStream) Next() bool {
	if s.inner == nil {
		s.releaseOnce()
		return false
	}
	if s.inner.Next() {
		return true
	}
	_ = s.Close()
	return false
}

func (s *releaseColumnStream) Column() model.ColumnSeries {
	if s.inner == nil {
		return model.ColumnSeries{}
	}
	return s.inner.Column()
}

func (s *releaseColumnStream) Err() error {
	if s.inner == nil {
		return nil
	}
	return s.inner.Err()
}

func (s *releaseColumnStream) Close() error {
	if s.inner == nil {
		s.releaseOnce()
		return nil
	}
	err := s.inner.Close()
	s.releaseOnce()
	return err
}

func (s *releaseColumnStream) releaseOnce() {
	if s.release != nil && s.closed.CompareAndSwap(false, true) {
		s.release()
	}
}
