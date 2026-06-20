package queryservice

import (
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryexec"
)

type Request struct {
	Query    model.Query   `json:"query"`
	Timeout  time.Duration `json:"timeout,omitempty"`
	Priority Priority      `json:"priority,omitempty"`
	Tenant   string        `json:"tenant,omitempty"`
	User     string        `json:"user,omitempty"`
}

type Priority uint8

const (
	PriorityNormal Priority = iota
	PriorityLow
	PriorityHigh
)

type Result struct {
	Columns           []model.ColumnSeries
	Rows              []model.Row
	Explain           model.QueryExplain
	Stats             model.QueryStats
	Profile           queryexec.Profile
	LogicalPlanRoot   string
	PhysicalOperators []string
	Pushdowns         []string
}

type ServiceStats struct {
	Active            int64 `json:"active"`
	Queued            int64 `json:"queued"`
	TotalAdmitted     int64 `json:"total_admitted"`
	TotalQueued       int64 `json:"total_queued"`
	TotalRejected     int64 `json:"total_rejected"`
	TotalTimedOut     int64 `json:"total_timed_out"`
	TotalUnauthorized int64 `json:"total_unauthorized"`
	TotalCacheHits    int64 `json:"total_cache_hits"`
	TotalCacheMisses  int64 `json:"total_cache_misses"`
	TotalAuditRecords int64 `json:"total_audit_records"`
}

type Principal struct {
	Tenant string `json:"tenant"`
	User   string `json:"user"`
}

type AuditRecord struct {
	Tenant           string    `json:"tenant,omitempty"`
	User             string    `json:"user,omitempty"`
	Measurement      string    `json:"measurement"`
	Accepted         bool      `json:"accepted"`
	Code             ErrorCode `json:"code,omitempty"`
	StartedUnixNanos int64     `json:"started_unix_nanos"`
	DurationNanos    int64     `json:"duration_nanos"`
}
