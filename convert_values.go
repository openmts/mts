package mts

import (
	"github.com/openmts/mts/internal/collections"
	"github.com/openmts/mts/internal/model"
)

func toModelFieldType(fieldType FieldType) model.FieldType {
	return model.FieldType(fieldType)
}

func fromModelFieldType(fieldType model.FieldType) FieldType {
	return FieldType(fieldType)
}

func toModelFieldValue(value FieldValue) model.FieldValue {
	// 与 model.FieldValue 内存布局一致；按字段拷贝保持类型安全。
	return model.FieldValue{
		Type:    model.FieldType(value.Type),
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

func toModelPoint(point Point) (model.Point, error) {
	timestamp := point.Timestamp
	if point.Precision != "" && point.Precision != PrecisionNanosecond {
		factor, err := timePrecisionFactor(point.Precision)
		if err != nil {
			return model.Point{}, err
		}
		timestamp, err = timestampToNanoseconds(point.Timestamp, factor)
		if err != nil {
			return model.Point{}, err
		}
	}
	fields := make(map[string]model.FieldValue, len(point.Fields))
	for name, value := range point.Fields {
		fields[name] = toModelFieldValue(value)
	}
	return model.Point{
		Database:        point.Database,
		RetentionPolicy: point.RetentionPolicy,
		Measurement:     point.Measurement,
		Tags:            cloneStringMap(point.Tags),
		Timestamp:       timestamp,
		Fields:          fields,
	}, nil
}

func toModelPoints(points []Point) ([]model.Point, error) {
	if len(points) == 0 {
		return nil, nil
	}
	out := make([]model.Point, len(points))
	// 热路径：整批同为默认纳秒精度时跳过逐点 precision 分支。
	uniformNanos := true
	for _, point := range points {
		if point.Precision != "" && point.Precision != PrecisionNanosecond {
			uniformNanos = false
			break
		}
	}
	if uniformNanos {
		for index, point := range points {
			fields := make(map[string]model.FieldValue, len(point.Fields))
			for name, value := range point.Fields {
				fields[name] = toModelFieldValue(value)
			}
			out[index] = model.Point{
				Database:        point.Database,
				RetentionPolicy: point.RetentionPolicy,
				Measurement:     point.Measurement,
				Tags:            cloneStringMap(point.Tags),
				Timestamp:       point.Timestamp,
				Fields:          fields,
			}
		}
		return out, nil
	}
	for index, point := range points {
		converted, err := toModelPoint(point)
		if err != nil {
			return nil, err
		}
		out[index] = converted
	}
	return out, nil
}

func toModelTypedBatch(batch TypedBatch) (model.TypedBatch, error) {
	factor, err := timePrecisionFactor(batch.Precision)
	if err != nil {
		return model.TypedBatch{}, err
	}
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
	timestamps := make([]int64, len(batch.Timestamps))
	for index, timestamp := range batch.Timestamps {
		converted, err := timestampToNanoseconds(timestamp, factor)
		if err != nil {
			return model.TypedBatch{}, err
		}
		timestamps[index] = converted
	}
	return model.TypedBatch{
		Database:        batch.Database,
		RetentionPolicy: batch.RetentionPolicy,
		Measurement:     batch.Measurement,
		Tags:            tags,
		Timestamps:      timestamps,
		Fields:          fields,
	}, nil
}

func toModelWriteOptions(opts WriteOptions) model.WriteOptions {
	return model.WriteOptions{Sync: opts.Sync}
}

func cloneStringMap(in map[string]string) map[string]string {
	return collections.CloneMapNilIfEmpty(in)
}
