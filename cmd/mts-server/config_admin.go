package main

import (
	"fmt"
	"strings"
)

func (r *serverRuntime) validateConfigPayload(cfg config) configValidateResponse {
	if err := cfg.validate(); err != nil {
		return configValidateResponse{OK: false, Path: routeAdminConfigValidate, Error: err.Error()}
	}
	return configValidateResponse{OK: true, Path: routeAdminConfigValidate}
}

func (r *serverRuntime) reloadConfig() (reloadConfigResponse, error) {
	old := r.currentConfig()
	newCfg, err := loadConfig(old.ConfigPath)
	if err != nil {
		return reloadConfigResponse{}, err
	}
	if fields := reloadRestartFields(old, newCfg); len(fields) > 0 {
		return reloadConfigResponse{}, fmt.Errorf(
			"configuration fields require restart: %s",
			strings.Join(fields, ", "),
		)
	}
	r.mu.Lock()
	r.config.Auth = newCfg.Auth
	r.config.Limits = newCfg.Limits
	r.config.Observability = newCfg.Observability
	r.applyLimitState(newCfg)
	r.mu.Unlock()
	return reloadConfigResponse{
		OK:     true,
		Path:   routeAdminConfigReload,
		Fields: []string{"auth", "limits", "observability"},
	}, nil
}

func reloadRestartFields(old config, newCfg config) []string {
	fields := make([]string, 0, 2)
	if old.User != newCfg.User {
		fields = append(fields, "user")
	}
	if old.Log != newCfg.Log {
		fields = append(fields, "log")
	}
	return fields
}
