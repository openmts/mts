package catalog

import (
	"fmt"
	"sort"

	"codeberg.org/mts/mts/internal/model"
)

func (c *Catalog) resolveSeriesLocked(measurement string, tags map[string]string) (Series, error) {
	series, changed, err := c.resolveSeriesNoSnapshotLocked(measurement, tags)
	if err != nil {
		return Series{}, err
	}
	if changed {
		if err := c.saveSnapshotLocked(); err != nil {
			return Series{}, err
		}
	}
	return series, nil
}

func (c *Catalog) resolveSeriesNoSnapshotLocked(
	measurement string,
	tags map[string]string,
) (Series, bool, error) {
	key := seriesKey(measurement, tags)
	if id, ok := c.seriesByKey[key]; ok {
		return c.series[id], false, nil
	}
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
	return series, true, nil
}

func (c *Catalog) resolveFieldsLocked(
	measurement string,
	values map[string]model.FieldValue,
) ([]model.ResolvedField, error) {
	fields, changed, err := c.resolveFieldsNoSnapshotLocked(measurement, values)
	if err != nil {
		return nil, err
	}
	if changed {
		if err := c.saveSnapshotLocked(); err != nil {
			return nil, err
		}
	}
	return fields, nil
}

func (c *Catalog) resolveFieldsNoSnapshotLocked(
	measurement string,
	values map[string]model.FieldValue,
) ([]model.ResolvedField, bool, error) {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)

	fields := make([]model.ResolvedField, 0, len(names))
	changed := false
	for _, name := range names {
		value := values[name]
		field, fieldChanged, err := c.resolveFieldNoSnapshotLocked(measurement, name, value.Type)
		if err != nil {
			return nil, false, err
		}
		changed = changed || fieldChanged
		fields = append(fields, model.ResolvedField{
			FieldID:   field.ID,
			FieldName: field.Name,
			Type:      field.Type,
			Value:     value,
		})
	}
	return fields, changed, nil
}

func (c *Catalog) resolveFieldNoSnapshotLocked(
	measurement string,
	name string,
	fieldType model.FieldType,
) (Field, bool, error) {
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
	return field, true, nil
}

func (c *Catalog) applySeries(series Series) {
	series.Tags = cloneTags(series.Tags)
	c.series[series.ID] = series
	c.seriesByKey[seriesKey(series.Measurement, series.Tags)] = series.ID
	if series.ID >= c.nextSeriesID {
		c.nextSeriesID = series.ID + 1
	}
}

func (c *Catalog) applyField(field Field) {
	c.fields[field.ID] = field
	c.fieldsByKey[fieldKey(field.Measurement, field.Name)] = field.ID
	if field.ID >= c.nextFieldID {
		c.nextFieldID = field.ID + 1
	}
}
