package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	mts "github.com/openmts/mts"
	"github.com/openmts/mts/internal/faultinject"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/sstable"
	"github.com/openmts/mts/internal/storagefs"
	"github.com/openmts/mts/internal/wal"
)

type faultReport struct {
	Cases []faultCaseReport `json:"cases"`
	OK    bool              `json:"ok"`
}

type faultCaseReport struct {
	Name              string `json:"name"`
	Operation         string `json:"operation"`
	Stage             string `json:"stage"`
	Expected          string `json:"expected"`
	Recovered         bool   `json:"recovered"`
	Rows              int    `json:"rows"`
	MaintenanceIssues int    `json:"maintenance_issues"`
}

type faultCase struct {
	name      string
	operation faultinject.Operation
	stage     string
	expected  string
	run       func(context.Context, string, faultinject.Operation) error
}

type recoveryResult struct {
	rows              int
	maintenanceIssues int
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() (err error) {
	root, err := os.MkdirTemp("", "mts-fault-matrix-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	if err := os.Chmod(root, 0700); err != nil {
		_ = os.RemoveAll(root)
		return fmt.Errorf("chmod temp dir: %w", err)
	}
	report, runErr := runMatrix(context.Background(), root)
	cleanupErr := os.RemoveAll(root)
	if runErr != nil {
		return errors.Join(runErr, cleanupErr)
	}
	if cleanupErr != nil {
		return cleanupErr
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}

func runMatrix(ctx context.Context, root string) (faultReport, error) {
	cases := []faultCase{
		{name: "open-create-failure", operation: faultinject.OpCreate, stage: "engine_open", expected: "open rejects create fault", run: runOpenCreateFailure},
		{name: "write-failure", operation: faultinject.OpWrite, stage: "engine_write", expected: "write rejects WAL fault", run: runWriteFailure},
		{name: "sync-failure", operation: faultinject.OpSync, stage: "engine_write_sync", expected: "sync write rejects fsync fault", run: runWriteFailure},
		{name: "wal-write-failure", operation: faultinject.OpWrite, stage: "wal_append", expected: "WAL append rejects write fault", run: runWALAppendFailure},
		{name: "wal-sync-failure", operation: faultinject.OpSync, stage: "wal_append_sync", expected: "WAL append rejects sync fault", run: runWALAppendFailure},
		{name: "wal-checkpoint-remove-failure", operation: faultinject.OpRemove, stage: "wal_checkpoint_remove", expected: "checkpoint failure preserves replay", run: runWALCheckpointRemoveFailure},
		{name: "wal-checkpoint-sync-failure", operation: faultinject.OpSync, stage: "wal_checkpoint_sync", expected: "checkpoint sync failure preserves replay", run: runWALCheckpointSyncFailure},
		{name: "partwriter-write-failure", operation: faultinject.OpWrite, stage: "part_writer", expected: "part writer abort removes partial output", run: runPartWriterFailure},
		{name: "flush-rename-failure", operation: faultinject.OpRename, stage: "flush_manifest", expected: "flush rejects manifest rename fault", run: runFlushRenameFailure},
		{name: "compact-remove-failure", operation: faultinject.OpRemove, stage: "compaction_cleanup", expected: "compaction records cleanup issue", run: runCompactRemoveFailure},
		{name: "retention-remove-failure", operation: faultinject.OpRemove, stage: "retention_cleanup", expected: "retention rejects remove fault", run: runRetentionRemoveFailure},
		{name: "reopen-stat-failure", operation: faultinject.OpStat, stage: "recovery_stat", expected: "reopen rejects stat fault", run: runReopenFailure},
		{name: "reopen-walk-failure", operation: faultinject.OpWalk, stage: "recovery_walk", expected: "reopen rejects walk fault", run: runReopenFailure},
	}
	report := faultReport{Cases: make([]faultCaseReport, 0, len(cases)), OK: true}
	for _, item := range cases {
		caseDir := filepath.Join(root, item.name)
		if err := storagefs.MkdirAll(caseDir); err != nil {
			return faultReport{}, fmt.Errorf("create case dir %s: %w", item.name, err)
		}
		if err := verifyFaultOperation(caseDir, item.operation); err != nil {
			return faultReport{}, fmt.Errorf("%s direct fault: %w", item.name, err)
		}
		if err := item.run(ctx, caseDir, item.operation); err != nil {
			return faultReport{}, fmt.Errorf("%s engine fault: %w", item.name, err)
		}
		recovery, err := verifyEngineRecovery(ctx, caseDir)
		if err != nil {
			return faultReport{}, fmt.Errorf("%s recovery: %w", item.name, err)
		}
		report.Cases = append(report.Cases, faultCaseReport{
			Name:              item.name,
			Operation:         string(item.operation),
			Stage:             item.stage,
			Expected:          item.expected,
			Recovered:         true,
			Rows:              recovery.rows,
			MaintenanceIssues: recovery.maintenanceIssues,
		})
	}
	return report, nil
}

func verifyFaultOperation(root string, op faultinject.Operation) error {
	fs := faultinject.NewFS()
	fs.Fail(op, errors.New("injected"))
	path := filepath.Join(root, "direct-"+string(op))
	file, createErr := fs.Create(path)
	if op == faultinject.OpCreate {
		return expectFault(createErr, op)
	}
	if createErr != nil {
		return fmt.Errorf("create direct file: %w", createErr)
	}
	switch op {
	case faultinject.OpWrite:
		_, createErr = fs.Write(file, []byte("x"))
	case faultinject.OpSync:
		createErr = fs.Sync(file)
	case faultinject.OpRename:
		createErr = fs.Rename(path, path+".next")
	case faultinject.OpRemove:
		createErr = fs.RemoveAll(path)
	case faultinject.OpStat:
		_, createErr = fs.Stat(path)
	case faultinject.OpWalk:
		createErr = fs.Walk(root, func(string, os.FileInfo, error) error { return nil })
	default:
		createErr = fmt.Errorf("unsupported operation %s", op)
	}
	closeErr := file.Close()
	if closeErr != nil {
		return closeErr
	}
	return expectFault(createErr, op)
}

func runOpenCreateFailure(ctx context.Context, dir string, op faultinject.Operation) error {
	return withFault(op, func() error {
		eng, err := mts.Open(ctx, options(dir))
		if err == nil {
			closeErr := eng.Close(ctx)
			return errors.Join(fmt.Errorf("open error = nil, want fault"), closeErr)
		}
		return err
	})
}

func runWriteFailure(ctx context.Context, dir string, op faultinject.Operation) error {
	eng, err := mts.Open(ctx, options(dir))
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	faultErr := withFault(op, func() error {
		return eng.Write(ctx, faultPoints("fault", 0, 4), mts.WriteOptions{Sync: true})
	})
	closeErr := eng.Close(ctx)
	if faultErr != nil {
		return errors.Join(faultErr, closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	return nil
}

func runWALAppendFailure(_ context.Context, dir string, op faultinject.Operation) error {
	log, err := wal.Open(filepath.Join(dir, "wal-append"), wal.Options{Sync: op == faultinject.OpSync})
	if err != nil {
		return fmt.Errorf("open wal append case: %w", err)
	}
	faultErr := withFault(op, func() error {
		return log.Append([]model.ResolvedPoint{walFaultPoint()}, true)
	})
	closeErr := log.Close()
	return errors.Join(faultErr, closeErr)
}

func runWALCheckpointRemoveFailure(_ context.Context, dir string, op faultinject.Operation) error {
	walDir := filepath.Join(dir, "wal-checkpoint-remove")
	log, err := wal.Open(walDir, wal.Options{Sync: true})
	if err != nil {
		return fmt.Errorf("open wal checkpoint remove case: %w", err)
	}
	if err := log.Append([]model.ResolvedPoint{walFaultPoint()}, true); err != nil {
		closeErr := log.Close()
		return errors.Join(fmt.Errorf("append wal checkpoint remove case: %w", err), closeErr)
	}
	faultErr := withFault(op, log.Checkpoint)
	closeErr := log.Close()
	if err := errors.Join(faultErr, closeErr); err != nil {
		return err
	}
	return verifyWALReplay(walDir)
}

func runWALCheckpointSyncFailure(_ context.Context, dir string, op faultinject.Operation) error {
	walDir := filepath.Join(dir, "wal-checkpoint-sync")
	log, err := wal.Open(walDir, wal.Options{})
	if err != nil {
		return fmt.Errorf("open wal checkpoint sync case: %w", err)
	}
	if err := log.Append([]model.ResolvedPoint{walFaultPoint()}, false); err != nil {
		closeErr := log.Close()
		return errors.Join(fmt.Errorf("append wal checkpoint sync case: %w", err), closeErr)
	}
	faultErr := withFault(op, log.Checkpoint)
	closeErr := log.Close()
	if err := errors.Join(faultErr, closeErr); err != nil {
		return err
	}
	return verifyWALReplay(walDir)
}

func runPartWriterFailure(_ context.Context, dir string, op faultinject.Operation) error {
	writer, err := sstable.NewPartWriter(dir, 0, "sst-partwriter-fault", sstable.WriteOptions{})
	if err != nil {
		return fmt.Errorf("create part writer: %w", err)
	}
	faultErr := withFault(op, func() error {
		return writer.AddSeries(partWriterFaultColumns())
	})
	abortErr := writer.Abort()
	if err := errors.Join(faultErr, abortErr); err != nil {
		return err
	}
	partPath := filepath.Join(dir, "sst-partwriter-fault")
	if _, err := os.Stat(partPath); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("part writer output stat=%v want not exist", err)
	}
	return nil
}

func runFlushRenameFailure(ctx context.Context, dir string, op faultinject.Operation) error {
	eng, err := mts.Open(ctx, options(dir))
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	if err := eng.Write(ctx, faultPoints("fault", 10, 2), mts.WriteOptions{Sync: true}); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("write before flush fault: %w", err), closeErr)
	}
	faultErr := withFault(op, func() error {
		return eng.Flush(ctx)
	})
	closeErr := eng.Close(ctx)
	if faultErr != nil {
		return errors.Join(faultErr, closeErr)
	}
	return closeErr
}

func runCompactRemoveFailure(ctx context.Context, dir string, op faultinject.Operation) error {
	eng, err := mts.Open(ctx, options(dir))
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	for index := range 3 {
		points := faultPoints("fault", int64(100+index*10), 2)
		if err := eng.Write(ctx, points, mts.WriteOptions{Sync: true}); err != nil {
			closeErr := eng.Close(ctx)
			return errors.Join(fmt.Errorf("write compact input: %w", err), closeErr)
		}
		if err := eng.Flush(ctx); err != nil {
			closeErr := eng.Close(ctx)
			return errors.Join(fmt.Errorf("flush compact input: %w", err), closeErr)
		}
	}
	fs := faultinject.NewFS()
	fs.FailNext(op, errors.New("injected"))
	restore := storagefs.SetFaultController(fs)
	compactErr := eng.Compact(ctx)
	restore()
	if compactErr != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("compact remove fault: %w", compactErr), closeErr)
	}
	maintenance := eng.MaintenanceErrors(ctx)
	closeErr := eng.Close(ctx)
	if len(maintenance) == 0 {
		return errors.Join(fmt.Errorf("maintenance issues = 0, want compact cleanup fault"), closeErr)
	}
	return closeErr
}

