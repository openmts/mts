package mts

import (
	"context"
	"time"
)

func (e *Engine) Close(ctx context.Context) error {
	return e.inner.Close(ctx)
}

func (e *Engine) Write(ctx context.Context, points []Point, opts WriteOptions) error {
	return e.inner.Write(ctx, points, opts)
}

func (e *Engine) Flush(ctx context.Context) error {
	return e.inner.Flush(ctx)
}

func (e *Engine) QueryColumns(ctx context.Context, query Query) ([]ColumnSeries, error) {
	return e.inner.QueryColumns(ctx, query)
}

func (e *Engine) QueryRows(ctx context.Context, query Query) ([]Row, error) {
	return e.inner.QueryRows(ctx, query)
}

func (e *Engine) Compact(ctx context.Context) error {
	return e.inner.Compact(ctx)
}

func (e *Engine) ApplyRetention(ctx context.Context, now time.Time) error {
	return e.inner.ApplyRetention(ctx, now)
}
