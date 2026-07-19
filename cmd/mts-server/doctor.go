package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// doctorCheck 是可商用部署检查的一条结构化结果。
type doctorCheck struct {
	Level   string `json:"level"` // ok | warn
	Code    string `json:"code"`
	Message string `json:"message"`
}

// doctorResponse 供 HTTP admin/doctor 与 CLI 共用。
type doctorResponse struct {
	OK             bool          `json:"ok"`
	HTTPTLSEnabled bool          `json:"http_tls_enabled"`
	Checks         []doctorCheck `json:"checks"`
	Lines          []string      `json:"lines"`
}

// evaluateDoctor 执行部署前检查；致命错误返回 error，warn 仍 ok=true。
func evaluateDoctor(cfg config) (doctorResponse, error) {
	resp := doctorResponse{
		OK:             true,
		HTTPTLSEnabled: cfg.HTTP.TLS.Enabled,
		Checks:         make([]doctorCheck, 0, 8),
	}
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return doctorResponse{}, fmt.Errorf("data_dir: %w", err)
	}
	resp.Checks = append(resp.Checks, doctorCheck{
		Level: "ok", Code: "data_dir", Message: "data_dir ready: " + cfg.DataDir,
	})

	backupDir := cfg.Backup.Dir
	if backupDir == "" {
		backupDir = filepath.Join(cfg.DataDir, "backups")
	}
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return doctorResponse{}, fmt.Errorf("backup_dir: %w", err)
	}
	resp.Checks = append(resp.Checks, doctorCheck{
		Level: "ok", Code: "backup_dir", Message: "backup_dir ready: " + backupDir,
	})

	if cfg.HTTP.TLS.Enabled {
		if _, err := buildTLSConfig(cfg.HTTP.TLS); err != nil {
			return doctorResponse{}, fmt.Errorf("http tls: %w", err)
		}
		resp.Checks = append(resp.Checks, doctorCheck{
			Level: "ok", Code: "http_tls", Message: "http tls enabled (HSTS will be emitted by server)",
		})
	} else {
		resp.Checks = append(resp.Checks, doctorCheck{
			Level: "warn", Code: "http_tls",
			Message: "http tls disabled; terminate HTTPS/HSTS at reverse proxy edge",
		})
	}

	if cfg.GRPC.TLS.Enabled {
		if _, err := buildTLSConfig(cfg.GRPC.TLS); err != nil {
			return doctorResponse{}, fmt.Errorf("grpc tls: %w", err)
		}
		resp.Checks = append(resp.Checks, doctorCheck{
			Level: "ok", Code: "grpc_tls", Message: "grpc tls enabled",
		})
	} else if cfg.GRPC.Enabled {
		resp.Checks = append(resp.Checks, doctorCheck{
			Level: "warn", Code: "grpc_tls",
			Message: "grpc tls disabled; keep gRPC on private network or enable tls",
		})
	}

	if !cfg.User.PasswordAuthDisabled {
		resp.Checks = append(resp.Checks, doctorCheck{
			Level: "ok", Code: "password_auth",
			Message: "password auth enabled (bootstrap admin requires password change)",
		})
	} else {
		resp.Checks = append(resp.Checks, doctorCheck{
			Level: "warn", Code: "password_auth", Message: "password auth disabled",
		})
	}

	if strings.TrimSpace(cfg.Auth.AdminToken) == "" && !cfg.Auth.RequireUser {
		resp.Checks = append(resp.Checks, doctorCheck{
			Level: "warn", Code: "auth_hardening",
			Message: "admin_token empty and require_user=false; tighten auth for production",
		})
	}

	resp.Lines = doctorLines(resp.Checks)
	return resp, nil
}

func doctorLines(checks []doctorCheck) []string {
	lines := make([]string, 0, len(checks))
	for _, c := range checks {
		lines = append(lines, c.Level+": "+c.Message)
	}
	return lines
}

// runDoctorChecks 保留 CLI 文本输出契约。
func runDoctorChecks(cfg config) ([]string, error) {
	resp, err := evaluateDoctor(cfg)
	if err != nil {
		return nil, err
	}
	return resp.Lines, nil
}
