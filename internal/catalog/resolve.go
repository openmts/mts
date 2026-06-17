package catalog

import (
	"fmt"
	"sort"

	"codeberg.org/mts/mts/internal/model"
)

type resolvedFieldArena struct {
	fields []model.ResolvedField
	offset int
}

type resolveBatchCache struct {
	series  map[string]Series
	tagKeys []string
}

func makeResolvedFields(count int, arena *resolvedFieldArena) []model.ResolvedField {
	if arena == nil {
		return make([]model.ResolvedField, count)
	}
	start := arena.offset
	arena.offset += count
	return arena.fields[start:arena.offset]
}

func (c *Catalog) resolveSeriesLocked(measurement string, tags map[string]string) (Series, error) {
	series, changed, err := c.resolveSeriesNoSnapshotLocked(measurement, tags)
	if err != nil {
		return Series{}, err
	}
	if changed {
		if err := c.checkpointSnapshotLocked(false); err != nil {
			return Series{}, err
		}
	}
	return series, nil
}

func (c *Catalog) resolveSeriesNoSnapshotLocked(
	measurement string,
	tags map[string]string,
) (Series, bool, error) {
	return c.resolveSeriesNoSnapshotCachedLocked(measurement, tags, nil)
}

func (c *Catalog) resolveSeriesNoSnapshotCachedLocked(
	measurement string,
	tags map[string]string,
	cache *resolveBatchCache,
) (Series, bool, error) {
	if cache != nil && len(tags) > 1 {
		var key string
		key, cache.tagKeys = seriesKeyWithScratch(measurement, tags, cache.tagKeys)
		if cache.series == nil {
			cache.series = make(map[string]Series)
		}
		if series, ok := cache.series[key]; ok {
			return series, false, nil
		}
		if id, ok := c.seriesByKey[key]; ok {
			series := c.series[id]
			cache.series[key] = series
			return series, false, nil
		}
		series, changed, err := c.createSeriesNoSnapshotLocked(measurement, tags)
		if err != nil {
			return Series{}, false, err
		}
		cache.series[key] = series
		return series, changed, nil
	}
	if id, ok := c.lookupSeriesID(measurement, tags); ok {
		return c.series[id], false, nil
	}
	var key string
	if len(tags) > 1 {
		key, c.seriesKeyScratch = seriesKeyWithScratch(measurement, tags, c.seriesKeyScratch)
	} else {
		key = seriesKey(measurement, tags)
	}
	if id, ok := c.seriesByKey[key]; ok {
		return c.series[id], false, nil
	}
	return c.createSeriesNoSnapshotLocked(measurement, tags)
}

func (c *Catalog) createSeriesNoSnapshotLocked(
	measurement string,
	tags map[string]string,
) (Series, bool, error) {
	series := Series{
		ID:          c.nextSeriesID,
		Measurement: measurement,
		Tags:        cloneTags(tags),
	}
	entry := walEntry{
		Type:   "series",
		Series: &series,
	}
	if err := c.appendEntryLocked(entry); err != nil {
		return Series{}, false, err
	}
	c.applySeries(series)
	c.nextSeriesID++
	c.snapshotDirtyRecords++
	return series, true, nil
}

func (c *Catalog) lookupSeriesID(measurement string, tags map[string]string) (uint64, bool) {
	if len(tags) == 0 {
		id, ok := c.seriesByKey[measurement]
		return id, ok
	}
	if len(tags) != 1 {
		return 0, false
	}
	for key, value := range tags {
		valuesByTagKey := c.seriesByTag[measurement]
		if valuesByTagKey == nil {
			return 0, false
		}
		values := valuesByTagKey[key]
		if values == nil {
			return 0, false
		}
		id, ok := values[value]
		return id, ok
	}
	return 0, false
}

func (c *Catalog) upsertSingleTagSeries(series Series) {
	if len(series.Tags) != 1 {
		return
	}
	for key, value := range series.Tags {
		valuesByTagKey := c.seriesByTag[series.Measurement]
		if valuesByTagKey == nil {
			valuesByTagKey = make(map[string]map[string]uint64, 1)
			c.seriesByTag[series.Measurement] = valuesByTagKey
		}
		values := valuesByTagKey[key]
		if values == nil {
			values = make(map[string]uint64, 1)
			valuesByTagKey[key] = values
		}
		values[value] = series.ID
		return
	}
}

func (c *Catalog) resolveFieldsLocked(
	measurement string,
	values map[string]model.FieldValue,
) ([]model.ResolvedField, error) {
	fields, changed, err := c.resolveFieldsNoSnapshotLocked(measurement, values, nil)
	if err != nil {
		return nil, err
	}
	if changed {
		if err := c.checkpointSnapshotLocked(false); err != nil {
			return nil, err
		}
	}
	return fields, nil
}

