package catalog

import "fmt"

func (c *Catalog) recordCardinalityRejectLocked() {
	c.cardinalityRejected++
}

// CardinalityRejected 返回因基数限制拒绝创建 series/field 的次数。
func (c *Catalog) CardinalityRejected() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cardinalityRejected
}

func (c *Catalog) ensureSeriesCardinalityLocked(measurement string, tags map[string]string) error {
	if c.limits.MaxSeries > 0 && len(c.series) >= c.limits.MaxSeries {
		c.recordCardinalityRejectLocked()
		return fmt.Errorf("%w: max series %d", ErrCardinalityLimit, c.limits.MaxSeries)
	}
	if c.limits.MaxTagValuesPerKey <= 0 {
		return nil
	}
	for key, value := range tags {
		values := c.tagValues[measurement]
		if values == nil {
			continue
		}
		keyValues := values[key]
		if keyValues == nil {
			continue
		}
		if _, exists := keyValues[value]; exists {
			continue
		}
		if len(keyValues) >= c.limits.MaxTagValuesPerKey {
			c.recordCardinalityRejectLocked()
			return fmt.Errorf(
				"%w: max tag values per key %d (measurement=%s tag=%s)",
				ErrCardinalityLimit,
				c.limits.MaxTagValuesPerKey,
				measurement,
				key,
			)
		}
	}
	return nil
}

func (c *Catalog) ensureFieldCardinalityLocked() error {
	if c.limits.MaxFields > 0 && len(c.fields) >= c.limits.MaxFields {
		c.recordCardinalityRejectLocked()
		return fmt.Errorf("%w: max fields %d", ErrCardinalityLimit, c.limits.MaxFields)
	}
	return nil
}

func (c *Catalog) recordTagValues(measurement string, tags map[string]string) {
	if len(tags) == 0 {
		return
	}
	valuesByKey := c.tagValues[measurement]
	if valuesByKey == nil {
		valuesByKey = make(map[string]map[string]struct{}, len(tags))
		c.tagValues[measurement] = valuesByKey
	}
	for key, value := range tags {
		values := valuesByKey[key]
		if values == nil {
			values = make(map[string]struct{}, 1)
			valuesByKey[key] = values
		}
		values[value] = struct{}{}
	}
}
