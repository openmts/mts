package engine

import (
	"context"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestOpenWithDepsUsesInjectedMetadataStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var openedDir string
	store := &recordingMetadataStore{}
	engine, err := OpenWithDeps(ctx, model.Options{
		Path:                   t.TempDir(),
		DefaultDatabase:        "default",
		DefaultRetentionPolicy: "autogen",
		ShardDuration:          1,
	}, Deps{
		OpenMetadataStore: func(dir string) (MetadataStore, error) {
			openedDir = dir
			return store, nil
		},
	})
	if err != nil {
		t.Fatalf("OpenWithDeps() error = %v", err)
	}
	if err := engine.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if openedDir == "" {
		t.Fatalf("injected metadata opener was not called")
	}
	if !store.closed {
		t.Fatalf("injected metadata store was not closed")
	}
}

type recordingMetadataStore struct {
	closed bool
}

func (s *recordingMetadataStore) ResolvePoints(
	context.Context,
	[]model.Point,
) ([]model.ResolvedPoint, error) {
	return nil, nil
}

func (s *recordingMetadataStore) ResolveTypedBatch(
	context.Context,
	model.TypedBatch,
) ([]model.ResolvedPoint, error) {
	return nil, nil
}

func (s *recordingMetadataStore) ResolveTypedBatchColumns(
	context.Context,
	model.TypedBatch,
) (model.ResolvedTypedBatch, error) {
	return model.ResolvedTypedBatch{}, nil
}

func (s *recordingMetadataStore) MatchSeries(context.Context, string, map[string]string) ([]uint64, error) {
	return nil, nil
}

func (s *recordingMetadataStore) FieldIDs(context.Context, string, []string) (map[uint32]struct{}, error) {
	return nil, nil
}

func (s *recordingMetadataStore) Snapshot(context.Context) (metadataSnapshot, error) {
	return metadataSnapshot{}, nil
}

func (s *recordingMetadataStore) CreateDatabase(context.Context, string) error {
	return nil
}

func (s *recordingMetadataStore) DropDatabase(context.Context, string) error {
	return nil
}

func (s *recordingMetadataStore) ListDatabases(context.Context) ([]string, error) {
	return nil, nil
}

func (s *recordingMetadataStore) CreateRetentionPolicy(
	context.Context,
	string,
	model.RetentionPolicy,
) error {
	return nil
}

func (s *recordingMetadataStore) ListRetentionPolicies(
	context.Context,
	string,
) ([]model.RetentionPolicy, error) {
	return nil, nil
}

func (s *recordingMetadataStore) ListMeasurements(context.Context, string) ([]string, error) {
	return nil, nil
}

func (s *recordingMetadataStore) ListFields(context.Context, string, string) ([]model.FieldSchema, error) {
	return nil, nil
}

func (s *recordingMetadataStore) ListSeries(
	context.Context,
	string,
	string,
	map[string]string,
) ([]model.Series, error) {
	return nil, nil
}

func (s *recordingMetadataStore) UpsertDownsamplePolicy(context.Context, model.DownsamplePolicy) error {
	return nil
}

func (s *recordingMetadataStore) DropDownsamplePolicy(context.Context, string) error {
	return nil
}

func (s *recordingMetadataStore) ListDownsamplePolicies(context.Context) ([]model.DownsamplePolicy, error) {
	return nil, nil
}

func (s *recordingMetadataStore) DownsampleWatermark(
	context.Context,
	string,
) (model.DownsampleWatermark, bool, error) {
	return model.DownsampleWatermark{}, false, nil
}

func (s *recordingMetadataStore) UpdateDownsampleWatermark(
	context.Context,
	model.DownsampleWatermark,
) error {
	return nil
}

func (s *recordingMetadataStore) Close() error {
	s.closed = true
	return nil
}
