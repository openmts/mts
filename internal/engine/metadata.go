package engine

import (
	"context"
	"errors"
	"path/filepath"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/storagefs"
)

func (e *Engine) CreateDatabase(_ context.Context, name string) error {
	return e.catalog.CreateDatabase(name)
}

func (e *Engine) DropDatabase(_ context.Context, name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	var err error
	for id, shard := range e.shards {
		if shard.opts.Database != name {
			continue
		}
		err = errors.Join(err, shard.Close())
		delete(e.shards, id)
	}
	err = errors.Join(err, e.catalog.DropDatabase(name))
	err = errors.Join(err, storagefs.RemoveAll(filepath.Join(e.opts.Path, "data", name)))
	return err
}

func (e *Engine) CreateRetentionPolicy(
	_ context.Context,
	database string,
	policy model.RetentionPolicy,
) error {
	return e.catalog.CreateRetentionPolicy(database, policy)
}

func (e *Engine) ListRetentionPolicies(
	_ context.Context,
	database string,
) ([]model.RetentionPolicy, error) {
	return e.catalog.ListRetentionPolicies(database)
}

func (e *Engine) ListMeasurements(_ context.Context, database string) ([]string, error) {
	return e.catalog.ListMeasurements(database)
}

func (e *Engine) ListFields(
	_ context.Context,
	database string,
	measurement string,
) ([]model.FieldSchema, error) {
	return e.catalog.ListFields(database, measurement)
}

func (e *Engine) ListSeries(
	_ context.Context,
	database string,
	measurement string,
	tags map[string]string,
) ([]model.Series, error) {
	return e.catalog.ListSeries(database, measurement, tags)
}
