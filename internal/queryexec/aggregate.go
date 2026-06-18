package queryexec

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/openmts/mts/internal/model"
)

type aggregateColumnStream struct {
	source  ColumnStream
	specs   []model.AggregateSpec
	window  time.Duration
	pending []model.ColumnSeries
	current model.ColumnSeries
	err     error
	index   int
	closed  bool
}

func NewAggregateColumnStream(
	source ColumnStream,
	specs []model.AggregateSpec,
	window ...time.Duration,
) ColumnStream {
	duration := time.Duration(0)
	if len(window) > 0 {
		duration = window[0]
	}
	return &aggregateColumnStream{
		source: source,
		specs:  append([]model.AggregateSpec(nil), specs...),
		window: duration,
	}
}

func (s *aggregateColumnStream) Next() bool {
	if s.closed || s.err != nil {
		return false
	}
	for {
		if s.nextPending() {
			return true
		}
		if !s.source.Next() {
			s.err = s.source.Err()
			return false
		}
		s.pending, s.err = aggregateColumn(s.source.Column(), s.specs, s.window)
		s.index = 0
		if s.err != nil {
			return false
		}
	}
}

func (s *aggregateColumnStream) Column() model.ColumnSeries {
	return s.current
}

func (s *aggregateColumnStream) Err() error {
	return s.err
}

func (s *aggregateColumnStream) Close() error {
	s.closed = true
	s.pending = nil
	if s.source == nil {
		return nil
	}
	return s.source.Close()
}

func (s *aggregateColumnStream) nextPending() bool {
	if s.index >= len(s.pending) {
		return false
	}
	s.current = s.pending[s.index]
	s.index++
	return true
}

func aggregateColumn(
	column model.ColumnSeries,
	specs []model.AggregateSpec,
	window time.Duration,
) ([]model.ColumnSeries, error) {
	normalized, err := normalizeAggregateColumn(column)
	if err != nil {
		return nil, err
	}
	out := make([]model.ColumnSeries, 0, len(specs))
	for _, spec := range specs {
		if spec.Field != "" && spec.Field != normalized.FieldName {
			continue
		}
		aggregated, err := aggregateColumnBySpec(normalized, spec, window)
		if err != nil {
			return nil, err
		}
		out = append(out, aggregated)
	}
	return out, nil
}

type aggregatePoint struct {
	timestamp int64
	value     model.FieldValue
}

func normalizeAggregateColumn(column model.ColumnSeries) (model.ColumnSeries, error) {
	if len(column.Timestamps) != len(column.Values) {
		return model.ColumnSeries{}, fmt.Errorf("column %s has %d timestamps and %d values", column.FieldName, len(column.Timestamps), len(column.Values))
	}
	if aggregateColumnOrdered(column) {
		return column, nil
	}
	points := make([]aggregatePoint, len(column.Values))
	for index, timestamp := range column.Timestamps {
		points[index] = aggregatePoint{timestamp: timestamp, value: column.Values[index]}
	}
	sort.SliceStable(points, func(i int, j int) bool {
		return points[i].timestamp < points[j].timestamp
	})
	column.Timestamps = column.Timestamps[:0]
	column.Values = column.Values[:0]
	for _, point := range points {
		if len(column.Timestamps) > 0 && column.Timestamps[len(column.Timestamps)-1] == point.timestamp {
			column.Values[len(column.Values)-1] = point.value
			continue
		}
		column.Timestamps = append(column.Timestamps, point.timestamp)
		column.Values = append(column.Values, point.value)
	}
	return column, nil
}

func aggregateColumnOrdered(column model.ColumnSeries) bool {
	for index := 1; index < len(column.Timestamps); index++ {
		if column.Timestamps[index-1] >= column.Timestamps[index] {
			return false
		}
	}
	return true
}

func aggregateColumnBySpec(
	column model.ColumnSeries,
	spec model.AggregateSpec,
	window time.Duration,
) (model.ColumnSeries, error) {
	fn := strings.ToLower(spec.Function)
	if fn == "" {
		return model.ColumnSeries{}, fmt.Errorf("aggregate function is empty")
	}
	out := model.ColumnSeries{
		SeriesID:    column.SeriesID,
		Measurement: column.Measurement,
		Tags:        column.Tags,
		FieldID:     column.FieldID,
		FieldName:   fn + "(" + column.FieldName + ")",
	}
	if window <= 0 {
		return aggregateWholeColumn(out, column, fn)
	}
	return aggregateWindowedColumn(out, column, fn, int64(window))
}

func aggregateWholeColumn(
	out model.ColumnSeries,
	column model.ColumnSeries,
	fn string,
) (model.ColumnSeries, error) {
	value, err := aggregateValues(column.Values, fn)
	if err != nil {
		return model.ColumnSeries{}, err
	}
	out.FieldType = value.Type
	out.Timestamps = append(out.Timestamps, aggregateTimestamp(column, fn))
	out.Values = append(out.Values, value)
	return out, nil
}

func aggregateWindowedColumn(
	out model.ColumnSeries,
	column model.ColumnSeries,
	fn string,
	window int64,
) (model.ColumnSeries, error) {
	for start := 0; start < len(column.Timestamps); {
		windowStart := (column.Timestamps[start] / window) * window
		end := start + 1
		for end < len(column.Timestamps) && column.Timestamps[end] < windowStart+window {
			end++
		}
		value, err := aggregateValues(column.Values[start:end], fn)
		if err != nil {
			return model.ColumnSeries{}, err
		}
		out.FieldType = value.Type
		out.Timestamps = append(out.Timestamps, windowStart)
		out.Values = append(out.Values, value)
		start = end
	}
	return out, nil
}

func aggregateValues(values []model.FieldValue, fn string) (model.FieldValue, error) {
	if len(values) == 0 {
		return model.FieldValue{}, fmt.Errorf("aggregate %q has no values", fn)
	}
	switch fn {
	case "count":
		return model.Int64Value(int64(len(values))), nil
	case "sum":
		return aggregateSum(values)
	case "avg":
		return aggregateAvg(values)
	case "min", "max":
		return aggregateMinMax(values, fn == "min")
	case "first":
		return values[0], nil
	case "last":
		return values[len(values)-1], nil
	default:
		return model.FieldValue{}, fmt.Errorf("unsupported aggregate function %q", fn)
	}
}

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

func aggregateTimestamp(column model.ColumnSeries, fn string) int64 {
	if len(column.Timestamps) == 0 {
		return 0
	}
	if fn == "last" {
		return column.Timestamps[len(column.Timestamps)-1]
	}
	return column.Timestamps[0]
}
