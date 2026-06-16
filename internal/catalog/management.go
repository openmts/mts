package catalog

import (
	"fmt"
	"sort"
	"strings"

	"codeberg.org/mts/mts/internal/model"
)

func (c *Catalog) CreateDatabase(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("database name is empty")
	}
	c.databases[name] = struct{}{}
	return c.saveMetadataLocked()
}

func (c *Catalog) DropDatabase(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.databases, name)
	delete(c.policies, name)
	return c.saveMetadataLocked()
}

func (c *Catalog) CreateRetentionPolicy(database string, policy model.RetentionPolicy) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if strings.TrimSpace(database) == "" || strings.TrimSpace(policy.Name) == "" {
		return fmt.Errorf("database and retention policy names are required")
	}
	c.databases[database] = struct{}{}
	if c.policies[database] == nil {
		c.policies[database] = make(map[string]model.RetentionPolicy)
	}
	c.policies[database][policy.Name] = policy
	return c.saveMetadataLocked()
}

func (c *Catalog) ListRetentionPolicies(database string) ([]model.RetentionPolicy, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if _, ok := c.databases[database]; !ok {
		return []model.RetentionPolicy{}, nil
	}
	policies := c.policies[database]
	out := make([]model.RetentionPolicy, 0, len(policies))
	for _, policy := range policies {
		out = append(out, policy)
	}
	sort.Slice(out, func(i int, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Catalog) ListMeasurements(database string) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if _, ok := c.databases[database]; !ok {
		return []string{}, nil
	}
	out := make([]string, 0, len(c.fieldSchemas))
	for measurement := range c.fieldSchemas {
		out = append(out, measurement)
	}
	sort.Strings(out)
	return out, nil
}

func (c *Catalog) ListFields(database string, measurement string) ([]model.FieldSchema, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if _, ok := c.databases[database]; !ok {
		return []model.FieldSchema{}, nil
	}
	fields := c.fieldSchemas[measurement]
	out := make([]model.FieldSchema, 0, len(fields))
	for _, field := range fields {
		out = append(out, model.FieldSchema{
			Measurement: field.Measurement,
			Name:        field.Name,
			Type:        field.Type,
		})
	}
	sort.Slice(out, func(i int, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Catalog) ListSeries(
	database string,
	measurement string,
	tags map[string]string,
) ([]model.Series, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if _, ok := c.databases[database]; !ok {
		return []model.Series{}, nil
	}
	out := make([]model.Series, 0)
	for _, series := range c.series {
		if series.Measurement != measurement || !tagsMatch(series.Tags, tags) {
			continue
		}
		out = append(out, model.Series{
			ID:          series.ID,
			Measurement: series.Measurement,
			Tags:        cloneTags(series.Tags),
		})
	}
	sort.Slice(out, func(i int, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