func runRetentionRemoveFailure(ctx context.Context, dir string, op faultinject.Operation) error {
	opts := options(dir)
	opts.Retention = time.Hour
	eng, err := mts.Open(ctx, opts)
	if err != nil {
		return fmt.Errorf("open retention case: %w", err)
	}
	if err := eng.Write(ctx, faultPoints("fault", 0, 2), mts.WriteOptions{Sync: true}); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("write retention input: %w", err), closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("flush retention input: %w", err), closeErr)
	}
	faultErr := withFault(op, func() error {
		return eng.ApplyRetention(ctx, time.Unix(0, int64(2*time.Hour)))
	})
	closeErr := eng.Close(ctx)
	return errors.Join(faultErr, closeErr)
}

func runReopenFailure(ctx context.Context, dir string, op faultinject.Operation) error {
	if err := writeStableDataset(ctx, dir, "fault"); err != nil {
		return err
	}
	return withFault(op, func() error {
		eng, err := mts.Open(ctx, options(dir))
		if err == nil {
			closeErr := eng.Close(ctx)
			return errors.Join(fmt.Errorf("reopen error = nil, want fault"), closeErr)
		}
		return err
	})
}

func verifyEngineRecovery(ctx context.Context, dir string) (recoveryResult, error) {
	if err := writeStableDataset(ctx, dir, "recover"); err != nil {
		return recoveryResult{}, err
	}
	eng, err := mts.Open(ctx, options(dir))
	if err != nil {
		return recoveryResult{}, fmt.Errorf("reopen after fault: %w", err)
	}
	rows, queryErr := eng.QueryRows(ctx, mts.Query{Measurement: "recover", StartTime: 0, EndTime: 50})
	maintenanceIssues := len(eng.MaintenanceErrors(ctx))
	closeErr := eng.Close(ctx)
	if queryErr != nil {
		return recoveryResult{}, errors.Join(fmt.Errorf("query after fault: %w", queryErr), closeErr)
	}
	if closeErr != nil {
		return recoveryResult{}, closeErr
	}
	if len(rows) != 5 {
		return recoveryResult{}, fmt.Errorf("recovered rows=%d want=5", len(rows))
	}
	for _, row := range rows {
		value := row.Fields["v"]
		if value.Type != mts.FieldInt64 || value.Int64 != row.Timestamp {
			return recoveryResult{}, fmt.Errorf("recovered row=%#v has invalid value", row)
		}
	}
	return recoveryResult{rows: len(rows), maintenanceIssues: maintenanceIssues}, nil
}

