package main

func (r *serverRuntime) validateConfigPayload(cfg config) configValidateResponse {
	if err := cfg.validate(); err != nil {
		return configValidateResponse{OK: false, Error: err.Error()}
	}
	return configValidateResponse{OK: true}
}

func (r *serverRuntime) reloadConfig() (reloadConfigResponse, error) {
	old := r.currentConfig()
	newCfg, err := loadConfig(old.ConfigPath)
	if err != nil {
		return reloadConfigResponse{}, err
	}
	r.mu.Lock()
	r.config.Auth = newCfg.Auth
	r.config.User = newCfg.User
	r.config.Limits = newCfg.Limits
	r.config.Observability = newCfg.Observability
	r.config.Log = newCfg.Log
	r.mu.Unlock()
	r.applyLimitState(newCfg)
	return reloadConfigResponse{OK: true, Fields: []string{"auth", "user", "limits", "observability", "log"}}, nil
}
