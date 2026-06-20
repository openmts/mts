package queryservice

import (
	"context"
	"sync"
	"time"

	"github.com/openmts/mts/internal/model"
)

type Authorizer interface {
	AuthorizeQuery(ctx context.Context, principal Principal, query model.Query) error
}

type staticTenantAuthorizer struct {
	allowed map[string]struct{}
}

func newStaticTenantAuthorizer(tenants []string) Authorizer {
	if len(tenants) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(tenants))
	for _, tenant := range tenants {
		if tenant != "" {
			allowed[tenant] = struct{}{}
		}
	}
	return staticTenantAuthorizer{allowed: allowed}
}

func (a staticTenantAuthorizer) AuthorizeQuery(
	_ context.Context,
	principal Principal,
	_ model.Query,
) error {
	if principal.Tenant == "" {
		return ErrUnauthorized
	}
	if _, ok := a.allowed[principal.Tenant]; !ok {
		return ErrUnauthorized
	}
	return nil
}

type auditLog struct {
	mu      sync.Mutex
	max     int
	records []AuditRecord
	total   int64
}

func newAuditLog(max int) *auditLog {
	if max <= 0 {
		return nil
	}
	return &auditLog{max: max, records: make([]AuditRecord, 0, max)}
}

func (l *auditLog) append(record AuditRecord) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.total++
	if len(l.records) == l.max {
		copy(l.records, l.records[1:])
		l.records[len(l.records)-1] = record
		return
	}
	l.records = append(l.records, record)
}

func (l *auditLog) snapshot() ([]AuditRecord, int64) {
	if l == nil {
		return nil, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]AuditRecord(nil), l.records...), l.total
}

func newAuditRecord(request Request, accepted bool, code ErrorCode, started time.Time) AuditRecord {
	return AuditRecord{
		Tenant:           request.Tenant,
		User:             request.User,
		Measurement:      request.Query.Measurement,
		Accepted:         accepted,
		Code:             code,
		StartedUnixNanos: started.UnixNano(),
		DurationNanos:    time.Since(started).Nanoseconds(),
	}
}
