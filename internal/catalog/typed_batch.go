package catalog

import (
	"fmt"
	"sort"
	"strings"

	"github.com/openmts/mts/internal/model"
)

func (c *Catalog) ResolveTypedBatch(batch model.TypedBatch) ([]model.ResolvedPoint, error) {
	resolved, err := c.ResolveTypedBatchColumns(batch)
	if err != nil {
		return nil, err
	}
	return resolvedPointsFromTypedColumns(resolved), nil
}

func (c *Catalog) ResolveTypedBatchColumns(batch model.TypedBatch) (model.ResolvedTypedBatch, error) {
	rows := len(batch.Timestamps)
	if rows == 0 {
		return model.ResolvedTypedBatch{}, nil
	}
	if err := validateTypedBatch(batch, rows); err != nil {
		return model.ResolvedTypedBatch{}, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	metadataChanged := c.ensureMetadataLocked(batch.Database, batch.RetentionPolicy)
	fieldDefs, fieldsChanged, err := c.resolveTypedFieldDefinitionsLocked(batch.Measurement, batch.Fields)
	if err != nil {
		return model.ResolvedTypedBatch{}, err
	}
	seriesIDs, seriesChanged, err := c.resolveTypedSeriesIDsLocked(batch, rows)
	if err != nil {
		return model.ResolvedTypedBatch{}, err
	}
	if fieldsChanged || seriesChanged {
		if err := c.checkpointSnapshotLocked(false); err != nil {
			return model.ResolvedTypedBatch{}, err
		}
	}
	if metadataChanged {
		if err := c.saveMetadataLocked(); err != nil {
			return model.ResolvedTypedBatch{}, err
		}
	}
	return model.ResolvedTypedBatch{
		Database:        batch.Database,
		RetentionPolicy: batch.RetentionPolicy,
		Measurement:     batch.Measurement,
		Tags:            batch.Tags,
		Timestamps:      batch.Timestamps,
		SeriesIDs:       seriesIDs,
		Fields:          resolvedTypedFieldColumns(fieldDefs, batch.Fields),
	}, nil
}

func (c *Catalog) resolveTypedSeriesIDsLocked(
	batch model.TypedBatch,
	rows int,
) ([]uint64, bool, error) {
	order := sortedTagColumnOrder(batch.Tags)
	seriesIDs := make([]uint64, rows)
	seriesChanged := false
	keyScratch := make([]byte, 0)
	for row := range rows {
		var series Series
		var changed bool
		var err error
		series, changed, keyScratch, err = c.resolveSeriesFromTypedTagsLocked(
			batch.Measurement,
			batch.Tags,
			row,
			order,
			keyScratch,
		)
		if err != nil {
			return nil, false, err
		}
		seriesChanged = seriesChanged || changed
		seriesIDs[row] = series.ID
	}
	return seriesIDs, seriesChanged, nil
}

func resolvedTypedFieldColumns(
	defs []Field,
	columns []model.TypedFieldColumn,
) []model.ResolvedTypedFieldColumn {
	fields := make([]model.ResolvedTypedFieldColumn, len(columns))
	for index, column := range columns {
		fields[index] = model.ResolvedTypedFieldColumn{
			FieldID:       defs[index].ID,
			Name:          defs[index].Name,
			Type:          defs[index].Type,
			Float64Values: column.Float64Values,
			Int64Values:   column.Int64Values,
			StringValues:  column.StringValues,
			BoolValues:    column.BoolValues,
		}
	}
	return fields
}

func resolvedPointsFromTypedColumns(batch model.ResolvedTypedBatch) []model.ResolvedPoint {
	rows := len(batch.Timestamps)
	points := make([]model.ResolvedPoint, rows)
	fieldArena := make([]model.ResolvedField, rows*len(batch.Fields))
	offset := 0
	for row := range rows {
		fields := fieldArena[offset : offset+len(batch.Fields)]
		offset += len(batch.Fields)
		for index, column := range batch.Fields {
			fields[index] = resolvedFieldAt(column, row)
		}
		points[row] = resolvedPointAt(batch, row, fields)
	}
	return points
}

func resolvedPointAt(
	batch model.ResolvedTypedBatch,
	row int,
	fields []model.ResolvedField,
) model.ResolvedPoint {
	return model.ResolvedPoint{
		Database:        batch.Database,
		RetentionPolicy: batch.RetentionPolicy,
		Measurement:     batch.Measurement,
		Tags:            tagMapFromColumns(batch.Tags, row),
		SeriesID:        batch.SeriesIDs[row],
		Timestamp:       batch.Timestamps[row],
		Fields:          fields,
	}
}

func resolvedFieldAt(column model.ResolvedTypedFieldColumn, row int) model.ResolvedField {
	return model.ResolvedField{
		FieldID:   column.FieldID,
		FieldName: column.Name,
		Type:      column.Type,
		Value:     resolvedTypedFieldValueAt(column, row),
	}
}

func validateTypedBatch(batch model.TypedBatch, rows int) error {
	if strings.TrimSpace(batch.Measurement) == "" {
		return ErrEmptyMeasurement
	}
	if len(batch.Fields) == 0 {
		return ErrEmptyFields
	}
	if err := validateTypedTags(batch.Tags, rows); err != nil {
		return err
	}
	return validateTypedFields(batch.Fields, rows)
}

func validateTypedTags(tags []model.TagColumn, rows int) error {
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		if strings.TrimSpace(tag.Name) == "" {
			return fmt.Errorf("tag name is empty")
		}
		if _, ok := seen[tag.Name]; ok {
			return fmt.Errorf("duplicate tag column %q", tag.Name)
		}
		seen[tag.Name] = struct{}{}
		if len(tag.Values) != rows {
			return fmt.Errorf("tag column %s length=%d want %d", tag.Name, len(tag.Values), rows)
		}
	}
	return nil
}

