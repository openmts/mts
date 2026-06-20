package queryexec

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/openmts/mts/internal/model"
)

func transformAggregate(fn string) bool {
	return fn == "difference" || fn == "derivative"
}

func aggregateTransformColumn(
	out model.ColumnSeries,
	column model.ColumnSeries,
	fn string,
) (model.ColumnSeries, error) {
	out.FieldType = model.FieldFloat64
	if fn == "difference" && allIntValues(column.Values) {
		out.FieldType = model.FieldInt64
	}
	for index := 1; index < len(column.Values); index++ {
		value, ok, err := transformValue(
			column.Values[index-1],
			column.Values[index],
			column.Timestamps[index-1],
			column.Timestamps[index],
			fn,
		)
		if err != nil {
			return model.ColumnSeries{}, err
		}
		if !ok {
			continue
		}
		out.Timestamps = append(out.Timestamps, column.Timestamps[index])
		out.Values = append(out.Values, value)
	}
	return out, nil
}

func transformValue(
	previous model.FieldValue,
	current model.FieldValue,
	previousTime int64,
	currentTime int64,
	fn string,
) (model.FieldValue, bool, error) {
	if !isNumericFieldValue(previous) || !isNumericFieldValue(current) {
		return model.FieldValue{}, false, fmt.Errorf("%s requires numeric values", fn)
	}
	diff := fieldValueAsFloat(current) - fieldValueAsFloat(previous)
	if fn == "difference" {
		if previous.Type == model.FieldInt64 && current.Type == model.FieldInt64 {
			return model.Int64Value(current.Int64 - previous.Int64), true, nil
		}
		return model.Float64Value(diff), true, nil
	}
	seconds := float64(currentTime-previousTime) / float64(time.Second)
	if seconds <= 0 {
		return model.FieldValue{}, false, nil
	}
	return model.Float64Value(diff / seconds), true, nil
}

func aggregateRate(values []model.FieldValue, timestamps []int64) (model.FieldValue, error) {
	if len(values) < 2 {
		return model.FieldValue{}, fmt.Errorf("rate requires at least two values")
	}
	increase, err := counterIncrease(values)
	if err != nil {
		return model.FieldValue{}, err
	}
	seconds := float64(len(values) - 1)
	if len(timestamps) == len(values) {
		seconds = float64(timestamps[len(timestamps)-1]-timestamps[0]) / float64(time.Second)
	}
	if seconds <= 0 {
		return model.FieldValue{}, fmt.Errorf("rate requires increasing timestamps")
	}
	return model.Float64Value(increase / seconds), nil
}

func aggregateIRate(values []model.FieldValue, timestamps []int64) (model.FieldValue, error) {
	if len(values) < 2 {
		return model.FieldValue{}, fmt.Errorf("irate requires at least two values")
	}
	last := len(values) - 1
	increase, err := counterPairIncrease(values[last-1], values[last])
	if err != nil {
		return model.FieldValue{}, err
	}
	seconds := 1.0
	if len(timestamps) == len(values) {
		seconds = float64(timestamps[last]-timestamps[last-1]) / float64(time.Second)
	}
	if seconds <= 0 {
		return model.FieldValue{}, fmt.Errorf("irate requires increasing timestamps")
	}
	return model.Float64Value(increase / seconds), nil
}

func aggregateIncrease(values []model.FieldValue) (model.FieldValue, error) {
	if len(values) < 2 {
		return model.FieldValue{}, fmt.Errorf("increase requires at least two values")
	}
	increase, err := counterIncrease(values)
	if err != nil {
		return model.FieldValue{}, err
	}
	return model.Float64Value(increase), nil
}

