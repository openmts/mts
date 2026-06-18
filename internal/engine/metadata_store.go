package engine

import (
	"context"

	"github.com/openmts/mts/internal/catalog"
	"github.com/openmts/mts/internal/model"
)

type MetadataStore interface {
	MetadataResolver
	MetadataQuerier
	MetadataManager
	Close() error
}

type MetadataResolver interface {
	ResolvePoints(context.Context, []model.Point) ([]model.ResolvedPoint, error)
}

type MetadataQuerier interface {
	MatchSeries(context.Context, string, map[string]string) ([]uint64, error)
	FieldIDs(context.Context, string, []string) (map[uint32]struct{}, error)
	Snapshot(context.Context) (catalog.Snapshot, error)
}

type MetadataManager interface {
	CreateDatabase(context.Context, string) error
	DropDatabase(context.Context, string) error
	CreateRetentionPolicy(context.Context, string, model.RetentionPolicy) error
	ListRetentionPolicies(context.Context, string) ([]model.RetentionPolicy, error)
	ListMeasurements(context.Context, string) ([]string, error)
	ListFields(context.Context, string, string) ([]model.FieldSchema, error)
	ListSeries(context.Context, string, string, map[string]string) ([]model.Series, error)
}

type LocalMetadataStore struct {
	catalog *catalog.Catalog
}

var _ MetadataStore = (*LocalMetadataStore)(nil)

func OpenLocalMetadataStore(dir string) (*LocalMetadataStore, error) {
	cat, err := catalog.Open(dir)
	if err != nil {
		return nil, err
	}
	return &LocalMetadataStore{catalog: cat}, nil
}

func (s *LocalMetadataStore) ResolvePoints(
	ctx context.Context,
	points []model.Point,
) ([]model.ResolvedPoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.catalog.ResolvePoints(points)
}

func (s *LocalMetadataStore) MatchSeries(
	ctx context.Context,
	measurement string,
	tags map[string]string,
) ([]uint64, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.catalog.MatchSeries(measurement, tags), nil
}

func (s *LocalMetadataStore) FieldIDs(
	ctx context.Context,
	measurement string,
	names []string,
) (map[uint32]struct{}, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.catalog.FieldIDs(measurement, names), nil
}

func (s *LocalMetadataStore) Snapshot(ctx context.Context) (catalog.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return catalog.Snapshot{}, err
	}
	return s.catalog.Snapshot(), nil
}

func (s *LocalMetadataStore) CreateDatabase(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.catalog.CreateDatabase(name)
}

func (s *LocalMetadataStore) DropDatabase(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.catalog.DropDatabase(name)
}

func (s *LocalMetadataStore) CreateRetentionPolicy(
	ctx context.Context,
	database string,
	policy model.RetentionPolicy,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.catalog.CreateRetentionPolicy(database, policy)
}

func (s *LocalMetadataStore) ListRetentionPolicies(
	ctx context.Context,
	database string,
) ([]model.RetentionPolicy, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.catalog.ListRetentionPolicies(database)
}

func (s *LocalMetadataStore) ListMeasurements(ctx context.Context, database string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.catalog.ListMeasurements(database)
}

func (s *LocalMetadataStore) ListFields(
	ctx context.Context,
	database string,
	measurement string,
) ([]model.FieldSchema, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.catalog.ListFields(database, measurement)
}

func (s *LocalMetadataStore) ListSeries(
	ctx context.Context,
	database string,
	measurement string,
	tags map[string]string,
) ([]model.Series, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.catalog.ListSeries(database, measurement, tags)
}

func (s *LocalMetadataStore) Close() error {
	if s == nil || s.catalog == nil {
		return nil
	}
	return s.catalog.Close()
}
