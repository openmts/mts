package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
