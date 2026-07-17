package engine

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/storagefs"
)

func (e *Engine) CreateDatabase(ctx context.Context, name string) error {
	return e.metadata.CreateDatabase(ctx, name)
}

func (e *Engine) ListDatabases(ctx context.Context) ([]string, error) {
	return e.metadata.ListDatabases(ctx)
}

func (e *Engine) DropDatabase(ctx context.Context, name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	var err error
	for id, shard := range e.shards {
		if shard.opts.Database != name {
			continue
		}
		closeErr := shard.Close()
		err = errors.Join(err, closeErr)
		if closeErr == nil {
			delete(e.shards, id)
		}
	}
	err = errors.Join(err, e.metadata.DropDatabase(ctx, name))
	err = errors.Join(err, storagefs.RemoveAll(filepath.Join(e.opts.Path, "data", name)))
	return err
}

func (e *Engine) CreateRetentionPolicy(
	ctx context.Context,
	database string,
	policy model.RetentionPolicy,
) error {
	return e.metadata.CreateRetentionPolicy(ctx, database, policy)
}

func (e *Engine) ListRetentionPolicies(
	ctx context.Context,
	database string,
) ([]model.RetentionPolicy, error) {
	return e.metadata.ListRetentionPolicies(ctx, database)
}

func (e *Engine) ListMeasurements(ctx context.Context, database string) ([]string, error) {
	return e.metadata.ListMeasurements(ctx, database)
}

func (e *Engine) ListFields(
	ctx context.Context,
	database string,
	measurement string,
) ([]model.FieldSchema, error) {
	return e.metadata.ListFields(ctx, database, measurement)
}

func (e *Engine) ListSeries(
	ctx context.Context,
	database string,
	measurement string,
	tags map[string]string,
) ([]model.Series, error) {
	return e.metadata.ListSeries(ctx, database, measurement, tags)
}
