package querylang

import (
	"time"

	"github.com/openmts/mts/internal/model"
)

type Defaults struct {
	Database        string
	RetentionPolicy string
	Output          OutputKind
}

type Scope struct {
	Database        string
	RetentionPolicy string
}

type TimeRange struct {
	Start int64
	End   int64
}

type Aggregate struct {
	Field    string
	Function string
}

type PredicateKind uint8

const (
	PredicateTimeRange PredicateKind = iota + 1
	PredicateTagEq
	PredicateTagNe
	PredicateTagExists
	PredicateTagIn
	PredicateFieldEq
	PredicateFieldNe
	PredicateFieldGT
	PredicateFieldGTE
	PredicateFieldLT
	PredicateFieldLTE
)

type Predicate struct {
	Kind         PredicateKind
	Name         string
	StringValues []string
	Value        model.FieldValue
	Start        int64
	End          int64
}

type GroupSpec struct {
	Tags   []string
	Window time.Duration
}

type OrderBy uint8

const (
	OrderByNone OrderBy = iota
	OrderByTime
)

type SortDirection uint8

const (
	SortAsc SortDirection = iota + 1
	SortDesc
)

type OrderSpec struct {
	By        OrderBy
	Direction SortDirection
}

type OutputKind uint8

const (
	OutputColumns OutputKind = iota + 1
	OutputRows
	OutputAggregates
	OutputExplain
	OutputProfile
)

type Output struct {
	Kind OutputKind
}

type QuerySpec struct {
	Scope       Scope
	Measurement string
	Tags        map[string]string
	Fields      []string
	TimeRange   TimeRange
	Predicates  []Predicate
	Expr        Expr
	Aggregates  []Aggregate
	Window      time.Duration
	Group       GroupSpec
	Order       OrderSpec
	Limit       int
	Offset      int
	Cursor      string
	Output      Output
}

type ExprKind uint8

const (
	ExprNone ExprKind = iota
	ExprPredicate
	ExprAnd
	ExprOr
	ExprNot
)

type Expr struct {
	Kind      ExprKind
	Predicate Predicate
	Children  []Expr
}

func (s QuerySpec) ToModelQuery() model.Query {
	aggregates := make([]model.AggregateSpec, 0, len(s.Aggregates))
	for _, aggregate := range s.Aggregates {
		aggregates = append(aggregates, model.AggregateSpec{
			Field:    aggregate.Field,
			Function: aggregate.Function,
		})
	}
	return model.Query{
		Database:        s.Scope.Database,
		RetentionPolicy: s.Scope.RetentionPolicy,
		Measurement:     s.Measurement,
		Tags:            cloneTags(s.Tags),
		Fields:          append([]string(nil), s.Fields...),
		StartTime:       s.TimeRange.Start,
		EndTime:         s.TimeRange.End,
		Predicates:      toModelPredicates(s.Predicates),
		Expr:            toModelExpr(s.Expr),
		Aggregates:      aggregates,
		Window:          s.Window,
		Group: model.QueryGroup{
			Tags:   append([]string(nil), s.Group.Tags...),
			Window: s.Group.Window,
		},
		Order: model.QueryOrder{
			By:        model.QueryOrderBy(s.Order.By),
			Direction: model.QuerySortDirection(s.Order.Direction),
		},
		Limit:  s.Limit,
		Offset: s.Offset,
		Cursor: s.Cursor,
	}
}

func toModelExpr(expr Expr) model.QueryExpr {
	out := model.QueryExpr{
		Kind:      model.QueryExprKind(expr.Kind),
		Predicate: toModelPredicate(expr.Predicate),
		Children:  make([]model.QueryExpr, 0, len(expr.Children)),
	}
	for _, child := range expr.Children {
		out.Children = append(out.Children, toModelExpr(child))
	}
	return out
}

func toModelPredicates(predicates []Predicate) []model.QueryPredicate {
	out := make([]model.QueryPredicate, 0, len(predicates))
	for _, predicate := range predicates {
		out = append(out, toModelPredicate(predicate))
	}
	return out
}

func toModelPredicate(predicate Predicate) model.QueryPredicate {
	return model.QueryPredicate{
		Kind:         model.QueryPredicateKind(predicate.Kind),
		Name:         predicate.Name,
		StringValues: append([]string(nil), predicate.StringValues...),
		Value:        predicate.Value,
		Start:        predicate.Start,
		End:          predicate.End,
	}
}
