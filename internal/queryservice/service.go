package queryservice

import (
	"context"
	"sync/atomic"

	"github.com/openmts/mts/internal/model"
)

type Executor interface {
	Query(ctx context.Context, query model.Query) (Result, error)
}

type Options struct {
	MaxConcurrent int
}

type Service struct {
	options  Options
	executor Executor
	active   int64
}

func New(options Options, executor Executor) *Service {
	return &Service{
		options:  options,
		executor: executor,
	}
}

func (s *Service) Admit(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !s.tryAcquire() {
		return nil, ErrAdmissionRejected
	}
	var released atomic.Bool
	return func() {
		if released.CompareAndSwap(false, true) {
			atomic.AddInt64(&s.active, -1)
		}
	}, nil
}

func (s *Service) Query(ctx context.Context, request Request) (Result, error) {
	release, err := s.Admit(ctx)
	if err != nil {
		return Result{}, err
	}
	defer release()
	if s.executor == nil {
		return Result{}, nil
	}
	return s.executor.Query(ctx, request.Query)
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
