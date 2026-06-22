package queryexec

import (
	"errors"
	"fmt"

	"github.com/openmts/mts/internal/model"
)

var ErrReadBudgetExceeded = errors.New("read budget exceeded")

var ErrUnsupportedAggregate = errors.New("unsupported aggregate")

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

type budgetRowStream struct {
	source  RowStream
	current model.Row
	err     error
	rows    int
	limit   int
	closed  bool
}

func NewBudgetColumnStream(source ColumnStream, budget model.QueryBudget) ColumnStream {
	return &budgetColumnStream{source: source, limit: budget.MaxSamples}
}

func NewBudgetRowStream(source RowStream, budget model.QueryBudget) RowStream {
	return &budgetRowStream{source: source, limit: budget.MaxSamples}
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

func (s *budgetRowStream) Next() bool {
	if s.closed || s.err != nil {
		return false
	}
	if !s.source.Next() {
		s.err = s.source.Err()
		return false
	}
	s.rows++
	if s.limit > 0 && s.rows > s.limit {
		s.err = NewReadBudgetError("rows", s.rows, s.limit)
		_ = s.Close()
		return false
	}
	s.current = s.source.Row()
	return true
}

func (s *budgetRowStream) Row() model.Row {
	return s.current
}

func (s *budgetRowStream) Err() error {
	return s.err
}

func (s *budgetRowStream) Close() error {
	s.closed = true
	if s.source == nil {
		return nil
	}
	return s.source.Close()
}
