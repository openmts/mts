package service

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/openmts/mts/internal/observability"
)

type Options struct {
	Addr         string
	AdminTimeout time.Duration
	EnablePprof  bool
	AuditLogger  AuditLogger
}

type MetricsProvider interface {
	MetricsSnapshot() []observability.Metric
}

type CompactFunc func(ctx context.Context) error

type AuditLogger interface {
	LogAdminAction(event AdminAuditEvent)
}

type Server struct {
	options Options
	server  *http.Server
}

func NewServer(options Options, metrics MetricsProvider, health HealthProvider, compact CompactFunc) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", metricsHandler(metrics))
	mux.HandleFunc("/healthz", healthHandler(health, false))
	mux.HandleFunc("/readyz", healthHandler(health, true))
	mux.HandleFunc("/admin/compact", compactHandler(options.AdminTimeout, options.AuditLogger, compact))
	if options.EnablePprof {
		registerPprof(mux)
	}
	return &Server{
		options: options,
		server:  &http.Server{Addr: options.Addr, Handler: mux},
	}
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.options.Addr)
	if err != nil {
		return err
	}
	go func() {
		err := s.server.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			return
		}
	}()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) HTTPHandler() http.Handler {
	return s.server.Handler
}

func metricsHandler(provider MetricsProvider) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/plain; version=0.0.4")
		if provider == nil {
			_, _ = writer.Write([]byte{})
			return
		}
		_, _ = writer.Write([]byte(observability.PrometheusText(provider.MetricsSnapshot())))
	}
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func registerPprof(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}
