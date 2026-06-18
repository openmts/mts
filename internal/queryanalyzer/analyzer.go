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
	Spec             querylang.QuerySpec
	Fields           map[string]model.FieldSchema
	RequiresBoundary bool
	ScanRequired     bool
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
		ScanRequired: true,
	}
	if err := analysis.validateFields(); err != nil {
		return Analysis{}, err
	}
	return analysis, nil
}

func validateSpec(spec querylang.QuerySpec) error {
	if spec.Window < 0 {
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
