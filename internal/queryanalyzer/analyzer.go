package queryanalyzer

import (
	"context"
	"fmt"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/querylang"
)

type SchemaProvider interface {
	ListFields(ctx context.Context, database string, measurement string) ([]model.FieldSchema, error)
}

type Analyzer struct {
	schema SchemaProvider
}

type Analysis struct {
	Spec                   querylang.QuerySpec
	Fields                 map[string]model.FieldSchema
	PushdownPredicates     []querylang.Predicate
	PostFilterPredicates   []querylang.Predicate
	Group                  querylang.GroupSpec
	Order                  querylang.OrderSpec
	RequiresBoundary       bool
	RequiresPostFilterExpr bool
	ScanRequired           bool
}

func New(schema SchemaProvider) *Analyzer {
	return &Analyzer{schema: schema}
}

func (a *Analyzer) Analyze(ctx context.Context, spec querylang.QuerySpec) (Analysis, error) {
	if err := validateSpec(spec); err != nil {
		return Analysis{}, err
	}
	fields, err := a.loadFields(ctx, spec)
	if err != nil {
		return Analysis{}, err
	}
	analysis := Analysis{
		Spec:         spec,
		Fields:       fields,
		Group:        spec.Group,
		Order:        spec.Order,
		ScanRequired: true,
	}
	if err := analysis.validateFields(); err != nil {
		return Analysis{}, err
	}
	if err := analysis.validatePredicates(); err != nil {
		return Analysis{}, err
	}
	if err := analysis.validateGroup(); err != nil {
		return Analysis{}, err
	}
	return analysis, nil
}

func validateSpec(spec querylang.QuerySpec) error {
	if spec.Window < 0 || spec.Group.Window < 0 {
		return newError(ErrInvalidWindow, "window must be greater than or equal to zero")
	}
	if spec.Limit < 0 || spec.Offset < 0 {
		return newError(ErrInvalidPagination, "limit and offset must be greater than or equal to zero")
	}
	return nil
}

func (a *Analyzer) loadFields(
	ctx context.Context,
	spec querylang.QuerySpec,
) (map[string]model.FieldSchema, error) {
	if a.schema == nil {
		return map[string]model.FieldSchema{}, nil
	}
	fields, err := a.schema.ListFields(ctx, spec.Scope.Database, spec.Measurement)
	if err != nil {
		return nil, fmt.Errorf("list fields: %w", err)
	}
	if len(fields) == 0 {
		return nil, newError(ErrMeasurementNotFound, "measurement has no schema fields")
	}
	out := make(map[string]model.FieldSchema, len(fields))
	for _, field := range fields {
		out[field.Name] = field
	}
	return out, nil
}

func (a *Analysis) validateFields() error {
	for _, fieldName := range a.Spec.Fields {
		if _, ok := a.Fields[fieldName]; !ok {
			return newError(ErrFieldNotFound, "query field is not present in schema")
		}
	}
	for _, aggregate := range a.Spec.Aggregates {
		field, ok := a.Fields[aggregate.Field]
		if !ok {
			return newError(ErrFieldNotFound, "aggregate field is not present in schema")
		}
		rule, err := validateFunction(aggregate.Function, field)
		if err != nil {
			return err
		}
		a.RequiresBoundary = a.RequiresBoundary || rule.boundary
	}
	return nil
}

func (a *Analysis) validatePredicates() error {
	if a.Spec.Expr.Kind != querylang.ExprNone {
		return a.validateExpr(a.Spec.Expr)
	}
	for _, predicate := range a.Spec.Predicates {
		if err := a.classifyPredicate(predicate); err != nil {
			return err
		}
	}
	return nil
}

func (a *Analysis) validateExpr(expr querylang.Expr) error {
	switch expr.Kind {
	case querylang.ExprNone:
		return nil
	case querylang.ExprPredicate:
		return a.classifyPredicate(expr.Predicate)
	case querylang.ExprAnd:
		return a.validateExprChildren(expr.Children)
	case querylang.ExprOr, querylang.ExprNot:
		a.RequiresPostFilterExpr = true
		return a.validateExprChildren(expr.Children)
	default:
		return newError(ErrUnsupportedFunction, "query expression is not supported")
	}
}

func (a *Analysis) validateExprChildren(children []querylang.Expr) error {
	for _, child := range children {
		if err := a.validateExpr(child); err != nil {
			return err
		}
	}
	return nil
}

func (a *Analysis) classifyPredicate(predicate querylang.Predicate) error {
	switch predicate.Kind {
	case querylang.PredicateTimeRange,
		querylang.PredicateTagEq,
		querylang.PredicateTagNe,
		querylang.PredicateTagExists,
		querylang.PredicateTagIn:
		a.PushdownPredicates = append(a.PushdownPredicates, predicate)
	case querylang.PredicateFieldEq,
		querylang.PredicateFieldNe,
		querylang.PredicateFieldGT,
		querylang.PredicateFieldGTE,
		querylang.PredicateFieldLT,
		querylang.PredicateFieldLTE:
		if err := a.validateFieldPredicate(predicate); err != nil {
			return err
		}
		a.RequiresPostFilterExpr = true
		a.PostFilterPredicates = append(a.PostFilterPredicates, predicate)
	default:
		return newError(ErrUnsupportedFunction, "query predicate is not supported")
	}
	return nil
}

func (a *Analysis) validateFieldPredicate(predicate querylang.Predicate) error {
	field, ok := a.Fields[predicate.Name]
	if !ok {
		return newError(ErrFieldNotFound, "predicate field is not present in schema")
	}
	if !predicateValueCompatible(field.Type, predicate.Value.Type, predicate.Kind) {
		return newError(ErrFunctionTypeMismatch, "predicate value type is not compatible with field")
	}
	return nil
}

func predicateValueCompatible(
	fieldType model.FieldType,
	valueType model.FieldType,
	kind querylang.PredicateKind,
) bool {
	if isComparisonPredicate(kind) {
		return isNumeric(fieldType) && isNumeric(valueType)
	}
	if fieldType == valueType {
		return true
	}
	return isNumeric(fieldType) && isNumeric(valueType)
}

func isComparisonPredicate(kind querylang.PredicateKind) bool {
	switch kind {
	case querylang.PredicateFieldGT,
		querylang.PredicateFieldGTE,
		querylang.PredicateFieldLT,
		querylang.PredicateFieldLTE:
		return true
	default:
		return false
	}
}

func (a *Analysis) validateGroup() error {
	if a.Spec.Group.Window <= 0 {
		return nil
	}
	if len(a.Spec.Aggregates) == 0 {
		return newError(ErrInvalidGroup, "time group requires aggregate query")
	}
	return nil
}
