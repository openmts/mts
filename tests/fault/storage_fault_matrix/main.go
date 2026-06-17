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

	mts "codeberg.org/mts/mts"
	"codeberg.org/mts/mts/internal/faultinject"
	"codeberg.org/mts/mts/internal/storagefs"
)

type faultReport struct {
	Cases []faultCaseReport `json:"cases"`
	OK    bool              `json:"ok"`
}

type faultCaseReport struct {
	Name      string `json:"name"`
	Operation string `json:"operation"`
	Recovered bool   `json:"recovered"`
}

type faultCase struct {
	name      string
	operation faultinject.Operation
	run       func(context.Context, string, faultinject.Operation) error
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
		{name: "open-create-failure", operation: faultinject.OpCreate, run: runOpenCreateFailure},
		{name: "write-failure", operation: faultinject.OpWrite, run: runWriteFailure},
		{name: "sync-failure", operation: faultinject.OpSync, run: runWriteFailure},
		{name: "flush-rename-failure", operation: faultinject.OpRename, run: runFlushRenameFailure},
		{name: "compact-remove-failure", operation: faultinject.OpRemove, run: runCompactRemoveFailure},
		{name: "reopen-stat-failure", operation: faultinject.OpStat, run: runReopenFailure},
		{name: "reopen-walk-failure", operation: faultinject.OpWalk, run: runReopenFailure},
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
		if err := verifyEngineRecovery(ctx, caseDir); err != nil {
			return faultReport{}, fmt.Errorf("%s recovery: %w", item.name, err)
		}
		report.Cases = append(report.Cases, faultCaseReport{
			Name:      item.name,
			Operation: string(item.operation),
			Recovered: true,
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
	faultErr := withFault(op, func() error {
		return eng.Compact(ctx)
	})
	closeErr := eng.Close(ctx)
	if faultErr != nil {
		return errors.Join(faultErr, closeErr)
	}
	return closeErr
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

func verifyEngineRecovery(ctx context.Context, dir string) error {
	if err := writeStableDataset(ctx, dir, "recover"); err != nil {
		return err
	}
	eng, err := mts.Open(ctx, options(dir))
	if err != nil {
		return fmt.Errorf("reopen after fault: %w", err)
	}
	rows, queryErr := eng.QueryRows(ctx, mts.Query{Measurement: "recover", StartTime: 0, EndTime: 50})
	closeErr := eng.Close(ctx)
	if queryErr != nil {
		return errors.Join(fmt.Errorf("query after fault: %w", queryErr), closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if len(rows) != 5 {
		return fmt.Errorf("recovered rows=%d want=5", len(rows))
	}
	for _, row := range rows {
		value := row.Fields["v"]
		if value.Type != mts.FieldInt64 || value.Int64 != row.Timestamp {
			return fmt.Errorf("recovered row=%#v has invalid value", row)
		}
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
