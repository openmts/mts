package storagecheck

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/openmts/mts/internal/sstable"
	"github.com/openmts/mts/internal/storagefs"
)

type SnapshotOptions struct {
	Overwrite bool
}

type SnapshotResult struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Files  int    `json:"files"`
	Bytes  int64  `json:"bytes"`
}

func Snapshot(source string, target string, opts SnapshotOptions) (SnapshotResult, error) {
	result, err := copyCheckedTree(source, target, opts.Overwrite)
	if err != nil {
		return SnapshotResult{}, err
	}
	return result, nil
}

func Restore(snapshot string, target string, opts SnapshotOptions) (SnapshotResult, error) {
	result, err := copyCheckedTree(snapshot, target, opts.Overwrite)
	if err != nil {
		return SnapshotResult{}, err
	}
	return result, nil
}

func copyCheckedTree(source string, target string, overwrite bool) (SnapshotResult, error) {
	cleanSource := filepath.Clean(source)
	cleanTarget := filepath.Clean(target)
	if err := ensureHealthySource(cleanSource); err != nil {
		return SnapshotResult{}, err
	}
	if err := ensureTargetAvailable(cleanTarget, overwrite); err != nil {
		return SnapshotResult{}, err
	}
	tmp := cleanTarget + ".tmp"
	if err := storagefs.RemoveAll(tmp); err != nil && !os.IsNotExist(err) {
		return SnapshotResult{}, fmt.Errorf("remove temp snapshot target: %w", err)
	}
	if err := storagefs.MkdirAll(tmp); err != nil {
		return SnapshotResult{}, fmt.Errorf("create temp snapshot target: %w", err)
	}
	result := SnapshotResult{Source: cleanSource, Target: cleanTarget}
	if err := copyTree(cleanSource, tmp, &result); err != nil {
		_ = storagefs.RemoveAll(tmp)
		return SnapshotResult{}, err
	}
	if err := rewriteManifestPaths(tmp, cleanSource, cleanTarget); err != nil {
		_ = storagefs.RemoveAll(tmp)
		return SnapshotResult{}, err
	}
	if overwrite {
		if err := storagefs.RemoveAll(cleanTarget); err != nil && !os.IsNotExist(err) {
			_ = storagefs.RemoveAll(tmp)
			return SnapshotResult{}, fmt.Errorf("remove existing target: %w", err)
		}
	}
	if err := storagefs.Rename(tmp, cleanTarget); err != nil {
		_ = storagefs.RemoveAll(tmp)
		return SnapshotResult{}, fmt.Errorf("publish snapshot target: %w", err)
	}
	if err := storagefs.SyncDir(filepath.Dir(cleanTarget)); err != nil {
		return SnapshotResult{}, fmt.Errorf("sync snapshot parent: %w", err)
	}
	return result, nil
}

func rewriteManifestPaths(root string, oldRoot string, newRoot string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() || !hasFile(path, "MANIFEST.bin") {
			return nil
		}
		manifest, err := sstable.LoadManifest(path)
		if err != nil {
			return fmt.Errorf("load copied manifest %s: %w", path, err)
		}
		changed := false
		for index := range manifest.Parts {
			nextPath := relocatedPartPath(path, manifest.Parts[index], oldRoot, newRoot)
			if manifest.Parts[index].Path != nextPath {
				manifest.Parts[index].Path = nextPath
				changed = true
			}
		}
		if !changed {
			return nil
		}
		if err := sstable.WriteManifest(path, manifest); err != nil {
			return fmt.Errorf("write copied manifest %s: %w", path, err)
		}
		return nil
	})
}

func relocatedPartPath(manifestDir string, part sstable.PartMeta, oldRoot string, newRoot string) string {
	if part.Path == "" {
		return filepath.Join(manifestDir, part.ID)
	}
	rel, err := filepath.Rel(oldRoot, part.Path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return filepath.Join(manifestDir, part.ID)
	}
	return filepath.Join(newRoot, rel)
}

func ensureHealthySource(source string) error {
	report, err := Check(source, Options{UnknownFiles: UnknownFileIgnore})
	if err != nil {
		return fmt.Errorf("check source: %w", err)
	}
	for _, issue := range report.Issues {
		if issue.Severity == SeverityFatal {
			return fmt.Errorf("source has fatal issue at %s: %s", issue.Path, issue.Reason)
		}
	}
	return nil
}

func ensureTargetAvailable(target string, overwrite bool) error {
	_, err := storagefs.Stat(target)
	if err == nil && !overwrite {
		return fmt.Errorf("target already exists: %s", target)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat target: %w", err)
	}
	return nil
}

func copyTree(source string, target string, result *SnapshotResult) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		dst := filepath.Join(target, rel)
		if entry.IsDir() {
			return storagefs.MkdirAll(dst)
		}
		return copyFile(path, dst, result)
	})
}

func copyFile(source string, target string, result *SnapshotResult) error {
	data, err := storagefs.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read snapshot file %s: %w", source, err)
	}
	if err := storagefs.WriteFileAtomic(target, data); err != nil {
		return fmt.Errorf("write snapshot file %s: %w", target, err)
	}
	result.Files++
	result.Bytes += int64(len(data))
	return nil
}
