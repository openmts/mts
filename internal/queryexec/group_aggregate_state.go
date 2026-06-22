package queryexec

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/openmts/mts/internal/model"
)

type incrementalAggregateState struct {
	fn             string
	valueType      model.FieldType
	count          int64
	sumFloat       float64
	sumInt         int64
	mean           float64
	m2             float64
	firstTimestamp int64
	lastTimestamp  int64
	firstValue     model.FieldValue
	lastValue      model.FieldValue
	minValue       model.FieldValue
	maxValue       model.FieldValue
	modeCounts     map[string]int
	modeValue      model.FieldValue
	modeCount      int
}

func newGroupAccumulator(
	spec model.AggregateSpec,
	column model.ColumnSeries,
	tags map[string]string,
	window time.Duration,
) *groupAccumulator {
	accumulator := &groupAccumulator{
		spec:        spec,
		measurement: column.Measurement,
		fieldName:   column.FieldName,
		tags:        tags,
		window:      int64(window),
	}
	if streamableGroupAggregate(spec.Function) {
		accumulator.state = newIncrementalAggregateState(spec.Function)
		return accumulator
	}
	accumulator.points = make([]aggregatePoint, 0, len(column.Values))
	return accumulator
}

func (a *groupAccumulator) add(timestamp int64, value model.FieldValue) error {
	if a.state == nil && len(a.windows) == 0 {
		a.points = append(a.points, aggregatePoint{timestamp: timestamp, value: value})
		return nil
	}
	state := a.stateFor(timestamp)
	return state.add(timestamp, value)
}

func (a *groupAccumulator) stateFor(timestamp int64) *incrementalAggregateState {
	if a.window <= 0 {
		return a.state
	}
	windowStart := timestamp / a.window * a.window
	if a.windows == nil {
		a.windows = map[int64]*incrementalAggregateState{}
	}
	state := a.windows[windowStart]
	if state == nil {
		state = newIncrementalAggregateState(a.spec.Function)
		a.windows[windowStart] = state
	}
	return state
}

func (a *groupAccumulator) materialize() (model.ColumnSeries, error) {
	out := model.ColumnSeries{
		Measurement: a.measurement,
		Tags:        a.tags,
		FieldName:   a.spec.Function + "(" + a.fieldName + ")",
	}
	if a.window <= 0 {
		value, timestamp, err := a.state.value()
		if err != nil {
			return model.ColumnSeries{}, err
		}
		out.FieldType = value.Type
		out.Timestamps = append(out.Timestamps, timestamp)
		out.Values = append(out.Values, value)
		return out, nil
	}
	return a.materializeWindows(out)
}

func (a *groupAccumulator) materializeWindows(out model.ColumnSeries) (model.ColumnSeries, error) {
	windows := make([]int64, 0, len(a.windows))
	for windowStart := range a.windows {
		windows = append(windows, windowStart)
	}
	sort.Slice(windows, func(i int, j int) bool {
		return windows[i] < windows[j]
	})
	for _, windowStart := range windows {
		value, _, err := a.windows[windowStart].value()
		if err != nil {
			return model.ColumnSeries{}, err
		}
		out.FieldType = value.Type
		out.Timestamps = append(out.Timestamps, windowStart)
		out.Values = append(out.Values, value)
	}
	return out, nil
}

func streamableGroupAggregate(fn string) bool {
	switch fn {
	case "count", "sum", "avg", "min", "max",
		"first", "last", "spread", "mode", "stddev", "stdvar", "top", "bottom":
		return true
	default:
		return false
	}
}

func newIncrementalAggregateState(fn string) *incrementalAggregateState {
	return &incrementalAggregateState{fn: fn}
}

func (s *incrementalAggregateState) add(timestamp int64, value model.FieldValue) error {
	if s.count == 0 {
		if s.requiresNumeric() && !isNumericFieldValue(value) {
			return fmt.Errorf("aggregate %q requires numeric values", s.fn)
		}
		s.initialize(timestamp, value)
		return nil
	}
	if err := s.validate(value); err != nil {
		return err
	}
	s.count++
	s.addByFunction(timestamp, value)
	return nil
}

func (s *incrementalAggregateState) initialize(timestamp int64, value model.FieldValue) {
	s.valueType = value.Type
	s.count = 1
	s.firstTimestamp = timestamp
	s.lastTimestamp = timestamp
	s.firstValue = value
	s.lastValue = value
	s.minValue = value
	s.maxValue = value
	s.addNumeric(value)
	s.addMode(value)
}

func (s *incrementalAggregateState) validate(value model.FieldValue) error {
	if value.Type != s.valueType {
		return fmt.Errorf("aggregate %q received mixed field types", s.fn)
	}
	if s.requiresNumeric() && !isNumericFieldValue(value) {
		return fmt.Errorf("aggregate %q requires numeric values", s.fn)
	}
	return nil
}

