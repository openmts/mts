package mts

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidQuery = errors.New("invalid query")

type QueryBuilder struct {
	query Query
	err   error
}

func NewQuery() *QueryBuilder {
	return &QueryBuilder{
		query: Query{
			Tags:  map[string]string{},
			Order: QueryOrder{By: QueryOrderByTime, Direction: QuerySortAsc},
		},
	}
}

func (b *QueryBuilder) Select(fields ...string) *QueryBuilder {
	b.query.Fields = cleanQueryStrings(fields)
	return b
}

func (b *QueryBuilder) From(database string, retentionPolicy string, measurement string) *QueryBuilder {
	b.query.Database = database
	b.query.RetentionPolicy = retentionPolicy
	b.query.Measurement = strings.TrimSpace(measurement)
	return b
}

func (b *QueryBuilder) FromDownsamplePolicy(policy DownsamplePolicy) *QueryBuilder {
	b.From(policy.TargetDatabase, policy.TargetRetention, policy.TargetMeasurement)
	tagName := strings.TrimSpace(policy.PolicyTagName)
	if tagName == "" {
		tagName = "mts_downsample_policy"
	}
	b.Where(TagEq(tagName, strings.TrimSpace(policy.Name)))
	return b
}

func (b *QueryBuilder) Where(predicates ...QueryPredicate) *QueryBuilder {
	for _, predicate := range predicates {
		b.addPredicate(predicate)
	}
	return b
}

func (b *QueryBuilder) WhereExpr(expr QueryExpr) *QueryBuilder {
	b.query.Expr = mergeQueryExpr(b.query.Expr, expr)
	b.query.Predicates = append(b.query.Predicates, collectQueryExprPredicates(expr)...)
	return b
}

func (b *QueryBuilder) TimeRange(start int64, end int64) *QueryBuilder {
	b.addPredicate(QueryPredicate{Kind: QueryPredicateTimeRange, Start: start, End: end})
	b.query.StartTime = start
	b.query.EndTime = end
	return b
}

func (b *QueryBuilder) Aggregate(function string, field string) *QueryBuilder {
	b.query.Aggregates = append(b.query.Aggregates, AggregateSpec{
		Field:    strings.TrimSpace(field),
		Function: normalizeQueryFunction(function),
	})
	return b
}

func (b *QueryBuilder) GroupByTags(tags ...string) *QueryBuilder {
	b.query.Group.Tags = cleanQueryStrings(tags)
	return b
}

func (b *QueryBuilder) GroupByTime(window time.Duration) *QueryBuilder {
	b.query.Group.Window = window
	b.query.Window = window
	return b
}

func (b *QueryBuilder) OrderByTimeAsc() *QueryBuilder {
	b.query.Order = QueryOrder{By: QueryOrderByTime, Direction: QuerySortAsc}
	return b
}

func (b *QueryBuilder) OrderByTimeDesc() *QueryBuilder {
	b.query.Order = QueryOrder{By: QueryOrderByTime, Direction: QuerySortDesc}
	return b
}

func (b *QueryBuilder) Limit(limit int) *QueryBuilder {
	b.query.Limit = limit
	return b
}

func (b *QueryBuilder) Offset(offset int) *QueryBuilder {
	b.query.Offset = offset
	return b
}

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

func TagEq(name string, value string) QueryPredicate {
	return QueryPredicate{Kind: QueryPredicateTagEq, Name: strings.TrimSpace(name), StringValues: []string{value}}
}

func TagNe(name string, value string) QueryPredicate {
	return QueryPredicate{Kind: QueryPredicateTagNe, Name: strings.TrimSpace(name), StringValues: []string{value}}
}

func TagExists(name string) QueryPredicate {
	return QueryPredicate{Kind: QueryPredicateTagExists, Name: strings.TrimSpace(name)}
}

func TagIn(name string, values ...string) QueryPredicate {
	return QueryPredicate{Kind: QueryPredicateTagIn, Name: strings.TrimSpace(name), StringValues: append([]string(nil), values...)}
}

func FieldEq(name string, value FieldValue) QueryPredicate {
	return fieldQueryPredicate(QueryPredicateFieldEq, name, value)
}

func FieldNe(name string, value FieldValue) QueryPredicate {
	return fieldQueryPredicate(QueryPredicateFieldNe, name, value)
}

func FieldGT(name string, value FieldValue) QueryPredicate {
	return fieldQueryPredicate(QueryPredicateFieldGT, name, value)
}

func FieldGTE(name string, value FieldValue) QueryPredicate {
	return fieldQueryPredicate(QueryPredicateFieldGTE, name, value)
}

func FieldLT(name string, value FieldValue) QueryPredicate {
	return fieldQueryPredicate(QueryPredicateFieldLT, name, value)
}

func FieldLTE(name string, value FieldValue) QueryPredicate {
	return fieldQueryPredicate(QueryPredicateFieldLTE, name, value)
}

func PredicateQueryExpr(predicate QueryPredicate) QueryExpr {
	return QueryExpr{Kind: QueryExprPredicate, Predicate: predicate}
}

func AndExpr(children ...QueryExpr) QueryExpr {
	return queryExpr(QueryExprAnd, children...)
}

func OrExpr(children ...QueryExpr) QueryExpr {
	return queryExpr(QueryExprOr, children...)
}

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
