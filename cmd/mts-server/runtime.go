package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"google.golang.org/grpc"

	mts "github.com/openmts/mts"
)

type serverRuntime struct {
	config     config
	engine     *mts.Engine
	httpServer *http.Server
	grpcServer *grpc.Server
	httpLn     net.Listener
	grpcLn     net.Listener
	serveErr   chan error
}

func openRuntime(ctx context.Context, cfg config) (*serverRuntime, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	engine, err := mts.Open(ctx, cfg.engineOptions())
	if err != nil {
		return nil, err
	}
	runtime := &serverRuntime{config: cfg, engine: engine}
	if cfg.HTTP.Enabled {
		runtime.httpServer = &http.Server{
			Addr:              cfg.HTTP.Addr,
			Handler:           runtime.httpHandler(),
			ReadHeaderTimeout: 5 * time.Second,
		}
	}
	if cfg.GRPC.Enabled {
		runtime.grpcServer = newGRPCServer(runtime)
	}
	return runtime, nil
}

func (r *serverRuntime) start() error {
	r.serveErr = make(chan error, 2)
	if r.httpServer != nil {
		ln, err := net.Listen("tcp", r.config.HTTP.Addr)
		if err != nil {
			return err
		}
		r.httpLn = ln
		go func() {
			err := r.httpServer.Serve(ln)
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				r.reportServeError(fmt.Errorf("http server stopped: %w", err))
			}
		}()
	}
	if r.grpcServer != nil {
		ln, err := net.Listen("tcp", r.config.GRPC.Addr)
		if err != nil {
			return err
		}
		r.grpcLn = ln
		go func() {
			if err := r.grpcServer.Serve(ln); err != nil {
				if !errors.Is(err, grpc.ErrServerStopped) {
					r.reportServeError(fmt.Errorf("grpc server stopped: %w", err))
				}
			}
		}()
	}
	return nil
}

func (r *serverRuntime) reportServeError(err error) {
	select {
	case r.serveErr <- err:
	default:
	}
}

func (r *serverRuntime) serveErrors() <-chan error {
	return r.serveErr
}

func (r *serverRuntime) shutdown(ctx context.Context) error {
	var err error
	if r.httpServer != nil {
		err = errors.Join(err, r.httpServer.Shutdown(ctx))
	}
	if r.grpcServer != nil {
		stopped := make(chan struct{})
		go func() {
			r.grpcServer.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-ctx.Done():
			r.grpcServer.Stop()
			err = errors.Join(err, ctx.Err())
		}
	}
	if r.engine != nil {
		err = errors.Join(err, r.engine.Close(ctx))
	}
	return err
}

func (r *serverRuntime) write(ctx context.Context, req writeRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.engine.Write(ctx, req.Points, req.Options)
}

func (r *serverRuntime) queryRows(ctx context.Context, req queryRowsRequest) ([]mts.Row, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.engine.QueryRows(ctx, req.Query)
}

func (r *serverRuntime) flush(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.engine.Flush(ctx)
}

func (r *serverRuntime) compact(ctx context.Context) (mts.CompactionResult, error) {
	if err := ctx.Err(); err != nil {
		return mts.CompactionResult{}, err
	}
	return r.engine.CompactWithResult(ctx)
}

func (r *serverRuntime) health() mts.HealthSnapshot {
	return r.engine.HealthSnapshot()
}

type writeRequest struct {
	Points  []mts.Point      `json:"points"`
	Options mts.WriteOptions `json:"options"`
}

type writeResponse struct {
	OK bool `json:"ok"`
}

type queryRowsRequest struct {
	Query mts.Query `json:"query"`
}

type queryRowsResponse struct {
	Rows []mts.Row `json:"rows"`
}

type maintenanceResponse struct {
	OK     bool                 `json:"ok"`
	Result mts.CompactionResult `json:"result,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}
