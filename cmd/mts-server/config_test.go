package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigAppliesDefaults(t *testing.T) {
	path := writeTestConfig(t, `data_dir: `+filepath.ToSlash(t.TempDir())+`
engine:
  shard_duration: 2h
  retention: 24h
shutdown_timeout: 3s
`)
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if !cfg.HTTP.Enabled || cfg.HTTP.Addr == "" {
		t.Fatalf("HTTP default = %#v, want enabled with addr", cfg.HTTP)
	}
	if !cfg.GRPC.Enabled || cfg.GRPC.Addr == "" {
		t.Fatalf("GRPC default = %#v, want enabled with addr", cfg.GRPC)
	}
}

func TestLoadConfigRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty data dir", body: "data_dir: ' '\n"},
		{name: "no listeners", body: "data_dir: " + filepath.ToSlash(t.TempDir()) + "\nhttp:\n  enabled: false\ngrpc:\n  enabled: false\n"},
		{name: "empty http addr", body: "data_dir: " + filepath.ToSlash(t.TempDir()) + "\nhttp:\n  enabled: true\n  addr: ''\n"},
		{name: "empty grpc addr", body: "data_dir: " + filepath.ToSlash(t.TempDir()) + "\ngrpc:\n  enabled: true\n  addr: ''\n"},
		{name: "bad shutdown", body: "data_dir: " + filepath.ToSlash(t.TempDir()) + "\nshutdown_timeout: 0s\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTestConfig(t, tt.body)
			_, err := loadConfig(path)
			if !errors.Is(err, errInvalidConfig) {
				t.Fatalf("loadConfig(invalid) error = %v, want errInvalidConfig", err)
			}
		})
	}
}

func TestLoadConfigRejectsDecodeErrors(t *testing.T) {
	for _, body := range []string{"unknown: true\n", "data_dir: x\nshutdown_timeout: 1\n"} {
		path := writeTestConfig(t, body)
		if _, err := loadConfig(path); err == nil {
			t.Fatalf("loadConfig(%s) error = nil, want error", body)
		}
	}
	if _, err := loadConfig(" "); !errors.Is(err, errInvalidConfig) {
		t.Fatalf("loadConfig(empty path) error = %v, want errInvalidConfig", err)
	}
}

func TestCLIPrintConfig(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(&stdout, &stderr)
	if err := app.RunContext(context.Background(), []string{"mts-server", "serve", "--print-config"}); err != nil {
		t.Fatalf("Run(print-config) error = %v", err)
	}
	if !strings.Contains(stdout.String(), "data_dir:") {
		t.Fatalf("print-config output = %s, want data_dir", stdout.String())
	}
}

func TestResolveServeConfigGeneratesDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var stdout bytes.Buffer
	cfg, err := resolveServeConfig("", &stdout)
	if err != nil {
		t.Fatalf("resolveServeConfig() error = %v", err)
	}
	wantPath := filepath.Join(home, ".mts", "mts-server.yaml")
	if cfg.ConfigPath != wantPath {
		t.Fatalf("ConfigPath = %q, want %q", cfg.ConfigPath, wantPath)
	}
	if !strings.Contains(stdout.String(), wantPath) {
		t.Fatalf("stdout = %q, want generated config hint", stdout.String())
	}
	info, err := os.Stat(wantPath)
	if err != nil {
		t.Fatalf("Stat(default config) error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("default config perm = %v, want 0600", perm)
	}
	if !cfg.HTTP.Enabled || !cfg.GRPC.Enabled {
		t.Fatalf("generated config listeners = http:%v grpc:%v, want enabled", cfg.HTTP.Enabled, cfg.GRPC.Enabled)
	}
	wantData := filepath.Join(home, ".mts", "data")
	if cfg.DataDir != wantData {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, wantData)
	}
}

func TestResolveServeConfigReusesExistingDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	defaultFile := filepath.Join(home, ".mts", "mts-server.yaml")
	if err := os.MkdirAll(filepath.Dir(defaultFile), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(defaultFile, []byte("data_dir: /tmp/mts-reuse\n"), 0600); err != nil {
		t.Fatalf("WriteFile(default) error = %v", err)
	}
	var stdout bytes.Buffer
	cfg, err := resolveServeConfig("", &stdout)
	if err != nil {
		t.Fatalf("resolveServeConfig() error = %v", err)
	}
	if cfg.DataDir != "/tmp/mts-reuse" {
		t.Fatalf("DataDir = %q, want reused value /tmp/mts-reuse", cfg.DataDir)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestResolveServeConfigFprintfError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := resolveServeConfig("", failWriter{}); err == nil {
		t.Fatal("resolveServeConfig() error = nil, want write error")
	}
}

func TestResolveServeConfigPropagatesDefaultConfigErrors(t *testing.T) {
	homeFile := filepath.Join(t.TempDir(), "home")
	if err := os.WriteFile(homeFile, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile(home) error = %v", err)
	}
	t.Setenv("HOME", homeFile)
	var stdout bytes.Buffer
	if _, err := resolveServeConfig("", &stdout); err == nil {
		t.Fatal("resolveServeConfig() error = nil, want stat error")
	}
}

func TestWriteDefaultConfigErrors(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile(blocker) error = %v", err)
	}
	if err := writeDefaultConfig(filepath.Join(blocker, "sub", "mts-server.yaml")); err == nil {
		t.Fatal("writeDefaultConfig(MkdirAll over file) error = nil, want error")
	}
	existingDir := filepath.Join(dir, "existing")
	if err := os.MkdirAll(existingDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := writeDefaultConfig(existingDir); err == nil {
		t.Fatal("writeDefaultConfig(to directory) error = nil, want error")
	}
}

func TestLoadConfigExpandsHomePaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := writeTestConfig(t, "data_dir: ~/.mts/data\nbackup:\n  dir: ~/.mts/data/backups\n")
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if want := filepath.Join(home, ".mts", "data"); cfg.DataDir != want {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, want)
	}
	if want := filepath.Join(home, ".mts", "data", "backups"); cfg.Backup.Dir != want {
		t.Fatalf("Backup.Dir = %q, want %q", cfg.Backup.Dir, want)
	}
}

func TestExpandHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tests := []struct {
		in   string
		want string
	}{
		{in: "~", want: home},
		{in: "~/data", want: filepath.Join(home, "data")},
		{in: "/var/lib/mts", want: "/var/lib/mts"},
		{in: "relative/path", want: "relative/path"},
		{in: "~not-home", want: "~not-home"},
	}
	for _, tt := range tests {
		got, err := expandHome(tt.in)
		if err != nil {
			t.Fatalf("expandHome(%q) error = %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("expandHome(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestResolveServeConfigExplicitPath(t *testing.T) {
	path := writeTestConfig(t, "data_dir: /tmp/mts-explicit\n")
	var stdout bytes.Buffer
	cfg, err := resolveServeConfig(path, &stdout)
	if err != nil {
		t.Fatalf("resolveServeConfig() error = %v", err)
	}
	if cfg.DataDir != "/tmp/mts-explicit" {
		t.Fatalf("DataDir = %q, want explicit value /tmp/mts-explicit", cfg.DataDir)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty for explicit path", stdout.String())
	}
}

func TestRunMain(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runMain([]string{"mts-server", "serve", "--print-config"}, &stdout, &stderr); code != 0 {
		t.Fatalf("runMain(print-config) code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "data_dir:") {
		t.Fatalf("runMain stdout = %s, want data_dir", stdout.String())
	}
}

func writeTestConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mts-server.yaml")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	return path
}

func TestEngineOptionsMapsAdvancedFields(t *testing.T) {
	path := writeTestConfig(t, `data_dir: `+filepath.ToSlash(t.TempDir())+`
engine:
  max_concurrent_compaction: 3
  max_concurrent_downsample: 2
  memtable_disorder_flush_ratio: 0.3
  memtable_disorder_flush_min_samples: 2048
  compression:
    enabled: true
    algorithm: zstd
    min_page_values: 64
    value_page_samples: 4096
    omit_write_seq: true
    zstd_level: better
  compaction:
    enabled: true
    max_concurrent: 2
  wal:
    sync: true
    segment_bytes: 1048576
    batch_records: 100
    batch_bytes: 65536
    batch_interval: 5ms
  query_page_cache:
    limit: 128
    max_samples: 256
  query_block_cache:
    limit: 64
    max_bytes: 1048576
  query_protection:
    default_max_samples: 1000
    default_limit: 500
  cardinality:
    max_series: 10000
    max_fields: 32
    max_tag_values_per_key: 1000
  storage_memory:
    soft_sample_limit: 1000
    hard_sample_limit: 2000
    soft_bytes_limit: 1048576
    hard_bytes_limit: 2097152
    query_bytes_limit: 1024
    flush_bytes_limit: 2048
    compaction_bytes_limit: 4096
    compression_bytes_limit: 8192
`)
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	opts := cfg.engineOptions()
	if opts.MaxConcurrentCompaction != 3 || opts.MaxConcurrentDownsample != 2 {
		t.Fatalf("concurrent opts = %#v", opts)
	}
	if !opts.Compression.OmitWriteSeq || opts.Compression.ValuePageSamples != 4096 || opts.Compression.ZstdLevel != "better" {
		t.Fatalf("compression opts = %#v", opts.Compression)
	}
	if opts.QueryPageCache.Limit != 128 || opts.QueryBlockCache.MaxBytes != 1048576 {
		t.Fatalf("cache opts page=%#v block=%#v", opts.QueryPageCache, opts.QueryBlockCache)
	}
	if opts.Cardinality.MaxSeries != 10000 || opts.StorageMemory.HardBytesLimit != 2097152 {
		t.Fatalf("limits opts card=%#v mem=%#v", opts.Cardinality, opts.StorageMemory)
	}
	if !opts.WAL.Sync || opts.WAL.BatchRecords != 100 {
		t.Fatalf("wal opts = %#v", opts.WAL)
	}
}

func TestLoadConfigRejectsAdditionalInvalidConfig(t *testing.T) {
	dataDir := filepath.ToSlash(t.TempDir())
	tests := []struct {
		name string
		body string
	}{
		{name: "grpc tls missing cert", body: "data_dir: " + dataDir + "\ngrpc:\n  enabled: true\n  addr: '127.0.0.1:0'\n  tls:\n    enabled: true\n"},
		{name: "negative concurrent", body: "data_dir: " + dataDir + "\nlimits:\n  max_concurrent_http: -1\n"},
		{name: "negative request timeout", body: "data_dir: " + dataDir + "\nlimits:\n  request_timeout: -1s\n"},
		{name: "negative grpc msg bytes", body: "data_dir: " + dataDir + "\ngrpc:\n  enabled: true\n  addr: '127.0.0.1:0'\n  max_recv_msg_bytes: -1\n"},
		{name: "empty log level", body: "data_dir: " + dataDir + "\nlog:\n  level: ' '\n"},
		{name: "client auth no ca", body: "data_dir: " + dataDir + "\ngrpc:\n  enabled: true\n  addr: '127.0.0.1:0'\n  tls:\n    enabled: true\n    cert_file: 'c'\n    key_file: 'k'\n    client_auth: true\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTestConfig(t, tt.body)
			if _, err := loadConfig(path); !errors.Is(err, errInvalidConfig) {
				t.Fatalf("loadConfig(%s) error = %v, want errInvalidConfig", tt.name, err)
			}
		})
	}
}

func TestDurationTextUnmarshalErrors(t *testing.T) {
	var d durationText
	if err := d.UnmarshalJSON([]byte(`123`)); err == nil {
		t.Fatal("UnmarshalJSON(number) error = nil, want error")
	}
	if err := d.UnmarshalJSON([]byte(`"abc"`)); err == nil {
		t.Fatal("UnmarshalJSON(bad duration) error = nil, want error")
	}
}

func TestCLIServeInvalidConfigPath(t *testing.T) {
	app := newApp(&bytes.Buffer{}, &bytes.Buffer{})
	if err := app.RunContext(context.Background(), []string{"mts-server", "serve", "--config", "/nonexistent-mts.yaml"}); err == nil {
		t.Fatal("Run(serve nonexistent config) error = nil, want error")
	}
}

func TestCLIServeRuntimeOpenFailure(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "data")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile(blocker) error = %v", err)
	}
	cfgPath := filepath.Join(dir, "mts-server.yaml")
	body := fmt.Sprintf("data_dir: %s\nshutdown_timeout: 3s\n", filepath.ToSlash(blocker))
	if err := os.WriteFile(cfgPath, []byte(body), 0600); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	app := newApp(&bytes.Buffer{}, &bytes.Buffer{})
	if err := app.RunContext(context.Background(), []string{"mts-server", "serve", "--config", cfgPath}); err == nil {
		t.Fatal("Run(serve invalid data_dir) error = nil, want error")
	}
}

func TestCLIDoctorErrors(t *testing.T) {
	app := newApp(&bytes.Buffer{}, &bytes.Buffer{})
	if err := app.RunContext(context.Background(), []string{"mts-server", "doctor", "--config", "/nonexistent-mts.yaml"}); err == nil {
		t.Fatal("doctor(nonexistent config) error = nil, want error")
	}
	cfgPath := writeTestConfig(t, "data_dir: "+filepath.ToSlash(t.TempDir())+"\n")
	if err := app.RunContext(context.Background(), []string{"mts-server", "doctor", "--config", cfgPath}); err != nil {
		t.Fatalf("doctor(valid config) error = %v", err)
	}
	failApp := newApp(failWriter{}, &bytes.Buffer{})
	if err := failApp.RunContext(context.Background(), []string{"mts-server", "doctor", "--config", cfgPath}); err == nil {
		t.Fatal("doctor(write fail) error = nil, want error")
	}
}

func TestCLIInitConfigPathErrors(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile(blocker) error = %v", err)
	}
	output := filepath.Join(blocker, "sub", "mts-server.yaml")
	app := newApp(&bytes.Buffer{}, &bytes.Buffer{})
	if err := app.RunContext(context.Background(), []string{"mts-server", "init-config", "--output", output}); err == nil {
		t.Fatal("init-config(parent is file) error = nil, want error")
	}
}

func TestNewLoggerLevels(t *testing.T) {
	for _, level := range []string{"debug", "warn", "error", "info", "bogus"} {
		logger := newLogger(&bytes.Buffer{}, logConfig{Level: level})
		if logger == nil {
			t.Fatalf("newLogger(%q) = nil", level)
		}
	}
}
