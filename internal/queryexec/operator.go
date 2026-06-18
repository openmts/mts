package queryexec

import (
	"context"
	"sync"
)

type Record struct{}

type Operator interface {
	ID() string
	Next(ctx context.Context) (Record, bool)
	Err() error
	Close() error
}

type CountingOperator struct {
	id     string
	total  int
	count  int
	closed bool
	mu     sync.Mutex
}

func NewCountingOperator(id string, total int) *CountingOperator {
	return &CountingOperator{id: id, total: total}
}

func (o *CountingOperator) ID() string {
	return o.id
}

func (o *CountingOperator) Next(ctx context.Context) (Record, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || ctx.Err() != nil || o.count >= o.total {
		return Record{}, false
	}
	o.count++
	return Record{}, true
}

func (o *CountingOperator) Err() error {
	return nil
}

func (o *CountingOperator) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.closed = true
	return nil
}

func (o *CountingOperator) Closed() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.closed
}