func validateTypedFields(fields []model.TypedFieldColumn, rows int) error {
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if strings.TrimSpace(field.Name) == "" {
			return fmt.Errorf("field name is empty")
		}
		if _, ok := seen[field.Name]; ok {
			return fmt.Errorf("duplicate field column %q", field.Name)
		}
		seen[field.Name] = struct{}{}
		length := typedFieldColumnLen(field)
		if length < 0 {
			return fmt.Errorf("unsupported typed field %s type=%d", field.Name, field.Type)
		}
		if length != rows {
			return fmt.Errorf("field column %s length=%d want %d", field.Name, length, rows)
		}
	}
	return nil
}

func typedFieldColumnLen(field model.TypedFieldColumn) int {
	switch field.Type {
	case model.FieldFloat64:
		return len(field.Float64Values)
	case model.FieldInt64:
		return len(field.Int64Values)
	case model.FieldString:
		return len(field.StringValues)
	case model.FieldBool:
		return len(field.BoolValues)
	default:
		return -1
	}
}

func (c *Catalog) resolveTypedFieldDefinitionsLocked(
	measurement string,
	columns []model.TypedFieldColumn,
) ([]Field, bool, error) {
	fields := make([]Field, len(columns))
	changed := false
	for index, column := range columns {
		field, fieldChanged, err := c.resolveFieldNoSnapshotLocked(measurement, column.Name, column.Type)
		if err != nil {
			return nil, false, err
		}
		fields[index] = field
		changed = changed || fieldChanged
	}
	return fields, changed, nil
}

func sortedTagColumnOrder(tags []model.TagColumn) []int {
	if len(tags) <= 1 {
		return nil
	}
	order := make([]int, len(tags))
	for index := range tags {
		order[index] = index
	}
	sort.Slice(order, func(i int, j int) bool {
		return tags[order[i]].Name < tags[order[j]].Name
	})
	return order
}

func (c *Catalog) resolveSeriesFromTypedTagsLocked(
	measurement string,
	tags []model.TagColumn,
	row int,
	order []int,
	keyScratch []byte,
) (Series, bool, []byte, error) {
	switch len(tags) {
	case 0:
		series, changed, err := c.resolveSeriesNoSnapshotLocked(measurement, nil)
		return series, changed, keyScratch, err
	case 1:
		series, changed, err := c.resolveSingleTagSeriesLocked(measurement, tags[0], row)
		return series, changed, keyScratch, err
	default:
		key, scratch := seriesKeyFromTagColumns(measurement, tags, row, order, keyScratch)
		if id, ok := c.seriesByKey[key]; ok {
			return c.series[id], false, scratch, nil
		}
		series, changed, err := c.createSeriesNoSnapshotLocked(measurement, tagMapFromColumns(tags, row))
		return series, changed, scratch, err
	}
}

func (c *Catalog) resolveSingleTagSeriesLocked(
	measurement string,
	tag model.TagColumn,
	row int,
) (Series, bool, error) {
	valuesByTagKey := c.seriesByTag[measurement]
	if valuesByTagKey != nil {
		if values := valuesByTagKey[tag.Name]; values != nil {
			if id, ok := values[tag.Values[row]]; ok {
				return c.series[id], false, nil
			}
		}
	}
	return c.createSeriesNoSnapshotLocked(measurement, map[string]string{tag.Name: tag.Values[row]})
}

func seriesKeyFromTagColumns(
	measurement string,
	tags []model.TagColumn,
	row int,
	order []int,
	dst []byte,
) (string, []byte) {
	if len(tags) == 0 {
		return measurement, dst
	}
	if len(tags) == 1 {
		tag := tags[0]
		return measurement + "\xff" + tag.Name + "=" + tag.Values[row], dst
	}
	size := len(measurement)
	for _, index := range order {
		tag := tags[index]
		size += 1 + len(tag.Name) + 1 + len(tag.Values[row])
	}
	if cap(dst) < size {
		dst = make([]byte, 0, size)
	}
	dst = dst[:0]
	dst = append(dst, measurement...)
	for _, index := range order {
		tag := tags[index]
		dst = append(dst, '\xff')
		dst = append(dst, tag.Name...)
		dst = append(dst, '=')
		dst = append(dst, tag.Values[row]...)
	}
	return string(dst), dst
}

func tagMapFromColumns(tags []model.TagColumn, row int) map[string]string {
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		out[tag.Name] = tag.Values[row]
	}
	return out
}

func resolvedTypedFieldValueAt(column model.ResolvedTypedFieldColumn, row int) model.FieldValue {
	switch column.Type {
	case model.FieldFloat64:
		return model.Float64Value(column.Float64Values[row])
	case model.FieldInt64:
		return model.Int64Value(column.Int64Values[row])
	case model.FieldString:
		return model.StringValue(column.StringValues[row])
	case model.FieldBool:
		return model.BoolValue(column.BoolValues[row])
	default:
		return model.FieldValue{Type: column.Type}
	}
}
