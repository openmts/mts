package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/openmts/mts/internal/observability"
)

type Options struct {
	Addr         string
	AdminTimeout time.Duration
	EnableAdmin  bool
	AdminToken   string
	EnablePprof  bool
	AuditLogger  AuditLogger
	Logger       *slog.Logger
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
	logger  *slog.Logger
}

func NewServer(options Options, metrics MetricsProvider, health HealthProvider, compact CompactFunc) *Server {
	logger := options.Logger
	if logger == nil {
		logger = slog.New(nopServiceHandler{})
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", metricsHandler(metrics))
	mux.HandleFunc("/healthz", healthHandler(health, false))
	mux.HandleFunc("/readyz", healthHandler(health, true))
	if options.EnableAdmin {
		mux.HandleFunc("/admin/compact", compactHandler(
			options.AdminTimeout,
			options.AdminToken,
			options.AuditLogger,
			compact,
			logger,
		))
	}
	if options.EnablePprof {
		registerPprof(mux)
	}
	return &Server{
		options: options,
		server:  &http.Server{Addr: options.Addr, Handler: mux},
		logger:  logger,
	}
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.options.Addr)
	if err != nil {
		return err
	}
	s.logger.Info("service listening", "addr", s.options.Addr)
	go func() {
		err := s.server.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			s.logger.Error("http server stopped unexpectedly", "error", err)
			return
		}
	}()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("service shutdown")
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

// nopServiceHandler 是 slog.Handler 的空操作实现，用于 nil Logger 归一化。
type nopServiceHandler struct{}

func (nopServiceHandler) Enabled(_ context.Context, _ slog.Level) bool { return false }

func (nopServiceHandler) Handle(_ context.Context, _ slog.Record) error { return nil }

func (h nopServiceHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }

func (h nopServiceHandler) WithGroup(_ string) slog.Handler { return h }
