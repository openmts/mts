package main

import "sync"

type requestLimiter struct {
	mu       sync.Mutex
	limit    int
	inFlight int
}

func (l *requestLimiter) setLimit(limit int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.limit = limit
}

func (l *requestLimiter) tryAcquire() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.limit > 0 && l.inFlight >= l.limit {
		return false
	}
	l.inFlight++
	return true
}

func (l *requestLimiter) release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inFlight > 0 {
		l.inFlight--
	}
}

func (l *requestLimiter) snapshot() (limit int, inFlight int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.limit, l.inFlight
}
