package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"

	mts "github.com/openmts/mts"
)

type serverRuntime struct {
	mu                     sync.RWMutex
	config                 config
	logger                 *slog.Logger
	metrics                *serverMetrics
	audit                  *auditLog
	httpSem                chan struct{}
	grpcSem                chan struct{}
	requestSeq             atomic.Uint64
	maintenanceBusy        atomic.Bool
	maintenanceOp          atomic.Value // string
	maintenanceStartedUnix atomic.Int64
	maintenanceStartedMs   atomic.Int64
	lastAdminHeavyMu       sync.Mutex
	lastAdminHeavy         *adminHeavyLastResult
	engine                 *mts.Engine
	httpServer             *http.Server
	grpcServer             *grpc.Server
	httpLn                 net.Listener
	grpcLn                 net.Listener
	serveErr               chan error
}

func openRuntime(ctx context.Context, cfg config) (*serverRuntime, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	engine, err := mts.Open(ctx, cfg.engineOptions())
	if err != nil {
		return nil, err
	}
	// bootstrap 不受 serve 父 context 取消影响（避免启动即 cancel 时无法预置管理员）。
	if err := bootstrapDefaultAdmin(context.WithoutCancel(ctx), cfg, engine); err != nil {
		_ = engine.Close(ctx)
		return nil, err
	}
	audit := newAuditLog(256)
	audit.engine = engine
	runtime := &serverRuntime{
		config:  cfg,
		logger:  slog.Default(),
		metrics: newServerMetrics(),
		audit:   audit,
		engine:  engine,
	}
	runtime.applyLimitState(cfg)
	if cfg.HTTP.Enabled {
		tlsConfig, err := buildTLSConfig(cfg.HTTP.TLS)
		if err != nil {
			_ = engine.Close(ctx)
			return nil, err
		}
		runtime.httpServer = &http.Server{
			Addr:              cfg.HTTP.Addr,
			Handler:           runtime.httpHandler(),
			TLSConfig:         tlsConfig,
			ReadHeaderTimeout: time.Duration(cfg.HTTP.ReadHeaderTimeout),
			ReadTimeout:       time.Duration(cfg.HTTP.ReadTimeout),
			WriteTimeout:      time.Duration(cfg.HTTP.WriteTimeout),
			IdleTimeout:       time.Duration(cfg.HTTP.IdleTimeout),
		}
	}
	if cfg.GRPC.Enabled {
		runtime.grpcServer, err = newGRPCServer(runtime)
		if err != nil {
			_ = engine.Close(ctx)
			return nil, err
		}
	}
	return runtime, nil
}

const (
	defaultSystemAdminName     = "admin"
	defaultSystemAdminPassword = "admin"
)

func bootstrapDefaultAdmin(ctx context.Context, cfg config, engine *mts.Engine) error {
	// Dashboard 强制登录；只要密码认证开启就预置默认管理员，便于 POC/演示首访可登录。
	// require_user 仅控制数据面是否强制用户鉴权，不再作为 bootstrap 门槛。
	if cfg.User.PasswordAuthDisabled {
		return nil
	}
	user, ok, err := engine.GetUser(ctx, defaultSystemAdminName)
	if err != nil {
		return err
	}
	if !ok {
		user := mts.User{
			Name:     defaultSystemAdminName,
			Role:     mts.UserRoleAdmin,
			Metadata: withMustChangePassword(nil, true),
		}
		if err := engine.CreateUser(ctx, user); err != nil {
			return err
		}
		return engine.SetPassword(ctx, defaultSystemAdminName, defaultSystemAdminPassword)
	}
	if user.Role == mts.UserRoleAdmin && !user.Disabled {
		return nil
	}
	user.Role = mts.UserRoleAdmin
	user.Disabled = false
	return engine.UpdateUser(ctx, user)
}

func (r *serverRuntime) applyLimitState(cfg config) {
	if cfg.Limits.MaxConcurrentHTTP > 0 {
		r.httpSem = make(chan struct{}, cfg.Limits.MaxConcurrentHTTP)
	} else {
		r.httpSem = nil
	}
	if cfg.Limits.MaxConcurrentGRPC > 0 {
		r.grpcSem = make(chan struct{}, cfg.Limits.MaxConcurrentGRPC)
	} else {
		r.grpcSem = nil
	}
}

func (r *serverRuntime) setLogger(logger *slog.Logger) {
	if logger == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logger = logger
}

func (r *serverRuntime) currentConfig() config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

