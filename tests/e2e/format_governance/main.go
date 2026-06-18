package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	mts "github.com/openmts/mts"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/sstable"
	"github.com/openmts/mts/internal/storagecheck"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("format_governance failed: %v", err)
	}
	log.Print("format_governance passed")
}

func run() (err error) {
	dir, err := os.MkdirTemp("", "mts-e2e-format-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("chmod temp dir: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(dir))
	}()
	return runWithDir(dir)
}

func runWithDir(dir string) error {
	ctx := context.Background()
	if err := writeEngineData(ctx, dir); err != nil {
		return err
	}
	shardDir, err := findShardDir(dir)
	if err != nil {
		return err
	}
	orphan, err := sstable.WritePart(shardDir, 0, "sst-orphan-e2e", []model.ColumnData{e2eColumn(99)})
	if err != nil {
		return fmt.Errorf("write orphan part: %w", err)
	}
	if err := assertCheckAndRepair(shardDir, orphan.Path); err != nil {
		return err
	}
	if err := assertNoJSONStorage(dir); err != nil {
		return err
	}
	if err := assertUnsafePermissionsRejected(ctx, dir); err != nil {
		return err
	}
	return nil
}

func writeEngineData(ctx context.Context, dir string) error {
	eng, err := mts.Open(ctx, mts.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1,
	})
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	point := mts.Point{
		Measurement: "format",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   1,
		Fields: map[string]mts.FieldValue{
			"value": mts.Float64Value(1),
			"state": mts.StringValue("ok"),
		},
	}
	if err := eng.Write(ctx, []mts.Point{point}, mts.WriteOptions{Sync: true}); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("write point: %w", err), closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("flush engine: %w", err), closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		return fmt.Errorf("close engine: %w", err)
	}
	return nil
}

func assertCheckAndRepair(shardDir string, orphanPath string) error {
	report, err := storagecheck.Check(shardDir, storagecheck.Options{})
	if err != nil {
		return fmt.Errorf("check storage: %w", err)
	}
	if !hasIssue(report, storagecheck.SeverityWarn, orphanPath, "orphan part") {
		return fmt.Errorf("check report missing orphan issue: %#v", report.Issues)
	}
	dryRun, err := storagecheck.Repair(shardDir, storagecheck.RepairOptions{})
	if err != nil {
		return fmt.Errorf("repair dry-run: %w", err)
	}
	if len(dryRun.Actions) != 1 || dryRun.Actions[0].Applied {
		return fmt.Errorf("dry-run actions = %#v, want one unapplied action", dryRun.Actions)
	}
	if _, err := os.Stat(orphanPath); err != nil {
		return fmt.Errorf("orphan removed during dry-run: %w", err)
	}
	applied, err := storagecheck.Repair(shardDir, storagecheck.RepairOptions{Apply: true})
	if err != nil {
		return fmt.Errorf("repair apply: %w", err)
	}
	if len(applied.Actions) != 1 || !applied.Actions[0].Applied {
		return fmt.Errorf("apply actions = %#v, want one applied action", applied.Actions)
	}
	if _, err := os.Stat(orphanPath); !os.IsNotExist(err) {
		return fmt.Errorf("orphan stat after apply = %v, want not exist", err)
	}
	return nil
}

func assertUnsafePermissionsRejected(ctx context.Context, dir string) error {
	if err := os.Chmod(dir, 0755); err != nil {
		return fmt.Errorf("chmod unsafe root: %w", err)
	}
	eng, err := mts.Open(ctx, mts.Options{Path: dir})
	if err == nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("open unsafe root succeeded"), closeErr)
	}
	return os.Chmod(dir, 0700)
}

func findShardDir(root string) (string, error) {
	var shardDir string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || !entry.IsDir() || shardDir != "" {
			return walkErr
		}
		if _, err := os.Stat(filepath.Join(path, "MANIFEST.bin")); err == nil {
			shardDir = path
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk shard dirs: %w", err)
	}
	if shardDir == "" {
		return "", fmt.Errorf("shard dir not found")
	}
	return shardDir, nil
}

func assertNoJSONStorage(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if bytes.Contains(data, []byte(`{"`)) || bytes.Contains(data, []byte(`":`)) {
			return fmt.Errorf("file %s appears to contain JSON payload", path)
		}
		return nil
	})
}

func hasIssue(report storagecheck.Report, severity storagecheck.Severity, path string, reason string) bool {
	for _, issue := range report.Issues {
		if issue.Severity == severity && issue.Path == path && issue.Reason == reason {
			return true
		}
	}
	return false
}

func e2eColumn(seriesID uint64) model.ColumnData {
	return model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   1,
		FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{{
			Timestamp: 1,
			WriteSeq:  1,
			Value:     model.Float64Value(1),
		}},
	}
}
