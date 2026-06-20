package engine

import (
	"context"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryexec"
	"github.com/openmts/mts/internal/querylang"
)

func (e *Engine) QuerySpecColumns(
	ctx context.Context,
	spec querylang.QuerySpec,
) ([]model.ColumnSeries, error) {
	return e.QueryColumns(ctx, spec.ToModelQuery())
}

func (e *Engine) QuerySpecRows(ctx context.Context, spec querylang.QuerySpec) ([]model.Row, error) {
	return e.QueryRows(ctx, spec.ToModelQuery())
}

func (e *Engine) QuerySpecColumnStream(
	ctx context.Context,
	spec querylang.QuerySpec,
) (queryexec.ColumnStream, error) {
	return e.QueryColumnIterator(ctx, spec.ToModelQuery())
}

func (e *Engine) QuerySpecRowStream(
	ctx context.Context,
	spec querylang.QuerySpec,
) (queryexec.RowStream, error) {
	return e.QueryRowIterator(ctx, spec.ToModelQuery())
}

func (e *Engine) QuerySpecWithExplain(
	ctx context.Context,
	spec querylang.QuerySpec,
) ([]model.ColumnSeries, model.QueryExplain, model.QueryStats, error) {
	return e.QueryWithExplain(ctx, spec.ToModelQuery())
}
