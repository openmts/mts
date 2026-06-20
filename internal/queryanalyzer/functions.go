package queryanalyzer

import "github.com/openmts/mts/internal/model"

type functionRule struct {
	numericOnly bool
	boundary    bool
}

var functionRules = map[string]functionRule{
	"count":      {},
	"sum":        {numericOnly: true},
	"avg":        {numericOnly: true},
	"mean":       {numericOnly: true},
	"min":        {numericOnly: true},
	"max":        {numericOnly: true},
	"first":      {boundary: true},
	"last":       {boundary: true},
	"difference": {numericOnly: true},
	"derivative": {numericOnly: true},
	"rate":       {numericOnly: true},
	"irate":      {numericOnly: true},
	"increase":   {numericOnly: true},
	"delta":      {numericOnly: true},
	"spread":     {numericOnly: true},
	"median":     {numericOnly: true},
	"mode":       {},
	"stddev":     {numericOnly: true},
	"stdvar":     {numericOnly: true},
	"top":        {numericOnly: true},
	"bottom":     {numericOnly: true},
}

func validateFunction(function string, field model.FieldSchema) (functionRule, error) {
	rule, ok := functionRules[function]
	if !ok || function == "" {
		return functionRule{}, newError(ErrUnsupportedFunction, "aggregate function is not supported")
	}
	if rule.numericOnly && !isNumeric(field.Type) {
		return functionRule{}, newError(ErrFunctionTypeMismatch, "aggregate function requires numeric field")
	}
	return rule, nil
}

func isNumeric(fieldType model.FieldType) bool {
	return fieldType == model.FieldFloat64 || fieldType == model.FieldInt64
}
