package mts

import (
	"context"
	"time"

	"codeberg.org/mts/mts/internal/model"
)

func (e *Engine) Close(ctx context.Context) error {
	return e.inner.Close(ctx)
}

func (e *Engine) Write(ctx context.Context, points []Point, opts WriteOptions) error {
	return e.inner.Write(ctx, toModelPoints(points), toModelWriteOptions(opts))
}

func (e *Engine) Flush(ctx context.Context) error {
	return e.inner.Flush(ctx)
}

func (e *Engine) QueryColumns(ctx context.Context, query Query) ([]ColumnSeries, error) {
	columns, err := e.inner.QueryColumns(ctx, toModelQuery(query))
	if err != nil {
		return nil, err
	}
	return fromModelColumnSeriesList(columns), nil
}

func (e *Engine) QueryColumnIterator(ctx context.Context, query Query) (ColumnIterator, error) {
	inner, err := e.inner.QueryColumnIterator(ctx, toModelQuery(query))
	if err != nil {
		return nil, err
	}
	return columnIterator{inner: inner}, nil
}

func (e *Engine) QueryRows(ctx context.Context, query Query) ([]Row, error) {
	rows, err := e.inner.QueryRows(ctx, toModelQuery(query))
	if err != nil {
		return nil, err
	}
	return fromModelRows(rows), nil
}

func (e *Engine) QueryRowIterator(ctx context.Context, query Query) (RowIterator, error) {
	inner, err := e.inner.QueryRowIterator(ctx, toModelQuery(query))
	if err != nil {
		return nil, err
	}
	return rowIterator{inner: inner}, nil
}

func (e *Engine) Compact(ctx context.Context) error {
	return e.inner.Compact(ctx)
}

func (e *Engine) CompactWithResult(ctx context.Context) (CompactionResult, error) {
	result, err := e.inner.CompactWithResult(ctx)
	return fromCompactionResult(result), err
}

func (e *Engine) ApplyRetention(ctx context.Context, now time.Time) error {
	return e.inner.ApplyRetention(ctx, now)
}

func (e *Engine) MaintenanceErrors(ctx context.Context) []error {
	return e.inner.MaintenanceErrors(ctx)
}

func (e *Engine) StorageMemorySnapshot() StorageMemorySnapshot {
	return fromStorageMemorySnapshot(e.inner.StorageMemorySnapshot())
}

func (e *Engine) CompactionStatsSnapshot() CompactionStats {
	return fromCompactionStats(e.inner.CompactionStatsSnapshot())
}

func (e *Engine) HealthSnapshot() HealthSnapshot {
	health := e.inner.HealthSnapshot()
	return HealthSnapshot{
		Healthy: health.Healthy,
		Ready:   health.Ready,
		Reasons: append([]string(nil), health.Reasons...),
	}
}

func (e *Engine) CreateDatabase(ctx context.Context, name string) error {
	return e.inner.CreateDatabase(ctx, name)
}

func (e *Engine) DropDatabase(ctx context.Context, name string) error {
	return e.inner.DropDatabase(ctx, name)
}

func (e *Engine) CreateRetentionPolicy(ctx context.Context, database string, policy RetentionPolicy) error {
	return e.inner.CreateRetentionPolicy(ctx, database, toModelRetentionPolicy(policy))
}

func (e *Engine) ListRetentionPolicies(ctx context.Context, database string) ([]RetentionPolicy, error) {
	policies, err := e.inner.ListRetentionPolicies(ctx, database)
	if err != nil {
		return nil, err
	}
	return fromModelRetentionPolicies(policies), nil
}

func (e *Engine) ListMeasurements(ctx context.Context, database string) ([]string, error) {
	return e.inner.ListMeasurements(ctx, database)
}

func (e *Engine) ListFields(ctx context.Context, database string, measurement string) ([]FieldSchema, error) {
	fields, err := e.inner.ListFields(ctx, database, measurement)
	if err != nil {
		return nil, err
	}
	return fromModelFieldSchemas(fields), nil
}

func (e *Engine) ListSeries(
	ctx context.Context,
	database string,
	measurement string,
	tags map[string]string,
) ([]Series, error) {
	series, err := e.inner.ListSeries(ctx, database, measurement, tags)
	if err != nil {
		return nil, err
	}
	return fromModelSeriesList(series), nil
}

type columnIterator struct {
	inner model.ColumnIterator
}

type rowIterator struct {
	inner model.RowIterator
}

func (i columnIterator) Next() bool {
	return i.inner.Next()
}

func (i columnIterator) Column() ColumnSeries {
	return fromModelColumnSeries(i.inner.Column())
}

func (i columnIterator) Err() error {
	return i.inner.Err()
}

func (i columnIterator) Close() error {
	return i.inner.Close()
}

func (i rowIterator) Next() bool {
	return i.inner.Next()
}

func (i rowIterator) Row() Row {
	return fromModelRow(i.inner.Row())
}

func (i rowIterator) Err() error {
	return i.inner.Err()
}

func (i rowIterator) Close() error {
	return i.inner.Close()
}