func aggregateDelta(values []model.FieldValue) (model.FieldValue, error) {
	if len(values) < 2 {
		return model.FieldValue{}, fmt.Errorf("delta requires at least two values")
	}
	first := values[0]
	last := values[len(values)-1]
	if !isNumericFieldValue(first) || !isNumericFieldValue(last) {
		return model.FieldValue{}, fmt.Errorf("delta requires numeric values")
	}
	if first.Type == model.FieldInt64 && last.Type == model.FieldInt64 {
		return model.Int64Value(last.Int64 - first.Int64), nil
	}
	return model.Float64Value(fieldValueAsFloat(last) - fieldValueAsFloat(first)), nil
}

func counterIncrease(values []model.FieldValue) (float64, error) {
	var total float64
	for index := 1; index < len(values); index++ {
		increase, err := counterPairIncrease(values[index-1], values[index])
		if err != nil {
			return 0, err
		}
		total += increase
	}
	return total, nil
}

func counterPairIncrease(previous model.FieldValue, current model.FieldValue) (float64, error) {
	if !isNumericFieldValue(previous) || !isNumericFieldValue(current) {
		return 0, fmt.Errorf("rate requires numeric values")
	}
	left := fieldValueAsFloat(previous)
	right := fieldValueAsFloat(current)
	if right >= left {
		return right - left, nil
	}
	return right, nil
}

func aggregateSpread(values []model.FieldValue) (model.FieldValue, error) {
	minValue, err := aggregateMinMax(values, true)
	if err != nil {
		return model.FieldValue{}, err
	}
	maxValue, err := aggregateMinMax(values, false)
	if err != nil {
		return model.FieldValue{}, err
	}
	if minValue.Type == model.FieldInt64 && maxValue.Type == model.FieldInt64 {
		return model.Int64Value(maxValue.Int64 - minValue.Int64), nil
	}
	return model.Float64Value(fieldValueAsFloat(maxValue) - fieldValueAsFloat(minValue)), nil
}

func aggregateMedian(values []model.FieldValue) (model.FieldValue, error) {
	if !numericValues(values) {
		return model.FieldValue{}, fmt.Errorf("median requires numeric values")
	}
	sorted := make([]float64, 0, len(values))
	for _, value := range values {
		sorted = append(sorted, fieldValueAsFloat(value))
	}
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return model.Float64Value(sorted[mid]), nil
	}
	return model.Float64Value((sorted[mid-1] + sorted[mid]) / 2), nil
}

func aggregateMode(values []model.FieldValue) (model.FieldValue, error) {
	counts := make(map[string]int, len(values))
	best := values[0]
	bestCount := 0
	for _, value := range values {
		key := fieldValueKey(value)
		counts[key]++
		if counts[key] > bestCount {
			best = value
			bestCount = counts[key]
		}
	}
	return best, nil
}

func aggregateStd(values []model.FieldValue, sqrt bool) (model.FieldValue, error) {
	if !numericValues(values) {
		return model.FieldValue{}, fmt.Errorf("stddev/stdvar requires numeric values")
	}
	mean, err := aggregateAvg(values)
	if err != nil {
		return model.FieldValue{}, err
	}
	var variance float64
	for _, value := range values {
		delta := fieldValueAsFloat(value) - mean.Float64
		variance += delta * delta
	}
	variance /= float64(len(values))
	if sqrt {
		return model.Float64Value(math.Sqrt(variance)), nil
	}
	return model.Float64Value(variance), nil
}

func numericValues(values []model.FieldValue) bool {
	for _, value := range values {
		if !isNumericFieldValue(value) {
			return false
		}
	}
	return true
}

func allIntValues(values []model.FieldValue) bool {
	for _, value := range values {
		if value.Type != model.FieldInt64 {
			return false
		}
	}
	return true
}

func fieldValueKey(value model.FieldValue) string {
	switch value.Type {
	case model.FieldFloat64:
		return fmt.Sprintf("f:%g", value.Float64)
	case model.FieldInt64:
		return fmt.Sprintf("i:%d", value.Int64)
	case model.FieldString:
		return "s:" + value.String
	case model.FieldBool:
		if value.Bool {
			return "b:true"
		}
		return "b:false"
	default:
		return "unknown"
	}
}
