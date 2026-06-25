package main

import (
	"bytes"
	"context"
	"errors"
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

func TestCLIDefaultConfigAndMissingConfig(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(&stdout, &stderr)
	if err := app.RunContext(context.Background(), []string{"mts-server", "serve", "--print-config"}); err != nil {
		t.Fatalf("Run(print-config) error = %v", err)
	}
	if !strings.Contains(stdout.String(), "data_dir:") {
		t.Fatalf("print-config output = %s, want data_dir", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := app.RunContext(context.Background(), []string{"mts-server", "serve"}); err == nil {
		t.Fatal("Run(missing config) error = nil, want error")
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

	stdout.Reset()
	stderr.Reset()
	if code := runMain([]string{"mts-server", "serve"}, &stdout, &stderr); code != 1 {
		t.Fatalf("runMain(missing config) code = %d, want 1", code)
	}
	if stderr.Len() == 0 {
		t.Fatal("runMain(missing config) stderr is empty, want error output")
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
