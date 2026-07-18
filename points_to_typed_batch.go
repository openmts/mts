package mts

import (
	"fmt"
	"sort"
)

// PointsToTypedBatch 将同构 []Point 转换为列式 TypedBatch。
//
// 用于把已有 []Point 接到 WriteTypedBatch 高性能路径。若业务可直接产出列式
// 数据，请跳过本转换，直接调用 WriteTypedBatch，以避免额外分配与扫描开销。
//
// 约束：
//   - 空输入返回空 TypedBatch
//   - 整批 Database/RetentionPolicy/Measurement/Precision 必须一致
//   - tag key 取并集；缺失 tag 填空字符串
//   - field 名集合必须完全一致，且同名字段类型不可冲突
//   - 不允许稀疏 field（某行缺少字段会返回错误）
//
// 返回的 TypedBatch 为新分配数据，修改输入 Point 不会影响结果。
func PointsToTypedBatch(points []Point) (TypedBatch, error) {
	if len(points) == 0 {
		return TypedBatch{}, nil
	}
	schema, err := typedBatchSchemaFromPoints(points)
	if err != nil {
		return TypedBatch{}, err
	}
	return materializeTypedBatch(points, schema), nil
}

type typedBatchSchema struct {
	database        string
	retentionPolicy string
	measurement     string
	precision       TimePrecision
	tagNames        []string
	fieldNames      []string
	fieldTypes      map[string]FieldType
}

func typedBatchSchemaFromPoints(points []Point) (typedBatchSchema, error) {
	first := points[0]
	if first.Measurement == "" {
		return typedBatchSchema{}, fmt.Errorf("%w: measurement is empty", ErrInvalidOptions)
	}
	fieldNames, fieldTypes, err := typedFieldSchemaFromPoint(first)
	if err != nil {
		return typedBatchSchema{}, err
	}
	tagNamesSet := make(map[string]struct{})
	for index, point := range points {
		if err := validateTypedBatchPointIdentity(point, first, index); err != nil {
			return typedBatchSchema{}, err
		}
		if err := validateTypedBatchPointFields(point, fieldTypes, index); err != nil {
			return typedBatchSchema{}, err
		}
		for name := range point.Tags {
			if name == "" {
				return typedBatchSchema{}, fmt.Errorf("%w: points[%d] tag name is empty", ErrInvalidOptions, index)
			}
			tagNamesSet[name] = struct{}{}
		}
	}
	tagNames := make([]string, 0, len(tagNamesSet))
	for name := range tagNamesSet {
		tagNames = append(tagNames, name)
	}
	sort.Strings(tagNames)
	return typedBatchSchema{
		database:        first.Database,
		retentionPolicy: first.RetentionPolicy,
		measurement:     first.Measurement,
		precision:       first.Precision,
		tagNames:        tagNames,
		fieldNames:      fieldNames,
		fieldTypes:      fieldTypes,
	}, nil
}

func typedFieldSchemaFromPoint(point Point) ([]string, map[string]FieldType, error) {
	if len(point.Fields) == 0 {
		return nil, nil, fmt.Errorf("%w: fields are empty", ErrInvalidOptions)
	}
	fieldNames := make([]string, 0, len(point.Fields))
	fieldTypes := make(map[string]FieldType, len(point.Fields))
	for name, value := range point.Fields {
		if name == "" {
			return nil, nil, fmt.Errorf("%w: field name is empty", ErrInvalidOptions)
		}
		if !isSupportedFieldType(value.Type) {
			return nil, nil, fmt.Errorf("%w: unsupported field type %d for %q", ErrInvalidOptions, value.Type, name)
		}
		fieldNames = append(fieldNames, name)
		fieldTypes[name] = value.Type
	}
	sort.Strings(fieldNames)
	return fieldNames, fieldTypes, nil
}

func validateTypedBatchPointIdentity(point, first Point, index int) error {
	if point.Database != first.Database ||
		point.RetentionPolicy != first.RetentionPolicy ||
		point.Measurement != first.Measurement ||
		point.Precision != first.Precision {
		return fmt.Errorf(
			"%w: points[%d] identity mismatch (database/rp/measurement/precision must be uniform)",
			ErrInvalidOptions,
			index,
		)
	}
	return nil
}

func validateTypedBatchPointFields(point Point, fieldTypes map[string]FieldType, index int) error {
	if len(point.Fields) != len(fieldTypes) {
		return fmt.Errorf(
			"%w: points[%d] field set size=%d want %d (sparse fields are not supported)",
			ErrInvalidOptions,
			index,
			len(point.Fields),
			len(fieldTypes),
		)
	}
	for name, value := range point.Fields {
		expected, ok := fieldTypes[name]
		if !ok {
			return fmt.Errorf("%w: points[%d] unexpected field %q", ErrInvalidOptions, index, name)
		}
		if value.Type != expected {
			return fmt.Errorf(
				"%w: points[%d] field %q type=%d want %d",
				ErrInvalidOptions,
				index,
				name,
				value.Type,
				expected,
			)
		}
	}
	return nil
}

func materializeTypedBatch(points []Point, schema typedBatchSchema) TypedBatch {
	rows := len(points)
	timestamps := make([]int64, rows)
	tags := make([]TagColumn, len(schema.tagNames))
	for index, name := range schema.tagNames {
		tags[index] = TagColumn{Name: name, Values: make([]string, rows)}
	}
	fields := allocateTypedFieldColumns(schema.fieldNames, schema.fieldTypes, rows)
	for row, point := range points {
		timestamps[row] = point.Timestamp
		for tagIndex := range tags {
			tags[tagIndex].Values[row] = point.Tags[tags[tagIndex].Name]
		}
		for fieldIndex := range fields {
			fillTypedFieldValue(&fields[fieldIndex], point.Fields[fields[fieldIndex].Name], row)
		}
	}
	return TypedBatch{
		Database:        schema.database,
		RetentionPolicy: schema.retentionPolicy,
		Measurement:     schema.measurement,
		Tags:            tags,
		Timestamps:      timestamps,
		Precision:       schema.precision,
		Fields:          fields,
	}
}

func allocateTypedFieldColumns(names []string, types map[string]FieldType, rows int) []TypedFieldColumn {
	fields := make([]TypedFieldColumn, len(names))
	for index, name := range names {
		column := TypedFieldColumn{Name: name, Type: types[name]}
		switch column.Type {
		case FieldFloat64:
			column.Float64Values = make([]float64, rows)
		case FieldInt64:
			column.Int64Values = make([]int64, rows)
		case FieldString:
			column.StringValues = make([]string, rows)
		case FieldBool:
			column.BoolValues = make([]bool, rows)
		}
		fields[index] = column
	}
	return fields
}

func fillTypedFieldValue(column *TypedFieldColumn, value FieldValue, row int) {
	switch column.Type {
	case FieldFloat64:
		column.Float64Values[row] = value.Float64
	case FieldInt64:
		column.Int64Values[row] = value.Int64
	case FieldString:
		column.StringValues[row] = value.String
	case FieldBool:
		column.BoolValues[row] = value.Bool
	}
}

func isSupportedFieldType(fieldType FieldType) bool {
	switch fieldType {
	case FieldFloat64, FieldInt64, FieldString, FieldBool:
		return true
	default:
		return false
	}
}
