package service

import "net/http"

type HealthProvider interface {
	HealthSnapshot() Health
}

type Health struct {
	Healthy bool          `json:"healthy"`
	Ready   bool          `json:"ready"`
	Reasons []string      `json:"reasons"`
	Checks  []HealthCheck `json:"checks"`
}

type HealthCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

func healthHandler(provider HealthProvider, readiness bool) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		health := Health{Healthy: true, Ready: true, Reasons: []string{}}
		if provider != nil {
			health = provider.HealthSnapshot()
		}
		ok := health.Healthy
		if readiness {
			ok = health.Ready
		}
		if !ok {
			writeJSON(writer, http.StatusServiceUnavailable, health)
			return
		}
		writeJSON(writer, http.StatusOK, health)
	}
}
