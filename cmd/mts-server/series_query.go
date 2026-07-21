package main

import (
	"net/http"
	"strconv"
	"strings"

	mts "github.com/openmts/mts"
)

// seriesQueryReserved 为 series 列表查询保留参数，不得当作 tag 过滤键。
var seriesQueryReserved = map[string]struct{}{
	"limit":  {},
	"offset": {},
	"page":   {},
	"q":      {},
}

func queryTags(request *http.Request) map[string]string {
	values := request.URL.Query()
	tags := make(map[string]string)
	for key, value := range values {
		if len(value) == 0 || key == "" {
			continue
		}
		if _, reserved := seriesQueryReserved[strings.ToLower(key)]; reserved {
			continue
		}
		tags[key] = value[0]
	}
	if len(tags) == 0 {
		return nil
	}
	return tags
}

func seriesLimit(request *http.Request) int {
	return queryNonNegativeInt(request, "limit")
}

func seriesOffset(request *http.Request) int {
	return queryNonNegativeInt(request, "offset")
}

func seriesQueryText(request *http.Request) string {
	return strings.TrimSpace(request.URL.Query().Get("q"))
}

func queryNonNegativeInt(request *http.Request, key string) int {
	raw := strings.TrimSpace(request.URL.Query().Get(key))
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

type seriesPageOpts struct {
	Limit  int
	Offset int
	Query  string
}

func seriesPageOptions(request *http.Request) seriesPageOpts {
	return seriesPageOpts{
		Limit:  seriesLimit(request),
		Offset: seriesOffset(request),
		Query:  seriesQueryText(request),
	}
}

func filterSeriesByQuery(series []mts.Series, q string) []mts.Series {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "" {
		return series
	}
	out := make([]mts.Series, 0, len(series))
	for _, item := range series {
		if seriesMatchesQuery(item, q) {
			out = append(out, item)
		}
	}
	return out
}

func seriesMatchesQuery(item mts.Series, q string) bool {
	if strings.Contains(strings.ToLower(item.Measurement), q) {
		return true
	}
	for key, value := range item.Tags {
		if strings.Contains(strings.ToLower(key), q) || strings.Contains(strings.ToLower(value), q) {
			return true
		}
	}
	return false
}

func buildSeriesResponse(series []mts.Series, opts seriesPageOpts) seriesResponse {
	filtered := filterSeriesByQuery(series, opts.Query)
	total := len(filtered)
	offset := opts.Offset
	if offset > total {
		offset = total
	}
	page := filtered[offset:]
	truncated := false
	if opts.Limit > 0 && len(page) > opts.Limit {
		page = page[:opts.Limit]
		truncated = true
	} else if offset+len(page) < total {
		truncated = true
	}
	return seriesResponse{
		Series:    page,
		Total:     total,
		Truncated: truncated,
		Limit:     opts.Limit,
		Offset:    opts.Offset,
	}
}
