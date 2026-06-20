package querylang

import (
	"strings"
	"time"

	"github.com/openmts/mts/internal/model"
)

type Builder struct {
	spec QuerySpec
}

func NewBuilder() *Builder {
	return &Builder{
		spec: QuerySpec{
			Tags:  map[string]string{},
			Order: OrderSpec{By: OrderByTime, Direction: SortAsc},
		},
	}
}

func (b *Builder) Select(fields ...string) *Builder {
	b.spec.Fields = cleanStrings(fields)
	return b
}

func (b *Builder) From(database string, retentionPolicy string, measurement string) *Builder {
	b.spec.Scope = Scope{Database: database, RetentionPolicy: retentionPolicy}
	b.spec.Measurement = strings.TrimSpace(measurement)
	return b
}

func (b *Builder) Where(predicates ...Predicate) *Builder {
	for _, predicate := range predicates {
		b.addPredicate(predicate)
	}
	return b
}

func (b *Builder) WhereExpr(expr Expr) *Builder {
	b.spec.Expr = mergeExpr(b.spec.Expr, expr)
	b.spec.Predicates = append(b.spec.Predicates, collectExprPredicates(expr)...)
	return b
}

func (b *Builder) TimeRange(start int64, end int64) *Builder {
	b.addPredicate(Predicate{Kind: PredicateTimeRange, Start: start, End: end})
	b.spec.TimeRange = TimeRange{Start: start, End: end}
	return b
}

func (b *Builder) Aggregate(function string, field string) *Builder {
	b.spec.Aggregates = append(b.spec.Aggregates, Aggregate{
		Field:    strings.TrimSpace(field),
		Function: normalizeFunction(function),
	})
	return b
}

func (b *Builder) GroupByTags(tags ...string) *Builder {
	b.spec.Group.Tags = cleanStrings(tags)
	return b
}

func (b *Builder) GroupByTime(window time.Duration) *Builder {
	b.spec.Group.Window = window
	b.spec.Window = window
	return b
}

func (b *Builder) OrderByTimeAsc() *Builder {
	b.spec.Order = OrderSpec{By: OrderByTime, Direction: SortAsc}
	return b
}

func (b *Builder) OrderByTimeDesc() *Builder {
	b.spec.Order = OrderSpec{By: OrderByTime, Direction: SortDesc}
	return b
}

func (b *Builder) Limit(limit int) *Builder {
	b.spec.Limit = limit
	return b
}

func (b *Builder) Offset(offset int) *Builder {
	b.spec.Offset = offset
	return b
}

func (b *Builder) Cursor(cursor string) *Builder {
	b.spec.Cursor = strings.TrimSpace(cursor)
	return b
}

func (b *Builder) Build() (QuerySpec, error) {
	spec := b.spec
	if spec.Tags == nil {
		spec.Tags = map[string]string{}
	}
	if spec.Order.By == OrderByNone {
		spec.Order = OrderSpec{By: OrderByTime, Direction: SortAsc}
	}
	if err := validate(spec); err != nil {
		return QuerySpec{}, err
	}
	return spec, nil
}

func (b *Builder) addPredicate(predicate Predicate) {
	if predicate.Kind == PredicateTimeRange {
		if predicate.Start != 0 {
			b.spec.TimeRange.Start = predicate.Start
		}
		if predicate.End != 0 {
			b.spec.TimeRange.End = predicate.End
		}
	}
	if predicate.Kind == PredicateTagEq && len(predicate.StringValues) > 0 {
		if b.spec.Tags == nil {
			b.spec.Tags = map[string]string{}
		}
		b.spec.Tags[predicate.Name] = predicate.StringValues[0]
	}
	b.spec.Predicates = append(b.spec.Predicates, predicate)
	b.spec.Expr = mergeExpr(b.spec.Expr, PredicateExpr(predicate))
}

func TagEq(name string, value string) Predicate {
	return Predicate{Kind: PredicateTagEq, Name: strings.TrimSpace(name), StringValues: []string{value}}
}

func TagNe(name string, value string) Predicate {
	return Predicate{Kind: PredicateTagNe, Name: strings.TrimSpace(name), StringValues: []string{value}}
}

func TagExists(name string) Predicate {
	return Predicate{Kind: PredicateTagExists, Name: strings.TrimSpace(name)}
}

func TagIn(name string, values ...string) Predicate {
	return Predicate{Kind: PredicateTagIn, Name: strings.TrimSpace(name), StringValues: append([]string(nil), values...)}
}

func FieldEq(name string, value model.FieldValue) Predicate {
	return fieldPredicate(PredicateFieldEq, name, value)
}

func FieldNe(name string, value model.FieldValue) Predicate {
	return fieldPredicate(PredicateFieldNe, name, value)
}

func FieldGT(name string, value model.FieldValue) Predicate {
	return fieldPredicate(PredicateFieldGT, name, value)
}

func FieldGTE(name string, value model.FieldValue) Predicate {
	return fieldPredicate(PredicateFieldGTE, name, value)
}

func FieldLT(name string, value model.FieldValue) Predicate {
	return fieldPredicate(PredicateFieldLT, name, value)
}

func FieldLTE(name string, value model.FieldValue) Predicate {
	return fieldPredicate(PredicateFieldLTE, name, value)
}

func PredicateExpr(predicate Predicate) Expr {
	return Expr{Kind: ExprPredicate, Predicate: predicate}
}

func And(children ...Expr) Expr {
	return exprWithChildren(ExprAnd, children...)
}

func Or(children ...Expr) Expr {
	return exprWithChildren(ExprOr, children...)
}

func Not(child Expr) Expr {
	return exprWithChildren(ExprNot, child)
}

func fieldPredicate(kind PredicateKind, name string, value model.FieldValue) Predicate {
	return Predicate{Kind: kind, Name: strings.TrimSpace(name), Value: value}
}

func exprWithChildren(kind ExprKind, children ...Expr) Expr {
	return Expr{Kind: kind, Children: append([]Expr(nil), children...)}
}

func mergeExpr(left Expr, right Expr) Expr {
	if left.Kind == ExprNone {
		return right
	}
	if right.Kind == ExprNone {
		return left
	}
	return And(left, right)
}

func collectExprPredicates(expr Expr) []Predicate {
	out := make([]Predicate, 0)
	appendExprPredicates(&out, expr)
	return out
}

func appendExprPredicates(out *[]Predicate, expr Expr) {
	if expr.Kind == ExprPredicate {
		*out = append(*out, expr.Predicate)
		return
	}
	for _, child := range expr.Children {
		appendExprPredicates(out, child)
	}
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		clean := strings.TrimSpace(value)
		if clean != "" {
			out = append(out, clean)
		}
	}
	return out
}