func verifyWALReplay(dir string) error {
	log, err := wal.Open(dir, wal.Options{})
	if err != nil {
		return fmt.Errorf("reopen wal after checkpoint fault: %w", err)
	}
	points, replayErr := log.Replay()
	closeErr := log.Close()
	if replayErr != nil {
		return errors.Join(fmt.Errorf("replay wal after checkpoint fault: %w", replayErr), closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if len(points) != 1 || points[0].SeriesID != 1 || points[0].Timestamp != 10 {
		return fmt.Errorf("wal replay points=%#v want one checkpoint survivor", points)
	}
	return nil
}

func writeStableDataset(ctx context.Context, dir string, measurement string) error {
	eng, err := mts.Open(ctx, options(dir))
	if err != nil {
		return fmt.Errorf("open stable dataset: %w", err)
	}
	if err := eng.Write(ctx, faultPoints(measurement, 0, 5), mts.WriteOptions{Sync: true}); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("write stable dataset: %w", err), closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("flush stable dataset: %w", err), closeErr)
	}
	if err := eng.Compact(ctx); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("compact stable dataset: %w", err), closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		return fmt.Errorf("close stable dataset: %w", err)
	}
	return nil
}

func withFault(op faultinject.Operation, fn func() error) error {
	fs := faultinject.NewFS()
	fs.FailNext(op, errors.New("injected"))
	restore := storagefs.SetFaultController(fs)
	err := fn()
	restore()
	return expectFault(err, op)
}

