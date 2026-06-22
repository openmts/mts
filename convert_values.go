package mts

import "github.com/openmts/mts/internal/model"

func toModelFieldType(fieldType FieldType) model.FieldType {
	return model.FieldType(fieldType)
}

func fromModelFieldType(fieldType model.FieldType) FieldType {
	return FieldType(fieldType)
}

func toModelFieldValue(value FieldValue) model.FieldValue {
	return model.FieldValue{
		Type:    toModelFieldType(value.Type),
		Float64: value.Float64,
		Int64:   value.Int64,
		String:  value.String,
		Bool:    value.Bool,
	}
}

func fromModelFieldValue(value model.FieldValue) FieldValue {
	return FieldValue{
		Type:    fromModelFieldType(value.Type),
		Float64: value.Float64,
		Int64:   value.Int64,
		String:  value.String,
		Bool:    value.Bool,
	}
}

func toModelPoint(point Point) model.Point {
	fields := make(map[string]model.FieldValue, len(point.Fields))
	for name, value := range point.Fields {
		fields[name] = toModelFieldValue(value)
	}
	return model.Point{
		Database:        point.Database,
		RetentionPolicy: point.RetentionPolicy,
		Measurement:     point.Measurement,
		Tags:            cloneStringMap(point.Tags),
		Timestamp:       point.Timestamp,
		Fields:          fields,
	}
}

func toModelPoints(points []Point) []model.Point {
	out := make([]model.Point, len(points))
	for index, point := range points {
		out[index] = toModelPoint(point)
	}
	return out
}

func toModelTypedBatch(batch TypedBatch) model.TypedBatch {
	tags := make([]model.TagColumn, len(batch.Tags))
	for index, tag := range batch.Tags {
		tags[index] = model.TagColumn{
			Name:   tag.Name,
			Values: tag.Values,
		}
	}
	fields := make([]model.TypedFieldColumn, len(batch.Fields))
	for index, field := range batch.Fields {
		fields[index] = model.TypedFieldColumn{
			Name:          field.Name,
			Type:          toModelFieldType(field.Type),
			Float64Values: field.Float64Values,
			Int64Values:   field.Int64Values,
			StringValues:  field.StringValues,
			BoolValues:    field.BoolValues,
		}
	}
	return model.TypedBatch{
		Database:        batch.Database,
		RetentionPolicy: batch.RetentionPolicy,
		Measurement:     batch.Measurement,
		Tags:            tags,
		Timestamps:      batch.Timestamps,
		Fields:          fields,
	}
}

func toModelWriteOptions(opts WriteOptions) model.WriteOptions {
	return model.WriteOptions{Sync: opts.Sync}
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
