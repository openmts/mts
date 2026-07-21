package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	mts "github.com/openmts/mts"
)

func (r *serverRuntime) storageValidate() storageValidateResponse {
	cfg := r.currentConfig()
	return storageValidateResponse{OK: r.health().Healthy, DataDir: cfg.DataDir, Health: r.health()}
}

func (r *serverRuntime) storageSnapshot(ctx context.Context) (storageSnapshotResponse, error) {
	if err := ctx.Err(); err != nil {
		return storageSnapshotResponse{}, err
	}
	if err := r.tryBeginAdminHeavy(); err != nil {
		return storageSnapshotResponse{}, err
	}
	defer r.endAdminHeavy()
	cfg := r.currentConfig()
	dir := cfg.Backup.Dir
	if dir == "" {
		dir = filepath.Join(cfg.DataDir, "backups")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return storageSnapshotResponse{}, err
	}
	path := filepath.Join(dir, fmt.Sprintf("snapshot-%d.json", time.Now().UTC().UnixNano()))
	data, err := json.MarshalIndent(r.storageExport(ctx), "", "  ")
	if err != nil {
		return storageSnapshotResponse{}, err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return storageSnapshotResponse{}, err
	}
	return storageSnapshotResponse{OK: true, Path: path}, nil
}

func (r *serverRuntime) storageExport(ctx context.Context) storageExport {
	users, _ := r.engine.ListUsers(ctx)
	grants := make(map[string][]mts.DatabaseGrant)
	for _, user := range users {
		userGrants, err := r.engine.ListDatabasePermissions(ctx, user.Name)
		if err == nil {
			grants[user.Name] = userGrants
		}
	}
	return storageExport{GeneratedAt: time.Now().UTC(), Config: r.effectiveConfig(), Health: r.health(), Users: users, Grants: grants}
}

type storageSnapshotInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	SizeBytes int64     `json:"size_bytes"`
	ModTime   time.Time `json:"mod_time"`
}

type storageSnapshotsResponse struct {
	Snapshots []storageSnapshotInfo `json:"snapshots"`
}

func (r *serverRuntime) listStorageSnapshots() (storageSnapshotsResponse, error) {
	cfg := r.currentConfig()
	dir := cfg.Backup.Dir
	if dir == "" {
		dir = filepath.Join(cfg.DataDir, "backups")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return storageSnapshotsResponse{Snapshots: []storageSnapshotInfo{}}, nil
		}
		return storageSnapshotsResponse{}, err
	}
	out := make([]storageSnapshotInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "snapshot-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		out = append(out, storageSnapshotInfo{
			Name:      name,
			Path:      filepath.Join(dir, name),
			SizeBytes: info.Size(),
			ModTime:   info.ModTime().UTC(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return storageSnapshotsResponse{Snapshots: out}, nil
}

func (r *serverRuntime) deleteStorageSnapshot(name string) error {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("invalid snapshot name")
	}
	if !strings.HasPrefix(name, "snapshot-") || !strings.HasSuffix(name, ".json") {
		return fmt.Errorf("invalid snapshot name")
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid snapshot name")
	}
	cfg := r.currentConfig()
	dir := cfg.Backup.Dir
	if dir == "" {
		dir = filepath.Join(cfg.DataDir, "backups")
	}
	path := filepath.Join(dir, name)
	// 确保仍在备份目录内
	cleanDir := filepath.Clean(dir)
	cleanPath := filepath.Clean(path)
	if cleanPath != filepath.Join(cleanDir, name) {
		return fmt.Errorf("invalid snapshot path")
	}
	if err := os.Remove(cleanPath); err != nil {
		return err
	}
	return nil
}
