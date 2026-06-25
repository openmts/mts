package main

import (
	"sort"
	"sync"
	"time"
)

type auditEvent struct {
	Time     time.Time `json:"time"`
	UserName string    `json:"user_name"`
	Action   string    `json:"action"`
	Database string    `json:"database,omitempty"`
	Detail   string    `json:"detail,omitempty"`
}

type auditLog struct {
	mu     sync.Mutex
	limit  int
	events []auditEvent
}

type userAuditResponse struct {
	Events []auditEvent `json:"events"`
}

func newAuditLog(limit int) *auditLog {
	if limit <= 0 {
		limit = 256
	}
	return &auditLog{limit: limit}
}

func (l *auditLog) record(event auditEvent) {
	if l == nil {
		return
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
	if len(l.events) > l.limit {
		l.events = append([]auditEvent(nil), l.events[len(l.events)-l.limit:]...)
	}
}

func (l *auditLog) list(userName string) []auditEvent {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]auditEvent, 0, len(l.events))
	for _, event := range l.events {
		if event.UserName == userName {
			out = append(out, event)
		}
	}
	sort.SliceStable(out, func(i int, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out
}
