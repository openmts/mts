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

func (r *serverRuntime) writeTypedBatch(ctx context.Context, req typedWriteRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.engine.WriteTypedBatch(ctx, req.Batch, req.Options)
}

func (r *serverRuntime) queryRows(ctx context.Context, req queryRowsRequest) ([]mts.Row, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.engine.QueryRows(ctx, req.Query)
}

func (r *serverRuntime) queryColumns(ctx context.Context, req queryRequest) ([]mts.ColumnSeries, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.engine.QueryColumns(ctx, req.Query)
}

func (r *serverRuntime) queryWithExplain(ctx context.Context, req queryRequest) (mts.QueryResult, error) {
	if err := ctx.Err(); err != nil {
		return mts.QueryResult{}, err
	}
	return r.engine.QueryWithExplain(ctx, req.Query)
}

func (r *serverRuntime) queryStats() mts.QueryStats {
	return r.engine.QueryStatsSnapshot()
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

func (r *serverRuntime) applyRetention(ctx context.Context, req retentionApplyRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.engine.ApplyRetention(ctx, unixNanosOrNow(req.NowUnixNanos))
}

func (r *serverRuntime) maintenanceErrors(ctx context.Context) []string {
	errors := r.engine.MaintenanceErrors(ctx)
	out := make([]string, len(errors))
	for index, err := range errors {
		out[index] = err.Error()
	}
	return out
}

func (r *serverRuntime) storageMemory() mts.StorageMemorySnapshot {
	return r.engine.StorageMemorySnapshot()
}

func (r *serverRuntime) compactionStats() mts.CompactionStats {
	return r.engine.CompactionStatsSnapshot()
}

func (r *serverRuntime) health() mts.HealthSnapshot {
	return r.engine.HealthSnapshot()
}

func (r *serverRuntime) effectiveConfig() config {
	return r.config
}
