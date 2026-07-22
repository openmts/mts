package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/openmts/mts/internal/storagecheck"
)

// backupRoot 返回备份根目录（0700 创建）。
func backupRoot(cfg config) (string, error) {
	dir := strings.TrimSpace(cfg.Backup.Dir)
	if dir == "" {
		dir = filepath.Join(cfg.DataDir, "backups")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Clean(dir), nil
}

func underDir(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// storageDataSnapshot 将 live data_dir 快照到 backups/data-snapshot-*。
func (r *serverRuntime) storageDataSnapshot(ctx context.Context, flush bool) (resp storageDataSnapshotResponse, err error) {
	if err = ctx.Err(); err != nil {
		return storageDataSnapshotResponse{}, err
	}
	if err = r.tryBeginAdminHeavy("data_snapshot"); err != nil {
		return storageDataSnapshotResponse{}, err
	}
	defer func() { r.finishAdminHeavy(err) }()
	cfg := r.currentConfig()
	var root string
	root, err = backupRoot(cfg)
	if err != nil {
		return storageDataSnapshotResponse{}, err
	}
	if flush {
		// 已持有 admin heavy 锁，直接刷引擎，避免 r.flush 重入互斥。
		if err = r.engine.Flush(ctx); err != nil {
			err = fmt.Errorf("flush before data snapshot: %w", err)
			return storageDataSnapshotResponse{}, err
		}
	}
	target := filepath.Join(root, fmt.Sprintf("data-snapshot-%d", time.Now().UTC().UnixNano()))
	var result storagecheck.SnapshotResult
	result, err = storagecheck.Snapshot(cfg.DataDir, target, storagecheck.SnapshotOptions{})
	if err != nil {
		return storageDataSnapshotResponse{}, err
	}
	return storageDataSnapshotResponse{
		OK:     true,
		Path:   result.Target,
		Source: result.Source,
		Files:  result.Files,
		Bytes:  result.Bytes,
	}, nil
}

// storageRestoreDrill 将 data-snapshot 旁路恢复到 backups/restore-drill-*，绝不写入 live data_dir。

func resolveRestoreDrillSource(root, sourcePath string) (string, error) {
	source := strings.TrimSpace(sourcePath)
	if source == "" {
		return latestDataSnapshotPath(root)
	}
	source = filepath.Clean(source)
	if !underDir(root, source) {
		return "", fmt.Errorf("source must be under backup dir")
	}
	if base := filepath.Base(source); !strings.HasPrefix(base, "data-snapshot-") {
		return "", fmt.Errorf("source must be a data-snapshot-* directory")
	}
	info, err := os.Stat(source)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("source is not a directory")
	}
	return source, nil
}

func countRestoreDrillFatals(report storagecheck.Report) int {
	fatal := 0
	for _, issue := range report.Issues {
		if issue.Severity == storagecheck.SeverityFatal {
			fatal++
		}
	}
	return fatal
}

func (r *serverRuntime) storageRestoreDrill(ctx context.Context, sourcePath string) (resp storageRestoreDrillResponse, err error) {
	if err = ctx.Err(); err != nil {
		return storageRestoreDrillResponse{}, err
	}
	if err = r.tryBeginAdminHeavy("restore_drill"); err != nil {
		return storageRestoreDrillResponse{}, err
	}
	defer func() { r.finishAdminHeavy(err) }()
	cfg := r.currentConfig()
	var root string
	root, err = backupRoot(cfg)
	if err != nil {
		return storageRestoreDrillResponse{}, err
	}
	var source string
	source, err = resolveRestoreDrillSource(root, sourcePath)
	if err != nil {
		return storageRestoreDrillResponse{}, err
	}
	target := filepath.Join(root, fmt.Sprintf("restore-drill-%d", time.Now().UTC().UnixNano()))
	live := filepath.Clean(cfg.DataDir)
	if filepath.Clean(target) == live || underDir(live, target) || underDir(target, live) {
		err = fmt.Errorf("restore target collides with live data_dir")
		return storageRestoreDrillResponse{}, err
	}
	var result storagecheck.SnapshotResult
	result, err = storagecheck.Restore(source, target, storagecheck.SnapshotOptions{})
	if err != nil {
		return storageRestoreDrillResponse{}, err
	}
	var report storagecheck.Report
	report, err = storagecheck.Check(target, storagecheck.Options{})
	if err != nil {
		err = fmt.Errorf("check restored tree: %w", err)
		return storageRestoreDrillResponse{}, err
	}
	fatal := countRestoreDrillFatals(report)
	resp = storageRestoreDrillResponse{
		OK:          fatal == 0,
		Source:      result.Source,
		Target:      result.Target,
		Files:       result.Files,
		Bytes:       result.Bytes,
		CheckIssues: len(report.Issues),
		CheckFatals: fatal,
		CheckRoot:   report.Root,
	}
	return resp, nil
}

func latestDataSnapshotPath(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "data-snapshot-") {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no data-snapshot-* found under %s", root)
	}
	sort.Strings(names)
	return filepath.Join(root, names[len(names)-1]), nil
}

func (r *serverRuntime) listDataSnapshots() (storageDataSnapshotsResponse, error) {
	cfg := r.currentConfig()
	root, err := backupRoot(cfg)
	if err != nil {
		return storageDataSnapshotsResponse{}, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return storageDataSnapshotsResponse{Snapshots: []storageDataSnapshotInfo{}}, nil
		}
		return storageDataSnapshotsResponse{}, err
	}
	out := make([]storageDataSnapshotInfo, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "data-snapshot-") && !strings.HasPrefix(name, "restore-drill-") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(root, name)
		size, _ := dirSize(path)
		kind := "data-snapshot"
		if strings.HasPrefix(name, "restore-drill-") {
			kind = "restore-drill"
		}
		out = append(out, storageDataSnapshotInfo{
			Name:      name,
			Kind:      kind,
			Path:      path,
			SizeBytes: size,
			ModTime:   info.ModTime().UTC(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return storageDataSnapshotsResponse{Snapshots: out}, nil
}

func dirSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total, err
}
