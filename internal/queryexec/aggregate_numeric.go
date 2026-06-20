package queryexec

import (
	"fmt"

	"github.com/openmts/mts/internal/model"
)

func aggregateSum(values []model.FieldValue) (model.FieldValue, error) {
	switch values[0].Type {
	case model.FieldFloat64:
		var sum float64
		for _, value := range values {
			sum += value.Float64
		}
		return model.Float64Value(sum), nil
	case model.FieldInt64:
		var sum int64
		for _, value := range values {
			sum += value.Int64
		}
		return model.Int64Value(sum), nil
	default:
		return model.FieldValue{}, fmt.Errorf("sum does not support field type %d", values[0].Type)
	}
}

func aggregateAvg(values []model.FieldValue) (model.FieldValue, error) {
	sum, err := aggregateSum(values)
	if err != nil {
		return model.FieldValue{}, err
	}
	if sum.Type == model.FieldFloat64 {
		return model.Float64Value(sum.Float64 / float64(len(values))), nil
	}
	return model.Float64Value(float64(sum.Int64) / float64(len(values))), nil
}

func aggregateMinMax(values []model.FieldValue, min bool) (model.FieldValue, error) {
	switch values[0].Type {
	case model.FieldFloat64:
		return aggregateFloatMinMax(values, min), nil
	case model.FieldInt64:
		return aggregateIntMinMax(values, min), nil
	default:
		return model.FieldValue{}, fmt.Errorf("min/max does not support field type %d", values[0].Type)
	}
}

func aggregateFloatMinMax(values []model.FieldValue, min bool) model.FieldValue {
	best := values[0].Float64
	for _, value := range values[1:] {
		if (min && value.Float64 < best) || (!min && value.Float64 > best) {
			best = value.Float64
		}
	}
	return model.Float64Value(best)
}

func aggregateIntMinMax(values []model.FieldValue, min bool) model.FieldValue {
	best := values[0].Int64
	for _, value := range values[1:] {
		if (min && value.Int64 < best) || (!min && value.Int64 > best) {
			best = value.Int64
		}
	}
	return model.Int64Value(best)
}
