package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestCatalogListDatabasesReturnsSortedIndependentSlice(t *testing.T) {
	cat := newCatalog(t.TempDir(), Limits{})
	if err := cat.CreateDatabase("zeta"); err != nil {
		t.Fatalf("CreateDatabase(zeta) error = %v", err)
	}
	if err := cat.CreateDatabase("alpha"); err != nil {
		t.Fatalf("CreateDatabase(alpha) error = %v", err)
	}

	databases, err := cat.ListDatabases()
	if err != nil {
		t.Fatalf("ListDatabases() error = %v", err)
	}
	if len(databases) != 2 || databases[0] != "alpha" || databases[1] != "zeta" {
		t.Fatalf("ListDatabases() = %v, want [alpha zeta]", databases)
	}
	databases[0] = "mutated"
	again, err := cat.ListDatabases()
	if err != nil {
		t.Fatalf("ListDatabases(again) error = %v", err)
	}
	if again[0] != "alpha" {
		t.Fatalf("ListDatabases(again) = %v, want independent result", again)
	}
	if err := cat.CreateRetentionPolicy("alpha", model.RetentionPolicy{Name: "zeta"}); err != nil {
		t.Fatalf("CreateRetentionPolicy(zeta) error = %v", err)
	}
	if err := cat.CreateRetentionPolicy("alpha", model.RetentionPolicy{Name: "alpha"}); err != nil {
		t.Fatalf("CreateRetentionPolicy(alpha) error = %v", err)
	}
	policies, err := cat.ListRetentionPolicies("alpha")
	if err != nil || len(policies) != 2 || policies[0].Name != "alpha" || policies[1].Name != "zeta" {
		t.Fatalf("ListRetentionPolicies() = %v, %v; want sorted policies", policies, err)
	}
	cat.applySeries(Series{ID: 2, Measurement: "cpu"})
	cat.applySeries(Series{ID: 1, Measurement: "cpu"})
	cat.applySeries(Series{ID: 3, Measurement: "memory"})
	series, err := cat.ListSeries("alpha", "cpu", nil)
	if err != nil || len(series) != 2 || series[0].ID != 1 || series[1].ID != 2 {
		t.Fatalf("ListSeries() = %v, %v; want cpu series sorted by ID", series, err)
	}

	measurements, err := cat.ListMeasurements("missing")
	if err != nil || len(measurements) != 0 {
		t.Fatalf("ListMeasurements(missing) = %v, %v; want empty", measurements, err)
	}
}

func TestCatalogCheckpointWithoutWALAndStringListRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cat := newCatalog(dir, Limits{})
	cat.snapshotDirtyRecords = 1
	if err := cat.checkpointSnapshotLocked(false); err != nil {
		t.Fatalf("checkpointSnapshotLocked(false) error = %v", err)
	}
	if cat.snapshotDirtyRecords != 1 {
		t.Fatalf("dirty records = %d, want 1 before forced checkpoint", cat.snapshotDirtyRecords)
	}
	if err := cat.checkpointSnapshotLocked(true); err != nil {
		t.Fatalf("checkpointSnapshotLocked(true) error = %v", err)
	}
	if cat.snapshotDirtyRecords != 0 {
		t.Fatalf("dirty records = %d, want 0 after checkpoint", cat.snapshotDirtyRecords)
	}
	info, err := os.Stat(filepath.Join(dir, "snapshot.bin"))
	if err != nil {
		t.Fatalf("Stat(snapshot.bin) error = %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("snapshot mode = %o, want 0600", info.Mode().Perm())
	}

	reader := newPayloadReader(appendStringList(nil, []string{"host", "region"}))
	values, err := readStringList(reader, "tag")
	if err != nil {
		t.Fatalf("readStringList() error = %v", err)
	}
	if len(values) != 2 || values[0] != "host" || values[1] != "region" {
		t.Fatalf("readStringList() = %v, want [host region]", values)
	}
	if err := reader.done("tag list"); err != nil {
		t.Fatalf("payloadReader.done() error = %v", err)
	}

	cat.snapshotDirtyRecords = 0
	if err := cat.checkpointSnapshotLocked(true); err != nil {
		t.Fatalf("checkpointSnapshotLocked(clean) error = %v", err)
	}
}

func TestCatalogSnapshotReadAndWriteErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "snapshot.bin"), 0700); err != nil {
		t.Fatalf("Mkdir(snapshot.bin) error = %v", err)
	}
	cat := newCatalog(dir, Limits{})
	if err := cat.loadSnapshot(); err == nil {
		t.Fatal("loadSnapshot(directory) error = nil, want error")
	}
	cat.snapshotDirtyRecords = 1
	if err := cat.checkpointSnapshotLocked(true); err == nil {
		t.Fatal("checkpointSnapshotLocked(unwritable snapshot) error = nil, want error")
	}
}
