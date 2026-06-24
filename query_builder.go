package mts

import (
	"errors"
	"strings"
	"time"
)

// ErrInvalidQuery 表示 QueryBuilder 构造的查询非法。
var ErrInvalidQuery = errors.New("invalid query")

// QueryBuilder 构造结构化查询。
type QueryBuilder struct {
	query Query
	err   error
}

// NewQuery 创建一个查询 Builder。
func NewQuery() *QueryBuilder {
	return &QueryBuilder{
		query: Query{
			Tags:  map[string]string{},
			Order: QueryOrder{By: QueryOrderByTime, Direction: QuerySortAsc},
		},
	}
}

// Select 设置查询字段；空字符串会被忽略。
func (b *QueryBuilder) Select(fields ...string) *QueryBuilder {
	b.query.Fields = cleanQueryStrings(fields)
	return b
}

// From 设置查询的 database、retention policy 和 measurement。
func (b *QueryBuilder) From(database string, retentionPolicy string, measurement string) *QueryBuilder {
	b.query.Database = database
	b.query.RetentionPolicy = retentionPolicy
	b.query.Measurement = strings.TrimSpace(measurement)
	return b
}

// FromDownsamplePolicy 设置查询源为降采样策略的目标 measurement。
func (b *QueryBuilder) FromDownsamplePolicy(policy DownsamplePolicy) *QueryBuilder {
	b.From(policy.TargetDatabase, policy.TargetRetention, policy.TargetMeasurement)
	tagName := strings.TrimSpace(policy.PolicyTagName)
	if tagName == "" {
		tagName = "mts_downsample_policy"
	}
	b.Where(TagEq(tagName, strings.TrimSpace(policy.Name)))
	return b
}

// Where 追加谓词，多个谓词按 AND 合并。
func (b *QueryBuilder) Where(predicates ...QueryPredicate) *QueryBuilder {
	for _, predicate := range predicates {
		b.addPredicate(predicate)
	}
	return b
}

// WhereExpr 追加结构化表达式。
func (b *QueryBuilder) WhereExpr(expr QueryExpr) *QueryBuilder {
	b.query.Expr = mergeQueryExpr(b.query.Expr, expr)
	b.query.Predicates = append(b.query.Predicates, collectQueryExprPredicates(expr)...)
	return b
}

// TimeRange 使用 Unix nanosecond 设置查询时间范围。
func (b *QueryBuilder) TimeRange(start int64, end int64) *QueryBuilder {
	b.addPredicate(QueryPredicate{Kind: QueryPredicateTimeRange, Start: start, End: end})
	b.query.StartTime = start
	b.query.EndTime = end
	return b
}

// TimeRangeTime 使用 time.Time 设置查询时间范围。
func (b *QueryBuilder) TimeRangeTime(start time.Time, end time.Time) *QueryBuilder {
	b.query.Precision = PrecisionNanosecond
	return b.TimeRange(start.UnixNano(), end.UnixNano())
}

// Precision 设置查询时间戳输入和返回结果时间戳的单位。
func (b *QueryBuilder) Precision(precision TimePrecision) *QueryBuilder {
	b.query.Precision = precision
	return b
}

// Aggregate 追加聚合函数。
func (b *QueryBuilder) Aggregate(function string, field string) *QueryBuilder {
	b.query.Aggregates = append(b.query.Aggregates, AggregateSpec{
		Field:    strings.TrimSpace(field),
		Function: normalizeQueryFunction(function),
	})
	return b
}

// GroupByTags 设置 tag 分组字段。
func (b *QueryBuilder) GroupByTags(tags ...string) *QueryBuilder {
	b.query.Group.Tags = cleanQueryStrings(tags)
	return b
}

// GroupByTime 设置时间窗口分组。
func (b *QueryBuilder) GroupByTime(window time.Duration) *QueryBuilder {
	b.query.Group.Window = window
	b.query.Window = window
	return b
}

// OrderByTimeAsc 按时间升序排序。
func (b *QueryBuilder) OrderByTimeAsc() *QueryBuilder {
	b.query.Order = QueryOrder{By: QueryOrderByTime, Direction: QuerySortAsc}
	return b
}

// OrderByTimeDesc 按时间降序排序。
func (b *QueryBuilder) OrderByTimeDesc() *QueryBuilder {
	b.query.Order = QueryOrder{By: QueryOrderByTime, Direction: QuerySortDesc}
	return b
}

// Limit 设置最大返回行数或列数。
func (b *QueryBuilder) Limit(limit int) *QueryBuilder {
	b.query.Limit = limit
	return b
}

// Offset 设置跳过数量。
func (b *QueryBuilder) Offset(offset int) *QueryBuilder {
	b.query.Offset = offset
	return b
}

// Build 返回可执行 Query。
func (b *QueryBuilder) Build() (Query, error) {
	if b.err != nil {
		return Query{}, b.err
	}
	if strings.TrimSpace(b.query.Measurement) == "" {
		return Query{}, ErrInvalidQuery
	}
	if b.query.Limit < 0 || b.query.Offset < 0 {
		return Query{}, ErrInvalidQuery
	}
	if b.query.EndTime != 0 && b.query.StartTime > b.query.EndTime {
		return Query{}, ErrInvalidQuery
	}
	if _, err := timePrecisionFactor(b.query.Precision); err != nil {
		return Query{}, err
	}
	if b.query.Order.By == QueryOrderByNone {
		b.query.Order = QueryOrder{By: QueryOrderByTime, Direction: QuerySortAsc}
	}
	return b.query, nil
}

