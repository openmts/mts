package queryservice

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/openmts/mts/internal/model"
)

type Executor interface {
	Query(ctx context.Context, query model.Query) (Result, error)
}

type Options struct {
	MaxConcurrent     int
	MaxQueued         int
	DefaultTimeout    time.Duration
	QueuePollInterval time.Duration
	CacheMaxEntries   int
	AuditMaxRecords   int
	AllowedTenants    []string
	Authorizer        Authorizer
}

type Service struct {
	options           Options
	executor          Executor
	cache             *resultCache
	audit             *auditLog
	authorizer        Authorizer
	active            int64
	queued            int64
	totalAdmitted     int64
	totalQueued       int64
	totalRejected     int64
	totalTimedOut     int64
	totalUnauthorized int64
	totalCacheHits    int64
	totalCacheMisses  int64
}

func New(options Options, executor Executor) *Service {
	authorizer := options.Authorizer
	if authorizer == nil {
		authorizer = newStaticTenantAuthorizer(options.AllowedTenants)
	}
	return &Service{
		options:    options,
		executor:   executor,
		cache:      newResultCache(options.CacheMaxEntries),
		audit:      newAuditLog(options.AuditMaxRecords),
		authorizer: authorizer,
	}
}

func (s *Service) Admit(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		s.recordTimeout(err)
		return nil, err
	}
	if s.tryAcquire() {
		atomic.AddInt64(&s.totalAdmitted, 1)
		return s.releaseFunc(), nil
	}
	if s.options.MaxQueued <= 0 {
		atomic.AddInt64(&s.totalRejected, 1)
		return nil, ErrAdmissionRejected
	}
	if !s.tryQueue() {
		atomic.AddInt64(&s.totalRejected, 1)
		return nil, ErrQueueFull
	}
	atomic.AddInt64(&s.totalQueued, 1)
	defer atomic.AddInt64(&s.queued, -1)
	ticker := time.NewTicker(s.queuePollInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.recordTimeout(ctx.Err())
			return nil, ctx.Err()
		case <-ticker.C:
			if s.tryAcquire() {
				atomic.AddInt64(&s.totalAdmitted, 1)
				return s.releaseFunc(), nil
			}
		}
	}
}

func (s *Service) releaseFunc() func() {
	var released atomic.Bool
	return func() {
		if released.CompareAndSwap(false, true) {
			atomic.AddInt64(&s.active, -1)
		}
	}
}

func (s *Service) Query(ctx context.Context, request Request) (Result, error) {
	started := time.Now()
	queryCtx, cancel := s.queryContext(ctx, request)
	if err := s.authorize(queryCtx, request); err != nil {
		cancel()
		atomic.AddInt64(&s.totalUnauthorized, 1)
		s.recordAudit(request, false, ErrorCodeUnauthorized, started)
		return Result{}, err
	}
	if result, hit := s.cacheLookup(request); hit {
		cancel()
		s.recordAudit(request, true, "", started)
		return result, nil
	}
	release, err := s.Admit(queryCtx)
	if err != nil {
		cancel()
		s.recordAudit(request, false, errorCode(err), started)
		return Result{}, err
	}
	defer func() {
		release()
		cancel()
	}()
	if s.executor == nil {
		s.recordAudit(request, true, "", started)
		return Result{}, nil
	}
	result, err := s.executor.Query(queryCtx, request.Query)
	s.recordTimeout(err)
	if err != nil {
		s.recordAudit(request, false, errorCode(err), started)
		return result, err
	}
	s.cacheStore(request, result)
	s.recordAudit(request, true, "", started)
	return result, err
}

