package main

import (
	"context"
	"sort"
	"sync"
	"time"

	mts "github.com/openmts/mts"
)

type auditEvent struct {
	Time     time.Time `json:"time"`
	UserName string    `json:"user_name"`
	Action   string    `json:"action"`
	Database string    `json:"database,omitempty"`
	Detail   string    `json:"detail,omitempty"`
}

type auditLog struct {
	mu        sync.Mutex
	limit     int
	events    []auditEvent
	engine    *mts.Engine
	dbCreated bool
	ch        chan auditEvent
}

type userAuditResponse struct {
	Events []auditEvent `json:"events"`
}

func newAuditLog(limit int) *auditLog {
	if limit <= 0 {
		limit = 256
	}
	l := &auditLog{limit: limit, ch: make(chan auditEvent, 1024)}
	go l.persistLoop()
	return l
}

func (l *auditLog) persistLoop() {
	for event := range l.ch {
		_ = l.persistEvent(event)
	}
}

func (l *auditLog) record(event auditEvent) {
	if l == nil {
		return
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	l.mu.Lock()
	l.events = append(l.events, event)
	if len(l.events) > l.limit {
		l.events = append([]auditEvent(nil), l.events[len(l.events)-l.limit:]...)
	}
	l.mu.Unlock()

	select {
	case l.ch <- event:
	default:
		// channel full, drop event to avoid blocking
	}
}

func (l *auditLog) persistEvent(event auditEvent) error {
	if l.engine == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	l.mu.Lock()
	if !l.dbCreated {
		_ = l.engine.CreateDatabase(ctx, "_internal")
		l.dbCreated = true
	}
	l.mu.Unlock()
	fields := map[string]mts.FieldValue{
		"action": mts.StringValue(event.Action),
	}
	if event.Detail != "" {
		fields["detail"] = mts.StringValue(event.Detail)
	}
	if event.Database != "" {
		fields["database"] = mts.StringValue(event.Database)
	}
	tags := map[string]string{}
	if event.UserName != "" {
		tags["user_name"] = event.UserName
	}
	point := mts.Point{
		Database:    "_internal",
		Measurement: "audit_log",
		Tags:        tags,
		Fields:      fields,
		Timestamp:   event.Time.UnixNano(),
	}
	return l.engine.Write(ctx, []mts.Point{point}, mts.WriteOptions{})
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
