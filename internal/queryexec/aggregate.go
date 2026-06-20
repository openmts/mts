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
		if transformAggregate(fn) {
			return aggregateTransformColumn(out, column, fn)
		}
		return aggregateWholeColumn(out, column, fn)
	}
	return aggregateWindowedColumn(out, column, fn, int64(window))
}

func aggregateWholeColumn(
	out model.ColumnSeries,
	column model.ColumnSeries,
	fn string,
) (model.ColumnSeries, error) {
	value, err := aggregateValuesByTime(column.Values, column.Timestamps, fn)
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
		value, err := aggregateValuesByTime(
			column.Values[start:end],
			column.Timestamps[start:end],
			fn,
		)
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

func aggregateValuesByTime(
	values []model.FieldValue,
	timestamps []int64,
	fn string,
) (model.FieldValue, error) {
	switch fn {
	case "rate":
		return aggregateRate(values, timestamps)
	case "irate":
		return aggregateIRate(values, timestamps)
	case "increase":
		return aggregateIncrease(values)
	case "delta":
		return aggregateDelta(values)
	default:
		return aggregateValues(values, fn)
	}
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
	case "rate":
		return aggregateRate(values, nil)
	case "irate":
		return aggregateIRate(values, nil)
	case "increase":
		return aggregateIncrease(values)
	case "delta":
		return aggregateDelta(values)
	case "spread":
		return aggregateSpread(values)
	case "median":
		return aggregateMedian(values)
	case "mode":
		return aggregateMode(values)
	case "stddev", "stdvar":
		return aggregateStd(values, fn == "stddev")
	case "top":
		return aggregateMinMax(values, false)
	case "bottom":
		return aggregateMinMax(values, true)
	default:
		return model.FieldValue{}, fmt.Errorf("unsupported aggregate function %q", fn)
	}
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