func (b *QueryBuilder) addPredicate(predicate QueryPredicate) {
	if predicate.Kind == QueryPredicateTimeRange {
		b.query.StartTime = predicate.Start
		b.query.EndTime = predicate.End
	}
	if predicate.Kind == QueryPredicateTagEq && len(predicate.StringValues) > 0 {
		if b.query.Tags == nil {
			b.query.Tags = map[string]string{}
		}
		b.query.Tags[predicate.Name] = predicate.StringValues[0]
	}
	b.query.Predicates = append(b.query.Predicates, predicate)
	b.query.Expr = mergeQueryExpr(b.query.Expr, PredicateQueryExpr(predicate))
}

// TagEq 构造 tag 等值谓词。
func TagEq(name string, value string) QueryPredicate {
	return QueryPredicate{Kind: QueryPredicateTagEq, Name: strings.TrimSpace(name), StringValues: []string{value}}
}

// TagNe 构造 tag 不等谓词。
func TagNe(name string, value string) QueryPredicate {
	return QueryPredicate{Kind: QueryPredicateTagNe, Name: strings.TrimSpace(name), StringValues: []string{value}}
}

// TagExists 构造 tag 存在谓词。
func TagExists(name string) QueryPredicate {
	return QueryPredicate{Kind: QueryPredicateTagExists, Name: strings.TrimSpace(name)}
}

// TagIn 构造 tag 集合谓词。
func TagIn(name string, values ...string) QueryPredicate {
	return QueryPredicate{Kind: QueryPredicateTagIn, Name: strings.TrimSpace(name), StringValues: append([]string(nil), values...)}
}

// FieldEq 构造字段等值谓词。
func FieldEq(name string, value FieldValue) QueryPredicate {
	return fieldQueryPredicate(QueryPredicateFieldEq, name, value)
}

// FieldNe 构造字段不等谓词。
func FieldNe(name string, value FieldValue) QueryPredicate {
	return fieldQueryPredicate(QueryPredicateFieldNe, name, value)
}

// FieldGT 构造字段大于谓词。
func FieldGT(name string, value FieldValue) QueryPredicate {
	return fieldQueryPredicate(QueryPredicateFieldGT, name, value)
}

// FieldGTE 构造字段大于等于谓词。
func FieldGTE(name string, value FieldValue) QueryPredicate {
	return fieldQueryPredicate(QueryPredicateFieldGTE, name, value)
}

// FieldLT 构造字段小于谓词。
func FieldLT(name string, value FieldValue) QueryPredicate {
	return fieldQueryPredicate(QueryPredicateFieldLT, name, value)
}

// FieldLTE 构造字段小于等于谓词。
func FieldLTE(name string, value FieldValue) QueryPredicate {
	return fieldQueryPredicate(QueryPredicateFieldLTE, name, value)
}

// PredicateQueryExpr 将谓词包装为表达式叶子节点。
func PredicateQueryExpr(predicate QueryPredicate) QueryExpr {
	return QueryExpr{Kind: QueryExprPredicate, Predicate: predicate}
}

// AndExpr 构造 AND 表达式。
func AndExpr(children ...QueryExpr) QueryExpr {
	return queryExpr(QueryExprAnd, children...)
}

// OrExpr 构造 OR 表达式。
func OrExpr(children ...QueryExpr) QueryExpr {
	return queryExpr(QueryExprOr, children...)
}

// NotExpr 构造 NOT 表达式。
func NotExpr(child QueryExpr) QueryExpr {
	return queryExpr(QueryExprNot, child)
}

func fieldQueryPredicate(kind QueryPredicateKind, name string, value FieldValue) QueryPredicate {
	return QueryPredicate{Kind: kind, Name: strings.TrimSpace(name), Value: value}
}

func queryExpr(kind QueryExprKind, children ...QueryExpr) QueryExpr {
	return QueryExpr{Kind: kind, Children: append([]QueryExpr(nil), children...)}
}

func mergeQueryExpr(left QueryExpr, right QueryExpr) QueryExpr {
	if left.Kind == QueryExprNone {
		return right
	}
	if right.Kind == QueryExprNone {
		return left
	}
	return AndExpr(left, right)
}

func collectQueryExprPredicates(expr QueryExpr) []QueryPredicate {
	out := make([]QueryPredicate, 0)
	appendQueryExprPredicates(&out, expr)
	return out
}

func appendQueryExprPredicates(out *[]QueryPredicate, expr QueryExpr) {
	if expr.Kind == QueryExprPredicate {
		*out = append(*out, expr.Predicate)
		return
	}
	for _, child := range expr.Children {
		appendQueryExprPredicates(out, child)
	}
}

func cleanQueryStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		clean := strings.TrimSpace(value)
		if clean != "" {
			out = append(out, clean)
		}
	}
	return out
}

func normalizeQueryFunction(function string) string {
	normalized := strings.ToLower(strings.TrimSpace(function))
	if normalized == "mean" {
		return "avg"
	}
	return normalized
}
