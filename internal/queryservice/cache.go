package queryservice

import (
	"encoding/json"
	"sync"

	"github.com/openmts/mts/internal/model"
)

type resultCache struct {
	mu      sync.Mutex
	max     int
	order   []string
	entries map[string]Result
}

func newResultCache(max int) *resultCache {
	if max <= 0 {
		return nil
	}
	return &resultCache{
		max:     max,
		order:   make([]string, 0, max),
		entries: make(map[string]Result, max),
	}
}

func (c *resultCache) get(key string) (Result, bool) {
	if c == nil {
		return Result{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	result, ok := c.entries[key]
	if !ok {
		return Result{}, false
	}
	return cloneResult(result), true
}

func (c *resultCache) set(key string, result Result) {
	if c == nil || c.max <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.entries[key]; !ok {
		c.order = append(c.order, key)
	}
	c.entries[key] = cloneResult(result)
	for len(c.order) > c.max {
		evicted := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, evicted)
	}
}

func (c *resultCache) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.entries)
	c.order = c.order[:0]
}

func cacheKey(request Request) (string, bool) {
	if request.Query.Cursor != "" {
		return "", false
	}
	payload := struct {
		Tenant string      `json:"tenant,omitempty"`
		Query  model.Query `json:"query"`
	}{
		Tenant: request.Tenant,
		Query:  request.Query,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func cloneResult(result Result) Result {
	out := result
	out.Columns = cloneColumns(result.Columns)
	out.Rows = cloneRows(result.Rows)
	out.Pushdowns = append([]string(nil), result.Pushdowns...)
	out.PhysicalOperators = append([]string(nil), result.PhysicalOperators...)
	return out
}

func cloneColumns(columns []model.ColumnSeries) []model.ColumnSeries {
	if len(columns) == 0 {
		return nil
	}
	out := make([]model.ColumnSeries, 0, len(columns))
	for _, column := range columns {
		cloned := column
		cloned.Tags = cloneStringMap(column.Tags)
		cloned.Timestamps = append([]int64(nil), column.Timestamps...)
		cloned.Values = append([]model.FieldValue(nil), column.Values...)
		out = append(out, cloned)
	}
	return out
}

func cloneRows(rows []model.Row) []model.Row {
	if len(rows) == 0 {
		return nil
	}
	out := make([]model.Row, 0, len(rows))
	for _, row := range rows {
		cloned := row
		cloned.Tags = cloneStringMap(row.Tags)
		cloned.Fields = cloneFieldMap(row.Fields)
		out = append(out, cloned)
	}
	return out
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func cloneFieldMap(values map[string]model.FieldValue) map[string]model.FieldValue {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]model.FieldValue, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
