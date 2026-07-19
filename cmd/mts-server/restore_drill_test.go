package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	mts "github.com/openmts/mts"
	"github.com/openmts/mts/internal/storagecheck"
)

// TestDataDirSidePathRestoreDrill 验证可商用备份演练关键路径：
// 写入并 flush 后快照 data_dir → 旁路 restore → 新引擎读回一致。
func TestDataDirSidePathRestoreDrill(t *testing.T) {
	ctx := context.Background()
	source := t.TempDir()
	eng, err := mts.Open(ctx, mts.DefaultOptions(source))
	if err != nil {
		t.Fatalf("Open(source) error = %v", err)
	}
	t.Cleanup(func() { _ = eng.Close(context.Background()) })

	point := mts.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "restore-e2e"},
		Timestamp:   1_700_000_000_000_000_000,
		Fields: map[string]mts.FieldValue{
			"usage": mts.Float64Value(0.42),
		},
	}
	if err := eng.Write(ctx, []mts.Point{point}, mts.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := eng.Flush(ctx); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	// 源侧读确认
	q, err := mts.NewQuery().From("default", "autogen", "cpu").Select("usage").Build()
	if err != nil {
		t.Fatalf("Build query error = %v", err)
	}
	srcRows, err := eng.QueryRows(ctx, q)
	if err != nil {
		t.Fatalf("QueryRows(source) error = %v", err)
	}
	if len(srcRows) != 1 {
		t.Fatalf("source rows = %d, want 1", len(srcRows))
	}

	snapshotDir := filepath.Join(t.TempDir(), "snapshot")
	if _, err := storagecheck.Snapshot(source, snapshotDir, storagecheck.SnapshotOptions{}); err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	restoreDir := filepath.Join(t.TempDir(), "restore")
	if _, err := storagecheck.Restore(snapshotDir, restoreDir, storagecheck.SnapshotOptions{}); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// 关闭源后再打开旁路，模拟故障切换验证（路径独立即可）
	restored, err := mts.Open(ctx, mts.DefaultOptions(restoreDir))
	if err != nil {
		t.Fatalf("Open(restore) error = %v", err)
	}
	t.Cleanup(func() { _ = restored.Close(context.Background()) })

	dstRows, err := restored.QueryRows(ctx, q)
	if err != nil {
		t.Fatalf("QueryRows(restore) error = %v", err)
	}
	if len(dstRows) != 1 {
		t.Fatalf("restore rows = %d, want 1", len(dstRows))
	}
	if dstRows[0].Tags["host"] != "restore-e2e" {
		t.Fatalf("restore tags = %#v", dstRows[0].Tags)
	}
	got := dstRows[0].Fields["usage"]
	if got.Type != mts.FieldFloat64 || got.Float64 != 0.42 {
		t.Fatalf("restore field = %#v, want float64 0.42", got)
	}
}

func TestRunDoctorChecksWarnsWithoutTLS(t *testing.T) {
	cfg := defaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.Backup.Dir = filepath.Join(t.TempDir(), "backups")
	lines, err := runDoctorChecks(cfg)
	if err != nil {
		t.Fatalf("runDoctorChecks error = %v", err)
	}
	joined := ""
	for _, line := range lines {
		joined += line + "\n"
	}
	for _, part := range []string{"data_dir ready", "backup_dir ready", "http tls disabled"} {
		if !strings.Contains(joined, part) {
			t.Fatalf("doctor lines missing %q: %q", part, joined)
		}
	}
}

func TestEvaluateDoctorStructured(t *testing.T) {
	cfg := defaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.Backup.Dir = filepath.Join(t.TempDir(), "backups")
	cfg.HTTP.TLS.Enabled = false
	resp, err := evaluateDoctor(cfg)
	if err != nil {
		t.Fatalf("evaluateDoctor error = %v", err)
	}
	if !resp.OK {
		t.Fatalf("ok = false, want true with warnings")
	}
	if resp.HTTPTLSEnabled {
		t.Fatal("http_tls_enabled = true, want false")
	}
	if len(resp.Checks) == 0 || len(resp.Lines) != len(resp.Checks) {
		t.Fatalf("checks/lines mismatch: %#v", resp)
	}
	joined := strings.Join(resp.Lines, "\n")
	for _, part := range []string{"data_dir ready", "backup_dir ready", "http tls disabled"} {
		if !strings.Contains(joined, part) {
			t.Fatalf("missing %q in %q", part, joined)
		}
	}
}

func TestAdminDoctorHTTP(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	server := httptest.NewServer(runtime.httpHandler())
	t.Cleanup(server.Close)

	getJSONWithHeaders(t, server.URL+routeAdminDoctor, nil, http.StatusUnauthorized, &errorResponse{})

	headers := map[string]string{"Authorization": "Bearer test-admin-token"}
	var body doctorResponse
	getJSONWithHeaders(t, server.URL+routeAdminDoctor, headers, http.StatusOK, &body)
	if !body.OK {
		t.Fatalf("doctor ok=false: %#v", body)
	}
	if len(body.Checks) == 0 {
		t.Fatal("doctor checks empty")
	}
	// 有 admin_token 时 auth_hardening 不应再出现 warn
	for _, c := range body.Checks {
		if c.Code == "auth_hardening" {
			t.Fatalf("unexpected auth_hardening check: %#v", c)
		}
	}
}
