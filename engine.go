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

func (e *Engine) QueryColumnIterator(ctx context.Context, query Query) (ColumnIterator, error) {
	return e.inner.QueryColumnIterator(ctx, query)
}

func (e *Engine) QueryRows(ctx context.Context, query Query) ([]Row, error) {
	return e.inner.QueryRows(ctx, query)
}

func (e *Engine) QueryRowIterator(ctx context.Context, query Query) (RowIterator, error) {
	return e.inner.QueryRowIterator(ctx, query)
}

func (e *Engine) Compact(ctx context.Context) error {
	return e.inner.Compact(ctx)
}

func (e *Engine) ApplyRetention(ctx context.Context, now time.Time) error {
	return e.inner.ApplyRetention(ctx, now)
}

func (e *Engine) MaintenanceErrors(ctx context.Context) []error {
	return e.inner.MaintenanceErrors(ctx)
}

func (e *Engine) CreateDatabase(ctx context.Context, name string) error {
	return e.inner.CreateDatabase(ctx, name)
}

func (e *Engine) DropDatabase(ctx context.Context, name string) error {
	return e.inner.DropDatabase(ctx, name)
}

func (e *Engine) CreateRetentionPolicy(ctx context.Context, database string, policy RetentionPolicy) error {
	return e.inner.CreateRetentionPolicy(ctx, database, policy)
}

func (e *Engine) ListRetentionPolicies(ctx context.Context, database string) ([]RetentionPolicy, error) {
	return e.inner.ListRetentionPolicies(ctx, database)
}

func (e *Engine) ListMeasurements(ctx context.Context, database string) ([]string, error) {
	return e.inner.ListMeasurements(ctx, database)
}

func (e *Engine) ListFields(ctx context.Context, database string, measurement string) ([]FieldSchema, error) {
	return e.inner.ListFields(ctx, database, measurement)
}

func (e *Engine) ListSeries(
	ctx context.Context,
	database string,
	measurement string,
	tags map[string]string,
) ([]Series, error) {
	return e.inner.ListSeries(ctx, database, measurement, tags)
}