func (s *Service) QueryStream(ctx context.Context, request Request) (StreamResult, error) {
	started := time.Now()
	queryCtx, cancel := s.queryContext(ctx, request)
	if err := s.authorize(queryCtx, request); err != nil {
		cancel()
		atomic.AddInt64(&s.totalUnauthorized, 1)
		s.recordAudit(request, false, ErrorCodeUnauthorized, started)
		return StreamResult{}, err
	}
	release, err := s.Admit(queryCtx)
	if err != nil {
		cancel()
		s.recordAudit(request, false, errorCode(err), started)
		return StreamResult{}, err
	}
	releaseAll := func() {
		release()
		cancel()
	}
	if s.executor == nil {
		releaseAll()
		s.recordAudit(request, true, "", started)
		return StreamResult{}, nil
	}
	streaming, ok := s.executor.(StreamingExecutor)
	if !ok {
		releaseAll()
		s.recordAudit(request, false, ErrorCodeStreamingUnsupported, started)
		return StreamResult{}, ErrStreamingUnsupported
	}
	result, err := streaming.QueryStream(queryCtx, request.Query)
	if err != nil {
		releaseAll()
		s.recordTimeout(err)
		s.recordAudit(request, false, errorCode(err), started)
		return StreamResult{}, err
	}
	s.recordAudit(request, true, "", started)
	return withStreamRelease(result, releaseAll), nil
}

func (s *Service) tryAcquire() bool {
	for {
		current := atomic.LoadInt64(&s.active)
		if s.options.MaxConcurrent > 0 && current >= int64(s.options.MaxConcurrent) {
			return false
		}
		if atomic.CompareAndSwapInt64(&s.active, current, current+1) {
			return true
		}
	}
}

func (s *Service) Stats() ServiceStats {
	return ServiceStats{
		Active:            atomic.LoadInt64(&s.active),
		Queued:            atomic.LoadInt64(&s.queued),
		TotalAdmitted:     atomic.LoadInt64(&s.totalAdmitted),
		TotalQueued:       atomic.LoadInt64(&s.totalQueued),
		TotalRejected:     atomic.LoadInt64(&s.totalRejected),
		TotalTimedOut:     atomic.LoadInt64(&s.totalTimedOut),
		TotalUnauthorized: atomic.LoadInt64(&s.totalUnauthorized),
		TotalCacheHits:    atomic.LoadInt64(&s.totalCacheHits),
		TotalCacheMisses:  atomic.LoadInt64(&s.totalCacheMisses),
		TotalAuditRecords: s.auditTotal(),
	}
}

func (s *Service) AuditRecords() []AuditRecord {
	records, _ := s.audit.snapshot()
	return records
}

func (s *Service) InvalidateCache() {
	s.cache.clear()
}

func (s *Service) tryQueue() bool {
	for {
		current := atomic.LoadInt64(&s.queued)
		if current >= int64(s.options.MaxQueued) {
			return false
		}
		if atomic.CompareAndSwapInt64(&s.queued, current, current+1) {
			return true
		}
	}
}

func (s *Service) queryContext(ctx context.Context, request Request) (context.Context, context.CancelFunc) {
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = s.options.DefaultTimeout
	}
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func (s *Service) queuePollInterval() time.Duration {
	if s.options.QueuePollInterval > 0 {
		return s.options.QueuePollInterval
	}
	return time.Millisecond
}

func (s *Service) recordTimeout(err error) {
	if err == context.DeadlineExceeded {
		atomic.AddInt64(&s.totalTimedOut, 1)
	}
}

func (s *Service) authorize(ctx context.Context, request Request) error {
	if s.authorizer == nil {
		return nil
	}
	return s.authorizer.AuthorizeQuery(ctx, Principal{
		Tenant: request.Tenant,
		User:   request.User,
	}, request.Query)
}

func (s *Service) cacheLookup(request Request) (Result, bool) {
	if s.cache == nil {
		return Result{}, false
	}
	key, ok := cacheKey(request)
	if !ok {
		return Result{}, false
	}
	result, hit := s.cache.get(key)
	if hit {
		atomic.AddInt64(&s.totalCacheHits, 1)
		return result, true
	}
	atomic.AddInt64(&s.totalCacheMisses, 1)
	return Result{}, false
}

func (s *Service) cacheStore(request Request, result Result) {
	if s.cache == nil {
		return
	}
	key, ok := cacheKey(request)
	if ok {
		s.cache.set(key, result)
	}
}

func (s *Service) recordAudit(
	request Request,
	accepted bool,
	code ErrorCode,
	started time.Time,
) {
	if s.audit == nil {
		return
	}
	s.audit.append(newAuditRecord(request, accepted, code, started))
}

func (s *Service) auditTotal() int64 {
	_, total := s.audit.snapshot()
	return total
}
