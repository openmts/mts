package storagecheck

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"codeberg.org/mts/mts/internal/storagefs"
)

type RepairOptions struct {
	Apply bool
}

type RepairPlan struct {
	Root    string         `json:"root"`
	Actions []RepairAction `json:"actions"`
}

type RepairAction struct {
	Type    string `json:"type"`
	Path    string `json:"path"`
	PartID  string `json:"part_id,omitempty"`
	Reason  string `json:"reason"`
	Applied bool   `json:"applied"`
}

func Repair(root string, opts RepairOptions) (RepairPlan, error) {
	report, err := Check(root, Options{})
	if err != nil {
		return RepairPlan{}, err
	}
	plan := RepairPlan{Root: filepath.Clean(root)}
	for _, issue := range report.Issues {
		if issue.Reason != "orphan part" {
			continue
		}
		action := RepairAction{
			Type:   "remove_orphan_part",
			Path:   issue.Path,
			PartID: issue.PartID,
			Reason: issue.Reason,
		}
		if opts.Apply {
			if err := storagefs.RemoveAll(issue.Path); err != nil {
				return RepairPlan{}, fmt.Errorf("remove orphan part %s: %w", issue.Path, err)
			}
			action.Applied = true
		}
		plan.Actions = append(plan.Actions, action)
	}
	return plan, nil
}

type MigrateOptions struct {
	Apply bool
}

type MigrationResult struct {
	Root           string `json:"root"`
	BackupPath     string `json:"backup_path"`
	CheckpointPath string `json:"checkpoint_path"`
	Applied        bool   `json:"applied"`
	Resumed        bool   `json:"resumed"`
}

func Migrate(root string, opts MigrateOptions) (MigrationResult, error) {
	clean := filepath.Clean(root)
	result := MigrationResult{
		Root:           clean,
		BackupPath:     filepath.Join(clean, "MANIFEST.bin.bak"),
		CheckpointPath: filepath.Join(clean, "MIGRATION.checkpoint"),
	}
	if _, err := storagefs.Stat(result.CheckpointPath); err == nil {
		result.Resumed = true
		result.Applied = opts.Apply
		return result, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return MigrationResult{}, fmt.Errorf("stat migration checkpoint: %w", err)
	}
	if !opts.Apply {
		return result, nil
	}
	manifestPath := filepath.Join(clean, "MANIFEST.bin")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return MigrationResult{}, fmt.Errorf("read manifest for migration: %w", err)
	}
	if err := storagefs.WriteFileAtomic(result.BackupPath, data); err != nil {
		return MigrationResult{}, fmt.Errorf("write manifest backup: %w", err)
	}
	if err := storagefs.WriteFileAtomic(result.CheckpointPath, migrationCheckpointPayload()); err != nil {
		return MigrationResult{}, fmt.Errorf("write migration checkpoint: %w", err)
	}
	result.Applied = true
	return result, nil
}

func migrationCheckpointPayload() []byte {
	out := []byte("MTSMIG1")
	return binary.LittleEndian.AppendUint64(out, 1)
}
