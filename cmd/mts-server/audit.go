package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
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
	mu         sync.Mutex
	limit      int
	events     []auditEvent
	engine     *mts.Engine
	dbCreated  bool
	ch         chan auditEvent
	closed     bool
	done       chan struct{}
	persistCtx context.Context
	cancel     context.CancelFunc
	dropped    uint64
	persistErr uint64
	lastError  string
}

type userAuditResponse struct {
	Events []auditEvent `json:"events"`
	Path   string       `json:"path,omitempty"`
}

type auditListRequest struct {
	UserName string `json:"user_name,omitempty"`
	Action   string `json:"action,omitempty"`
	// SinceUnix / UntilUnix 为 Unix 秒；0 表示不限制
	SinceUnix int64 `json:"since_unix,omitempty"`
	UntilUnix int64 `json:"until_unix,omitempty"`
	Limit     int   `json:"limit,omitempty"`
}

type auditListResponse struct {
	Events        []auditEvent          `json:"events"`
	Total         int                   `json:"total"`
	Path          string                `json:"path,omitempty"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

func newAuditLog(limit int) *auditLog {
	if limit <= 0 {
		limit = 256
	}
	persistCtx, cancel := context.WithCancel(context.Background())
	l := &auditLog{
		limit:      limit,
		ch:         make(chan auditEvent, 1024),
		done:       make(chan struct{}),
		persistCtx: persistCtx,
		cancel:     cancel,
	}
	go l.persistLoop()
	return l
}

func (l *auditLog) persistLoop() {
	defer close(l.done)
	for event := range l.ch {
		if err := l.persistEvent(event); err != nil {
			l.mu.Lock()
			l.persistErr++
			l.lastError = err.Error()
			l.mu.Unlock()
		}
	}
}

// close 停止接收新事件，并等待已接收事件完成持久化。
func (l *auditLog) close(ctx context.Context) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	if l.closed {
		done := l.done
		l.mu.Unlock()
		return l.waitClosed(ctx, done)
	}
	l.closed = true
	close(l.ch)
	done := l.done
	l.mu.Unlock()
	return l.waitClosed(ctx, done)
}

func (l *auditLog) waitClosed(ctx context.Context, done <-chan struct{}) error {
	var waitErr error
	select {
	case <-done:
	case <-ctx.Done():
		waitErr = ctx.Err()
		l.cancel()
		<-done
	}
	l.cancel()
	l.mu.Lock()
	l.engine = nil
	dropped := l.dropped
	persistErr := l.persistErr
	lastError := l.lastError
	l.mu.Unlock()
	return errors.Join(waitErr, auditCloseStatsError(dropped, persistErr, lastError))
}

func auditCloseStatsError(dropped uint64, persistErr uint64, lastError string) error {
	if dropped == 0 && persistErr == 0 {
		return nil
	}
	return fmt.Errorf(
		"audit close: dropped=%d persist_errors=%d last_error=%q",
		dropped,
		persistErr,
		lastError,
	)
}

func (l *auditLog) record(event auditEvent) {
	if l == nil {
		return
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	l.mu.Lock()
	if l.closed {
		l.dropped++
		l.mu.Unlock()
		return
	}
	l.events = append(l.events, event)
	if len(l.events) > l.limit {
		l.events = append([]auditEvent(nil), l.events[len(l.events)-l.limit:]...)
	}
	select {
	case l.ch <- event:
	default:
		l.dropped++
	}
	l.mu.Unlock()
}

func (l *auditLog) persistEvent(event auditEvent) error {
	l.mu.Lock()
	eng := l.engine
	dbCreated := l.dbCreated
	persistCtx := l.persistCtx
	l.mu.Unlock()
	if eng == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(persistCtx, 5*time.Second)
	defer cancel()
	if !dbCreated {
		if err := eng.CreateDatabase(ctx, "_internal"); err != nil {
			return fmt.Errorf("create audit database: %w", err)
		}
		l.mu.Lock()
		l.dbCreated = true
		l.mu.Unlock()
	}
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
	return eng.Write(ctx, []mts.Point{point}, mts.WriteOptions{})
}

func (l *auditLog) list(userName string) []auditEvent {
	return l.listFiltered(auditListRequest{UserName: userName})
}

func (l *auditLog) loadPersisted(req auditListRequest) []auditEvent {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	eng := l.engine
	l.mu.Unlock()
	if eng == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	limit := req.Limit
	if limit <= 0 {
		limit = l.limit
		if limit <= 0 {
			limit = 256
		}
	}
	// 多取一些，便于再过滤后仍够 limit
	queryLimit := limit * 4
	if queryLimit < 64 {
		queryLimit = 64
	}
	if queryLimit > 2000 {
		queryLimit = 2000
	}
	q := mts.Query{
		Database:    "_internal",
		Measurement: "audit_log",
		Precision:   mts.PrecisionNanosecond,
		Limit:       queryLimit,
		Order:       mts.QueryOrder{By: mts.QueryOrderByTime, Direction: mts.QuerySortDesc},
	}
	if req.UserName != "" {
		q.Tags = map[string]string{"user_name": req.UserName}
	}
	if req.SinceUnix > 0 {
		q.StartTime = req.SinceUnix * int64(time.Second)
	}
	if req.UntilUnix > 0 {
		q.EndTime = req.UntilUnix * int64(time.Second)
	}
	rows, err := eng.QueryRows(ctx, q)
	if err != nil {
		return nil
	}
	out := make([]auditEvent, 0, len(rows))
	for _, row := range rows {
		ev := auditEvent{
			Time:     time.Unix(0, row.Timestamp).UTC(),
			UserName: row.Tags["user_name"],
		}
		if v, ok := row.Fields["action"]; ok {
			ev.Action = fieldString(v)
		}
		if v, ok := row.Fields["detail"]; ok {
			ev.Detail = fieldString(v)
		}
		if v, ok := row.Fields["database"]; ok {
			ev.Database = fieldString(v)
		}
		if req.Action != "" && !strings.Contains(ev.Action, req.Action) {
			continue
		}
		out = append(out, ev)
	}
	return out
}

func fieldString(v mts.FieldValue) string {
	switch v.Type {
	case mts.FieldString:
		return v.String
	case mts.FieldInt64:
		return fmt.Sprintf("%d", v.Int64)
	case mts.FieldFloat64:
		return fmt.Sprintf("%g", v.Float64)
	case mts.FieldBool:
		if v.Bool {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func mergeAuditEvents(memory, persisted []auditEvent, limit int) []auditEvent {
	// 以 time+user+action+detail 去重，优先 memory 最新视图
	type key struct {
		t int64
		u string
		a string
		d string
	}
	seen := make(map[key]struct{}, len(memory)+len(persisted))
	out := make([]auditEvent, 0, len(memory)+len(persisted))
	add := func(ev auditEvent) {
		k := key{t: ev.Time.UnixNano(), u: ev.UserName, a: ev.Action, d: ev.Detail}
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		out = append(out, ev)
	}
	for _, ev := range memory {
		add(ev)
	}
	for _, ev := range persisted {
		add(ev)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	if limit > 0 && len(out) > limit {
		out = append([]auditEvent(nil), out[:limit]...)
	}
	return out
}

func (l *auditLog) listFiltered(req auditListRequest) []auditEvent {
	if l == nil {
		return nil
	}
	userName := strings.TrimSpace(req.UserName)
	action := strings.TrimSpace(req.Action)
	var since, until time.Time
	if req.SinceUnix > 0 {
		since = time.Unix(req.SinceUnix, 0).UTC()
	}
	if req.UntilUnix > 0 {
		until = time.Unix(req.UntilUnix, 0).UTC()
	}
	l.mu.Lock()
	memory := make([]auditEvent, 0, len(l.events))
	for _, event := range l.events {
		if userName != "" && event.UserName != userName {
			continue
		}
		if action != "" && !strings.Contains(event.Action, action) {
			continue
		}
		if !since.IsZero() && event.Time.Before(since) {
			continue
		}
		if !until.IsZero() && event.Time.After(until) {
			continue
		}
		memory = append(memory, event)
	}
	l.mu.Unlock()

	persisted := l.loadPersisted(req)
	limit := req.Limit
	if limit <= 0 {
		limit = l.limit
	}
	return mergeAuditEvents(memory, persisted, limit)
}