func (s *incrementalAggregateState) addByFunction(timestamp int64, value model.FieldValue) {
	s.addNumeric(value)
	s.addMinMax(value)
	s.addFirstLast(timestamp, value)
	s.addMode(value)
}

func (s *incrementalAggregateState) addNumeric(value model.FieldValue) {
	if !isNumericFieldValue(value) {
		return
	}
	number := fieldValueAsFloat(value)
	s.sumFloat += number
	if value.Type == model.FieldInt64 {
		s.sumInt += value.Int64
	}
	delta := number - s.mean
	s.mean += delta / float64(s.count)
	s.m2 += delta * (number - s.mean)
}

func (s *incrementalAggregateState) addMinMax(value model.FieldValue) {
	if fieldValuesCompare(value, s.minValue) < 0 {
		s.minValue = value
	}
	if fieldValuesCompare(value, s.maxValue) > 0 {
		s.maxValue = value
	}
}

func (s *incrementalAggregateState) addFirstLast(timestamp int64, value model.FieldValue) {
	if timestamp < s.firstTimestamp {
		s.firstTimestamp = timestamp
		s.firstValue = value
	}
	if timestamp >= s.lastTimestamp {
		s.lastTimestamp = timestamp
		s.lastValue = value
	}
}

func (s *incrementalAggregateState) addMode(value model.FieldValue) {
	if s.fn != "mode" {
		return
	}
	if s.modeCounts == nil {
		s.modeCounts = map[string]int{}
	}
	key := fieldValueKey(value)
	s.modeCounts[key]++
	if s.modeCounts[key] > s.modeCount {
		s.modeCount = s.modeCounts[key]
		s.modeValue = value
	}
}

func (s *incrementalAggregateState) value() (model.FieldValue, int64, error) {
	if s.count == 0 {
		return model.FieldValue{}, 0, fmt.Errorf("aggregate %q has no values", s.fn)
	}
	value, err := s.aggregateValue()
	if err != nil {
		return model.FieldValue{}, 0, err
	}
	return value, s.outputTimestamp(), nil
}

func (s *incrementalAggregateState) aggregateValue() (model.FieldValue, error) {
	switch s.fn {
	case "count":
		return model.Int64Value(s.count), nil
	case "sum":
		return s.sumValue(), nil
	case "avg":
		return model.Float64Value(s.sumFloat / float64(s.count)), nil
	case "min":
		return s.minValue, s.numericError("min")
	case "max":
		return s.maxValue, s.numericError("max")
	case "top":
		return s.maxValue, s.numericError("top")
	case "bottom":
		return s.minValue, s.numericError("bottom")
	case "first":
		return s.firstValue, nil
	case "last":
		return s.lastValue, nil
	case "spread":
		return s.spreadValue()
	case "mode":
		return s.modeValue, nil
	case "stddev", "stdvar":
		return s.stdValue(), nil
	default:
		return model.FieldValue{}, fmt.Errorf("%w: %q", ErrUnsupportedAggregate, s.fn)
	}
}

func (s *incrementalAggregateState) sumValue() model.FieldValue {
	if s.valueType == model.FieldInt64 {
		return model.Int64Value(s.sumInt)
	}
	return model.Float64Value(s.sumFloat)
}

func (s *incrementalAggregateState) spreadValue() (model.FieldValue, error) {
	if err := s.numericError("spread"); err != nil {
		return model.FieldValue{}, err
	}
	if s.valueType == model.FieldInt64 {
		return model.Int64Value(s.maxValue.Int64 - s.minValue.Int64), nil
	}
	return model.Float64Value(s.maxValue.Float64 - s.minValue.Float64), nil
}

func (s *incrementalAggregateState) stdValue() model.FieldValue {
	variance := s.m2 / float64(s.count)
	if s.fn == "stddev" {
		return model.Float64Value(math.Sqrt(variance))
	}
	return model.Float64Value(variance)
}

func (s *incrementalAggregateState) numericError(fn string) error {
	if isNumericFieldValue(s.minValue) {
		return nil
	}
	return fmt.Errorf("%s does not support field type %d", fn, s.valueType)
}

func (s *incrementalAggregateState) requiresNumeric() bool {
	switch s.fn {
	case "sum", "avg", "min", "max", "spread", "stddev", "stdvar", "top", "bottom":
		return true
	default:
		return false
	}
}

func (s *incrementalAggregateState) outputTimestamp() int64 {
	if s.fn == "last" {
		return s.lastTimestamp
	}
	return s.firstTimestamp
}
