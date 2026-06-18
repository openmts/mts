package queryexec

import (
	"errors"
	"fmt"

	"github.com/openmts/mts/internal/model"
)

var ErrReadBudgetExceeded = errors.New("read budget exceeded")

type ReadBudgetError struct {
	Metric string
	Actual int
	Limit  int
}

func (e ReadBudgetError) Error() string {
	return fmt.Sprintf("%s: %s actual=%d limit=%d", ErrReadBudgetExceeded, e.Metric, e.Actual, e.Limit)
}

func (e ReadBudgetError) Unwrap() error {
	return ErrReadBudgetExceeded
}

type budgetColumnStream struct {
	source  ColumnStream
	current model.ColumnSeries
	err     error
	samples int
	limit   int
	closed  bool
}

func NewBudgetColumnStream(source ColumnStream, budget model.QueryBudget) ColumnStream {
	return &budgetColumnStream{source: source, limit: budget.MaxSamples}
}

func NewReadBudgetError(metric string, actual int, limit int) error {
	return ReadBudgetError{Metric: metric, Actual: actual, Limit: limit}
}

func (s *budgetColumnStream) Next() bool {
	if s.closed || s.err != nil {
		return false
	}
	if !s.source.Next() {
		s.err = s.source.Err()
		return false
	}
	column := s.source.Column()
	s.samples += len(column.Values)
	if s.limit > 0 && s.samples > s.limit {
		s.err = NewReadBudgetError("samples", s.samples, s.limit)
		return false
	}
	s.current = column
	return true
}

func (s *budgetColumnStream) Column() model.ColumnSeries {
	return s.current
}

func (s *budgetColumnStream) Err() error {
	return s.err
}

func (s *budgetColumnStream) Close() error {
	s.closed = true
	if s.source == nil {
		return nil
	}
	return s.source.Close()
}
