package main

import (
	"net/url"
	"strconv"
	"strings"

	mts "github.com/openmts/mts"
)

// filterDownsampleStatuses 按 Dashboard 查询参数过滤状态列表（可商用扫视）。
// 支持: q, health=error|active|lagging, min_lag_seconds。
func filterDownsampleStatuses(
	statuses []mts.DownsamplePolicyStatus,
	query url.Values,
) []mts.DownsamplePolicyStatus {
	if len(statuses) == 0 || query == nil {
		return statuses
	}
	q := strings.ToLower(strings.TrimSpace(query.Get("q")))
	health := strings.ToLower(strings.TrimSpace(query.Get("health")))
	minLag := 0
	if raw := strings.TrimSpace(query.Get("min_lag_seconds")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			minLag = n
		}
	}
	if q == "" && health == "" && minLag == 0 {
		return statuses
	}
	out := make([]mts.DownsamplePolicyStatus, 0, len(statuses))
	for _, st := range statuses {
		if q != "" {
			name := strings.ToLower(st.PolicyName)
			errText := strings.ToLower(st.LastError)
			if !strings.Contains(name, q) && !strings.Contains(errText, q) {
				continue
			}
		}
		switch health {
		case "error":
			if strings.TrimSpace(st.LastError) == "" {
				continue
			}
		case "active":
			if !st.Active {
				continue
			}
		case "lagging":
			threshold := int64(minLag)
			if threshold < 0 {
				threshold = 0
			}
			if st.LagSeconds <= threshold {
				continue
			}
		default:
			if minLag > 0 && st.LagSeconds <= int64(minLag) {
				continue
			}
		}
		out = append(out, st)
	}
	return out
}