func (r *serverRuntime) currentLogger() *slog.Logger {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.logger
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
			err := r.serveHTTP(ln)
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				r.reportServeError(fmt.Errorf("http server stopped: %w", err))
			}
		}()
	}
	if r.grpcServer != nil {
		ln, err := net.Listen("tcp", r.config.GRPC.Addr)
		if err != nil {
			_ = r.shutdown(context.Background())
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

func (r *serverRuntime) serveHTTP(ln net.Listener) error {
	if r.config.HTTP.TLS.Enabled {
		return r.httpServer.ServeTLS(ln, "", "")
	}
	return r.httpServer.Serve(ln)
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
	if r.audit != nil {
		r.audit.close()
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
	if max := r.currentConfig().Limits.MaxWritePoints; max > 0 && len(req.Points) > max {
		return newAPIError(errorCodeBadRequest, "too many points in write request", nil)
	}
	return r.engine.Write(ctx, req.Points, req.Options)
}

func (r *serverRuntime) writeTypedBatch(ctx context.Context, req typedWriteRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if max := r.currentConfig().Limits.MaxWritePoints; max > 0 && len(req.Batch.Timestamps) > max {
		return newAPIError(errorCodeBadRequest, "too many points in typed write request", nil)
	}
	return r.engine.WriteTypedBatch(ctx, req.Batch, req.Options)
}

func (r *serverRuntime) writePointsAsTyped(ctx context.Context, req writeRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if max := r.currentConfig().Limits.MaxWritePoints; max > 0 && len(req.Points) > max {
		return newAPIError(errorCodeBadRequest, "too many points in write request", nil)
	}
	return r.engine.WritePointsAsTypedBatch(ctx, req.Points, req.Options)
}

func (r *serverRuntime) deleteData(ctx context.Context, req mts.DeleteRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.engine.Delete(ctx, req)
}

func (r *serverRuntime) queryRows(ctx context.Context, req queryRowsRequest) ([]mts.Row, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query, err := r.limitedQuery(req.Query)
	if err != nil {
		return nil, err
	}
	return r.engine.QueryRows(ctx, query)
}

func (r *serverRuntime) queryColumns(ctx context.Context, req queryRequest) ([]mts.ColumnSeries, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query, err := r.limitedQuery(req.Query)
	if err != nil {
		return nil, err
	}
	return r.engine.QueryColumns(ctx, query)
}

func (r *serverRuntime) queryWithExplain(ctx context.Context, req queryRequest) (mts.QueryResult, error) {
	if err := ctx.Err(); err != nil {
		return mts.QueryResult{}, err
	}
	query, err := r.limitedQuery(req.Query)
	if err != nil {
		return mts.QueryResult{}, err
	}
	return r.engine.QueryWithExplain(ctx, query)
}

func (r *serverRuntime) limitedQuery(query mts.Query) (mts.Query, error) {
	cfg := r.currentConfig()
	if cfg.Limits.MaxQueryLimit > 0 && query.Limit > cfg.Limits.MaxQueryLimit {
		return mts.Query{}, newAPIError(errorCodeBadRequest, "query limit exceeds max_query_limit", nil)
	}
	if query.Limit == 0 && cfg.Limits.DefaultQueryLimit > 0 {
		query.Limit = cfg.Limits.DefaultQueryLimit
	}
	return query, nil
}

func (r *serverRuntime) queryStats() mts.QueryStats {
	return r.engine.QueryStatsSnapshot()
}

// tryBeginAdminHeavy 串行化 flush/compact/retention 与 storage 重操作，避免 Dashboard 并发压垮引擎/磁盘。
func (r *serverRuntime) tryBeginAdminHeavy(op string) error {
	if !r.maintenanceBusy.CompareAndSwap(false, true) {
		cur := ""
		if v := r.maintenanceOp.Load(); v != nil {
			cur, _ = v.(string)
		}
		return newAdminHeavyBusyError(cur)
	}
	if op == "" {
		op = "admin_heavy"
	}
	now := time.Now()
	r.maintenanceOp.Store(op)
	r.maintenanceStartedUnix.Store(now.Unix())
	r.maintenanceStartedMs.Store(now.UnixMilli())
	return nil
}

func (r *serverRuntime) endAdminHeavy() {
	r.finishAdminHeavy(nil)
}

// finishAdminHeavy 释放互斥并记录最近一次结果；err 非 nil 时写入 last.error。
func (r *serverRuntime) finishAdminHeavy(err error) {
	op := ""
	if v := r.maintenanceOp.Load(); v != nil {
		op, _ = v.(string)
	}
	startedUnix := r.maintenanceStartedUnix.Load()
	startedMs := r.maintenanceStartedMs.Load()
	now := time.Now()
	finishedUnix := now.Unix()
	durationMs := int64(0)
	if startedMs > 0 {
		durationMs = now.UnixMilli() - startedMs
		if durationMs < 0 {
			durationMs = 0
		}
	} else if startedUnix > 0 && finishedUnix >= startedUnix {
		durationMs = (finishedUnix - startedUnix) * 1000
	}
	result := &adminHeavyLastResult{
		Op:             op,
		OK:             err == nil,
		StartedAtUnix:  startedUnix,
		FinishedAtUnix: finishedUnix,
		DurationMs:     durationMs,
	}
	if err != nil {
		result.Error = err.Error()
		result.OK = false
	}
	r.lastAdminHeavyMu.Lock()
	r.lastAdminHeavy = result
	r.lastAdminHeavyMu.Unlock()
	r.maintenanceBusy.Store(false)
	r.maintenanceOp.Store("")
	r.maintenanceStartedUnix.Store(0)
	r.maintenanceStartedMs.Store(0)
}

func (r *serverRuntime) lastAdminHeavySnapshot() *adminHeavyLastResult {
	r.lastAdminHeavyMu.Lock()
	defer r.lastAdminHeavyMu.Unlock()
	if r.lastAdminHeavy == nil {
		return nil
	}
	cp := *r.lastAdminHeavy
	return &cp
}

// 兼容旧名（运维路径）。
func (r *serverRuntime) tryBeginMaintenance() error { return r.tryBeginAdminHeavy("maintenance") }

func (r *serverRuntime) endMaintenance() { r.endAdminHeavy() }

func (r *serverRuntime) flush(ctx context.Context) (err error) {
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = r.tryBeginAdminHeavy("flush"); err != nil {
		return err
	}
	defer func() { r.finishAdminHeavy(err) }()
	err = r.engine.Flush(ctx)
	return err
}

func (r *serverRuntime) compact(ctx context.Context) (result mts.CompactionResult, err error) {
	if err = ctx.Err(); err != nil {
		return mts.CompactionResult{}, err
	}
	if err = r.tryBeginAdminHeavy("compact"); err != nil {
		return mts.CompactionResult{}, err
	}
	defer func() { r.finishAdminHeavy(err) }()
	result, err = r.engine.CompactWithResult(ctx)
	return result, err
}

func (r *serverRuntime) applyRetention(ctx context.Context, req retentionApplyRequest) (err error) {
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = r.tryBeginAdminHeavy("retention"); err != nil {
		return err
	}
	defer func() { r.finishAdminHeavy(err) }()
	err = r.engine.ApplyRetention(ctx, unixNanosOrNow(req.NowUnixNanos))
	return err
}

func (r *serverRuntime) maintenanceErrors(ctx context.Context) []string {
	errs := r.engine.MaintenanceErrors(ctx)
	out := make([]string, len(errs))
	for index, e := range errs {
		out[index] = e.Error()
	}
	return out
}

func (r *serverRuntime) storageMemory() mts.StorageMemorySnapshot {
	return r.engine.StorageMemorySnapshot()
}

func (r *serverRuntime) compactionStats() mts.CompactionStats {
	return r.engine.CompactionStatsSnapshot()
}

func (r *serverRuntime) maintenanceStats() mts.MaintenanceStats {
	return r.engine.MaintenanceStatsSnapshot()
}

func (r *serverRuntime) maintenanceStatsPayload() maintenanceStatsResponse {
	busy, op, started := r.adminHeavyState()
	return maintenanceStatsResponse{
		Stats:         r.maintenanceStats(),
		AdminOpBusy:   busy,
		Op:            op,
		StartedAtUnix: started,
		Last:          r.lastAdminHeavySnapshot(),
	}
}

func (r *serverRuntime) opsStatusPayload() opsStatusResponse {
	busy, op, started := r.adminHeavyState()
	return opsStatusResponse{
		AdminOpBusy:   busy,
		Op:            op,
		StartedAtUnix: started,
		Last:          r.lastAdminHeavySnapshot(),
	}
}

func (r *serverRuntime) adminHealthPayload() adminHealthResponse {
	busy, op, started := r.adminHeavyState()
	return adminHealthResponse{
		Health:        r.health(),
		AdminOpBusy:   busy,
		Op:            op,
		StartedAtUnix: started,
		Last:          r.lastAdminHeavySnapshot(),
	}
}

func (r *serverRuntime) adminVersionPayload() versionResponse {
	busy, op, started := r.adminHeavyState()
	return versionResponse{
		Version:       version,
		Commit:        commit,
		BuiltAt:       builtAt,
		AdminOpBusy:   busy,
		Op:            op,
		StartedAtUnix: started,
		Last:          r.lastAdminHeavySnapshot(),
	}
}

func (r *serverRuntime) adminHeavyState() (busy bool, op string, startedAtUnix int64) {
	busy = r.maintenanceBusy.Load()
	if v := r.maintenanceOp.Load(); v != nil {
		op, _ = v.(string)
	}
	startedAtUnix = r.maintenanceStartedUnix.Load()
	if !busy {
		op = ""
		startedAtUnix = 0
	}
	return busy, op, startedAtUnix
}

func (r *serverRuntime) health() mts.HealthSnapshot {
	return r.engine.HealthSnapshot()
}

func (r *serverRuntime) effectiveConfig() config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}
