package mts

import "time"

const (
	// AggregateCount 统计样本数量。
	AggregateCount = "count"
	// AggregateSum 计算数值样本总和。
	AggregateSum = "sum"
	// AggregateAvg 计算数值样本平均值。
	AggregateAvg = "avg"
	// AggregateMean 是 AggregateAvg 的别名。
	AggregateMean = "mean"
	// AggregateMin 计算最小值。
	AggregateMin = "min"
	// AggregateMax 计算最大值。
	AggregateMax = "max"
	// AggregateFirst 返回窗口内首个样本。
	AggregateFirst = "first"
	// AggregateLast 返回窗口内最后一个样本。
	AggregateLast = "last"
)

// Query 表示一次 Builder/API 查询。
//
// Database 和 RetentionPolicy 为空时使用 Engine 默认值。StartTime 和 EndTime
// 使用 Unix nanosecond；EndTime 为 0 时内部查询会使用最大时间边界。
type Query struct {
	Database        string            `json:"database"`
	RetentionPolicy string            `json:"retention_policy"`
	Measurement     string            `json:"measurement"`
	Tags            map[string]string `json:"tags"`
	Fields          []string          `json:"fields"`
	StartTime       int64             `json:"start_time"`
	EndTime         int64             `json:"end_time"`
	Predicates      []QueryPredicate  `json:"predicates"`
	Expr            QueryExpr         `json:"expr,omitempty"`
	Aggregates      []AggregateSpec   `json:"aggregates"`
	Window          time.Duration     `json:"window"`
	Group           QueryGroup        `json:"group"`
	Order           QueryOrder        `json:"order"`
	Limit           int               `json:"limit"`
	Offset          int               `json:"offset"`
	Budget          QueryBudget       `json:"budget"`
}

// QueryPredicateKind 表示查询谓词类型。
type QueryPredicateKind uint8

const (
	// QueryPredicateTimeRange 表示时间范围谓词。
	QueryPredicateTimeRange QueryPredicateKind = iota + 1
	// QueryPredicateTagEq 表示 tag 等值谓词。
	QueryPredicateTagEq
	// QueryPredicateTagNe 表示 tag 不等谓词。
	QueryPredicateTagNe
	// QueryPredicateTagExists 表示 tag 存在谓词。
	QueryPredicateTagExists
	// QueryPredicateTagIn 表示 tag 集合谓词。
	QueryPredicateTagIn
	// QueryPredicateFieldEq 表示字段等值谓词。
	QueryPredicateFieldEq
	// QueryPredicateFieldNe 表示字段不等谓词。
	QueryPredicateFieldNe
	// QueryPredicateFieldGT 表示字段大于谓词。
	QueryPredicateFieldGT
	// QueryPredicateFieldGTE 表示字段大于等于谓词。
	QueryPredicateFieldGTE
	// QueryPredicateFieldLT 表示字段小于谓词。
	QueryPredicateFieldLT
	// QueryPredicateFieldLTE 表示字段小于等于谓词。
	QueryPredicateFieldLTE
)

// QueryPredicate 表示可下推或 post-filter 的查询谓词。
type QueryPredicate struct {
	Kind         QueryPredicateKind `json:"kind"`
	Name         string             `json:"name"`
	StringValues []string           `json:"string_values,omitempty"`
	Value        FieldValue         `json:"value,omitempty"`
	Start        int64              `json:"start,omitempty"`
	End          int64              `json:"end,omitempty"`
}

// QueryExprKind 表示查询表达式节点类型。
type QueryExprKind uint8

const (
	// QueryExprNone 表示空表达式。
	QueryExprNone QueryExprKind = iota
	// QueryExprPredicate 表示叶子谓词表达式。
	QueryExprPredicate
	// QueryExprAnd 表示 AND 表达式。
	QueryExprAnd
	// QueryExprOr 表示 OR 表达式。
	QueryExprOr
	// QueryExprNot 表示 NOT 表达式。
	QueryExprNot
)

// QueryExpr 表示结构化查询表达式树。
type QueryExpr struct {
	Kind      QueryExprKind  `json:"kind,omitempty"`
	Predicate QueryPredicate `json:"predicate,omitempty"`
	Children  []QueryExpr    `json:"children,omitempty"`
}

// QueryGroup 表示 group by 条件。
type QueryGroup struct {
	Tags   []string      `json:"tags,omitempty"`
	Window time.Duration `json:"window,omitempty"`
}

// QueryOrderBy 表示查询排序字段。
type QueryOrderBy uint8

const (
	// QueryOrderByNone 表示未指定排序字段。
	QueryOrderByNone QueryOrderBy = iota
	// QueryOrderByTime 表示按时间排序。
	QueryOrderByTime
)

// QuerySortDirection 表示排序方向。
type QuerySortDirection uint8

const (
	// QuerySortAsc 表示升序排序。
	QuerySortAsc QuerySortDirection = iota + 1
	// QuerySortDesc 表示降序排序。
	QuerySortDesc
)

// QueryOrder 表示查询排序规则。
type QueryOrder struct {
	By        QueryOrderBy       `json:"by"`
	Direction QuerySortDirection `json:"direction"`
}

// AggregateSpec 表示一个聚合函数和目标字段。
type AggregateSpec struct {
	Field    string `json:"field"`
	Function string `json:"function"`
}

// QueryBudget 表示查询读取预算。
type QueryBudget struct {
	MaxShards  int `json:"max_shards"`
	MaxParts   int `json:"max_parts"`
	MaxSamples int `json:"max_samples"`
}
