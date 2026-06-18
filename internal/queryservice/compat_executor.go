package queryservice

import (
	"context"

	"github.com/openmts/mts/internal/model"
)

type ColumnRowReader interface {
	QueryColumns(ctx context.Context, query model.Query) ([]model.ColumnSeries, error)
	QueryRows(ctx context.Context, query model.Query) ([]model.Row, error)
}

type CompatExecutor struct {
	reader ColumnRowReader
}

func NewCompatExecutor(reader ColumnRowReader) CompatExecutor {
	return CompatExecutor{reader: reader}
}

func (e CompatExecutor) Query(ctx context.Context, query model.Query) (Result, error) {
	if e.reader == nil {
		return Result{}, nil
	}
	if len(query.Aggregates) > 0 {
		return e.queryColumns(ctx, query)
	}
	return e.queryRows(ctx, query)
}

func (e CompatExecutor) queryColumns(ctx context.Context, query model.Query) (Result, error) {
	columns, err := e.reader.QueryColumns(ctx, query)
	if err != nil {
		return Result{}, err
	}
	return Result{Columns: columns}, nil
}

func (e CompatExecutor) queryRows(ctx context.Context, query model.Query) (Result, error) {
	rows, err := e.reader.QueryRows(ctx, query)
	if err != nil {
		return Result{}, err
	}
	return Result{Rows: rows}, nil
}
