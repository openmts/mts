package mts

// FieldType 表示字段值的物理类型。
type FieldType uint8

const (
	// FieldFloat64 表示 float64 数值字段。
	FieldFloat64 FieldType = iota + 1
	// FieldInt64 表示 int64 数值字段。
	FieldInt64
	// FieldString 表示字符串字段。
	FieldString
	// FieldBool 表示 bool 字段。
	FieldBool
)

// FieldValue 表示一个字段值。
//
// Type 决定读取哪个具体值字段。调用方通常使用 Float64Value、Int64Value、
// StringValue 或 BoolValue 构造，避免手动维护 Type 与值字段的一致性。
type FieldValue struct {
	Type    FieldType `json:"type"`
	Float64 float64   `json:"float64,omitempty"`
	Int64   int64     `json:"int64,omitempty"`
	String  string    `json:"string,omitempty"`
	Bool    bool      `json:"bool,omitempty"`
}

// Point 表示一条时序写入记录。
//
// Database 和 RetentionPolicy 为空时使用 Engine 的默认配置。Timestamp 使用
// Unix nanosecond。Write 会复制 Tags 和 Fields 的内容，调用返回后复用输入
// map 不会污染已写入数据。
type Point struct {
	Database        string                `json:"database"`
	RetentionPolicy string                `json:"retention_policy"`
	Measurement     string                `json:"measurement"`
	Tags            map[string]string     `json:"tags"`
	Timestamp       int64                 `json:"timestamp"`
	Fields          map[string]FieldValue `json:"fields"`
}

// TagColumn 表示 typed batch 中一列 tag 值。
type TagColumn struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// TypedFieldColumn 表示 typed batch 中一列字段值。
//
// Type 决定使用 Float64Values、Int64Values、StringValues 或 BoolValues。
// 对应值切片长度必须与 TypedBatch.Timestamps 一致。
type TypedFieldColumn struct {
	Name          string    `json:"name"`
	Type          FieldType `json:"type"`
	Float64Values []float64 `json:"float64_values,omitempty"`
	Int64Values   []int64   `json:"int64_values,omitempty"`
	StringValues  []string  `json:"string_values,omitempty"`
	BoolValues    []bool    `json:"bool_values,omitempty"`
}

// TypedBatch 表示按列组织的批量写入。
//
// Database 和 RetentionPolicy 为空时使用 Engine 的默认配置。Tags 中每列的
// Values 长度、每个字段值切片长度都必须与 Timestamps 一致。
type TypedBatch struct {
	Database        string             `json:"database"`
	RetentionPolicy string             `json:"retention_policy"`
	Measurement     string             `json:"measurement"`
	Tags            []TagColumn        `json:"tags"`
	Timestamps      []int64            `json:"timestamps"`
	Fields          []TypedFieldColumn `json:"fields"`
}

// WriteOptions 控制写入持久化行为。
type WriteOptions struct {
	// Sync 为 true 时要求 WAL 写入同步落盘，适合更强持久性要求。
	Sync bool
}

// Float64Value 构造 float64 字段值。
func Float64Value(value float64) FieldValue {
	return FieldValue{
		Type:    FieldFloat64,
		Float64: value,
	}
}

// Int64Value 构造 int64 字段值。
func Int64Value(value int64) FieldValue {
	return FieldValue{
		Type:  FieldInt64,
		Int64: value,
	}
}

// StringValue 构造字符串字段值。
func StringValue(value string) FieldValue {
	return FieldValue{
		Type:   FieldString,
		String: value,
	}
}

// BoolValue 构造 bool 字段值。
func BoolValue(value bool) FieldValue {
	return FieldValue{
		Type: FieldBool,
		Bool: value,
	}
}