func (c *Catalog) resolveFieldsNoSnapshotLocked(
	measurement string,
	values map[string]model.FieldValue,
	arena *resolvedFieldArena,
) ([]model.ResolvedField, bool, error) {
	if fields, ok, err := c.resolveFieldsFromSchema(measurement, values, arena); ok || err != nil {
		return fields, false, err
	}

	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)

	fields := makeResolvedFields(len(names), arena)
	changed := false
	for index, name := range names {
		value := values[name]
		field, fieldChanged, err := c.resolveFieldNoSnapshotLocked(measurement, name, value.Type)
		if err != nil {
			return nil, false, err
		}
		changed = changed || fieldChanged
		fields[index] = model.ResolvedField{
			FieldID:   field.ID,
			FieldName: field.Name,
			Type:      field.Type,
			Value:     value,
		}
	}
	return fields, changed, nil
}

func (c *Catalog) resolveFieldsFromSchema(
	measurement string,
	values map[string]model.FieldValue,
	arena *resolvedFieldArena,
) ([]model.ResolvedField, bool, error) {
	schema := c.fieldSchemas[measurement]
	if len(schema) == 0 || len(schema) != len(values) {
		return nil, false, nil
	}
	for _, field := range schema {
		value, ok := values[field.Name]
		if !ok {
			return nil, false, nil
		}
		if field.Type != value.Type {
			return nil, true, fmt.Errorf("%w: %s", ErrFieldTypeConflict, field.Name)
		}
	}
	fields := makeResolvedFields(len(schema), arena)
	for index, field := range schema {
		value := values[field.Name]
		fields[index] = model.ResolvedField{
			FieldID:   field.ID,
			FieldName: field.Name,
			Type:      field.Type,
			Value:     value,
		}
	}
	return fields, true, nil
}

func (c *Catalog) resolveFieldNoSnapshotLocked(
	measurement string,
	name string,
	fieldType model.FieldType,
) (Field, bool, error) {
	if field, ok := c.schemaField(measurement, name); ok {
		if field.Type != fieldType {
			return Field{}, false, fmt.Errorf("%w: %s", ErrFieldTypeConflict, name)
		}
		return field, false, nil
	}
	key := fieldKey(measurement, name)
	if id, ok := c.fieldsByKey[key]; ok {
		field := c.fields[id]
		if field.Type != fieldType {
			return Field{}, false, fmt.Errorf("%w: %s", ErrFieldTypeConflict, name)
		}
		return field, false, nil
	}
	field := Field{
		ID:          c.nextFieldID,
		Measurement: measurement,
		Name:        name,
		Type:        fieldType,
	}
	entry := walEntry{
		Type:  "field",
		Field: &field,
	}
	if err := c.appendEntryLocked(entry); err != nil {
		return Field{}, false, err
	}
	c.applyField(field)
	c.nextFieldID++
	c.snapshotDirtyRecords++
	return field, true, nil
}

func (c *Catalog) schemaField(measurement string, name string) (Field, bool) {
	schema := c.fieldSchemas[measurement]
	index := sort.Search(len(schema), func(index int) bool {
		return schema[index].Name >= name
	})
	if index >= len(schema) || schema[index].Name != name {
		return Field{}, false
	}
	return schema[index], true
}

func (c *Catalog) applySeries(series Series) {
	series.Tags = cloneTags(series.Tags)
	c.series[series.ID] = series
	c.seriesByKey[seriesKey(series.Measurement, series.Tags)] = series.ID
	c.upsertSingleTagSeries(series)
	if series.ID >= c.nextSeriesID {
		c.nextSeriesID = series.ID + 1
	}
}

func (c *Catalog) applyField(field Field) {
	c.fields[field.ID] = field
	c.fieldsByKey[fieldKey(field.Measurement, field.Name)] = field.ID
	c.upsertFieldSchema(field)
	if field.ID >= c.nextFieldID {
		c.nextFieldID = field.ID + 1
	}
}

func (c *Catalog) upsertFieldSchema(field Field) {
	schema := c.fieldSchemas[field.Measurement]
	for index, existing := range schema {
		if existing.Name == field.Name {
			schema[index] = field
			c.fieldSchemas[field.Measurement] = schema
			return
		}
		if existing.Name > field.Name {
			schema = append(schema, Field{})
			copy(schema[index+1:], schema[index:])
			schema[index] = field
			c.fieldSchemas[field.Measurement] = schema
			return
		}
	}
	c.fieldSchemas[field.Measurement] = append(schema, field)
}
