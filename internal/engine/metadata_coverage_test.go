package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestEngineAndLocalMetadataStoreListDatabases(t *testing.T) {
	ctx := context.Background()
	store, err := OpenLocalMetadataStore(t.TempDir())
	if err != nil {
		t.Fatalf("OpenLocalMetadataStore() error = %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	if err := store.CreateDatabase(ctx, "zeta"); err != nil {
		t.Fatalf("CreateDatabase(zeta) error = %v", err)
	}
	if err := store.CreateDatabase(ctx, "alpha"); err != nil {
		t.Fatalf("CreateDatabase(alpha) error = %v", err)
	}
	databases, err := store.ListDatabases(ctx)
	if err != nil {
		t.Fatalf("ListDatabases() error = %v", err)
	}
	if len(databases) != 2 || databases[0] != "alpha" || databases[1] != "zeta" {
		t.Fatalf("ListDatabases() = %v, want [alpha zeta]", databases)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.ListDatabases(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("ListDatabases(canceled) error = %v, want context.Canceled", err)
	}

	wantErr := errors.New("list databases failed")
	facadeStore := &listMetadataStore{
		databases: []string{"metrics"},
		err:       wantErr,
	}
	engine := &Engine{metadata: facadeStore}
	if _, err := engine.ListDatabases(ctx); !errors.Is(err, wantErr) {
		t.Fatalf("Engine.ListDatabases() error = %v, want %v", err, wantErr)
	}
}

func TestSplitSeriesSampleWindowsHandlesUnevenColumns(t *testing.T) {
	columns := []model.ColumnData{
		{FieldID: 1, Samples: versionedSamples(1, 2, 3, 4, 5)},
		{FieldID: 2, Samples: versionedSamples(10, 20, 30)},
		{FieldID: 3},
	}
	if got := splitSeriesSampleWindows(columns, 0); len(got) != 1 || len(got[0]) != 3 {
		t.Fatalf("splitSeriesSampleWindows(max=0) = %#v, want original columns", got)
	}
	if got := splitSeriesSampleWindows(nil, 2); len(got) != 1 || got[0] != nil {
		t.Fatalf("splitSeriesSampleWindows(nil) = %#v, want one nil window", got)
	}
	if got := splitSeriesSampleWindows(columns, 5); len(got) != 1 {
		t.Fatalf("splitSeriesSampleWindows(max=5) windows = %d, want 1", len(got))
	}

	windows := splitSeriesSampleWindows(columns, 2)
	if len(windows) != 3 {
		t.Fatalf("window count = %d, want 3", len(windows))
	}
	if len(windows[0]) != 2 || len(windows[0][0].Samples) != 2 || len(windows[0][1].Samples) != 2 {
		t.Fatalf("first window = %#v, want two columns with two samples", windows[0])
	}
	if len(windows[1]) != 2 || len(windows[1][0].Samples) != 2 || len(windows[1][1].Samples) != 1 {
		t.Fatalf("second window = %#v, want clipped second column", windows[1])
	}
	if len(windows[2]) != 1 || len(windows[2][0].Samples) != 1 || windows[2][0].Samples[0].Timestamp != 5 {
		t.Fatalf("third window = %#v, want final first-column sample", windows[2])
	}
	if len(columns[0].Samples) != 5 || len(columns[1].Samples) != 3 {
		t.Fatal("splitSeriesSampleWindows mutated input sample lengths")
	}
}

type listMetadataStore struct {
	recordingMetadataStore
	databases []string
	err       error
}

func (s *listMetadataStore) ListDatabases(context.Context) ([]string, error) {
	return s.databases, s.err
}

func versionedSamples(timestamps ...int64) []model.VersionedSample {
	samples := make([]model.VersionedSample, len(timestamps))
	for index, timestamp := range timestamps {
		samples[index] = model.VersionedSample{Timestamp: timestamp, WriteSeq: uint64(index + 1)}
	}
	return samples
}