func expectFault(err error, op faultinject.Operation) error {
	if err == nil {
		return fmt.Errorf("%s error = nil, want injected fault", op)
	}
	if !strings.Contains(err.Error(), "fault "+string(op)) {
		return fmt.Errorf("%s error = %v, want injected fault", op, err)
	}
	return nil
}

func options(dir string) mts.Options {
	return mts.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 4,
		Compaction: mts.CompactionOptions{
			Enabled:         true,
			Level0PartLimit: 1,
			MaxCascadeSteps: 4,
		},
	}
}

func faultPoints(measurement string, start int64, count int) []mts.Point {
	points := make([]mts.Point, 0, count)
	for index := range count {
		timestamp := start + int64(index)
		points = append(points, mts.Point{
			Measurement: measurement,
			Timestamp:   timestamp,
			Fields:      map[string]mts.FieldValue{"v": mts.Int64Value(timestamp)},
		})
	}
	return points
}

func walFaultPoint() model.ResolvedPoint {
	return model.ResolvedPoint{
		SeriesID:  1,
		Timestamp: 10,
		WriteSeq:  1,
		Fields: []model.ResolvedField{
			{FieldID: 1, FieldName: "v", Type: model.FieldInt64, Value: model.Int64Value(10)},
		},
	}
}

func partWriterFaultColumns() []model.ColumnData {
	return []model.ColumnData{{
		SeriesID:  1,
		FieldID:   1,
		FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{
			{Timestamp: 1, WriteSeq: 1, Value: model.Float64Value(1)},
		},
	}}
}
