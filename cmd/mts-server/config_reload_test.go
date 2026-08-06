package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestReloadConfigRejectsRestartOnlyFields(t *testing.T) {
	runtime := openTestRuntime(t)
	old := runtime.currentConfig()
	path := writeRuntimeConfig(t, old)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(config) error = %v", err)
	}
	body = bytes.Replace(body, []byte("log:\n  level: info"), []byte(
		"user:\n  endpoint: local\n  password_auth_disabled: true\nlog:\n  level: debug",
	), 1)
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	runtime.config.ConfigPath = path

	_, err = runtime.reloadConfig()
	if err == nil || !strings.Contains(err.Error(), "require restart") {
		t.Fatalf("reloadConfig() error = %v, want require restart", err)
	}
	current := runtime.currentConfig()
	if current.User != old.User || current.Log != old.Log {
		t.Fatalf("config changed after rejected reload: user=%#v log=%#v", current.User, current.Log)
	}
}

func TestReloadConfigReportsOnlyCommittedFields(t *testing.T) {
	runtime := openTestRuntime(t)
	newCfg := runtime.currentConfig()
	newCfg.Auth.AdminToken = "reloaded-admin-token"
	runtime.config.ConfigPath = writeRuntimeConfig(t, newCfg)

	response, err := runtime.reloadConfig()
	if err != nil {
		t.Fatalf("reloadConfig() error = %v", err)
	}
	want := []string{"auth", "limits", "observability"}
	if strings.Join(response.Fields, ",") != strings.Join(want, ",") {
		t.Fatalf("fields = %#v, want %#v", response.Fields, want)
	}
	if runtime.currentConfig().Auth.AdminToken != "reloaded-admin-token" {
		t.Fatal("admin token was not committed")
	}
}
