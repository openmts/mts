package engine

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/openmts/mts/internal/model"
)

type downsampleWindowAggregator struct {
	policy       model.DownsamplePolicy
	start        int64
	end          int64
	functions    map[string][]model.DownsampleFunction
	points       map[string]*downsamplePointState
	outputFields []string
}

type downsamplePointState struct {
	tags      map[string]string
	timestamp int64
	fields    map[string]*downsampleAggregateState
}

type downsampleAggregateState struct {
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
	series         []downsampleSample
}

type downsampleSample struct {
	timestamp int64
	value     model.FieldValue
}

func newDownsampleWindowAggregator(
	policy model.DownsamplePolicy,
	start int64,
	end int64,
) *downsampleWindowAggregator {
	return &downsampleWindowAggregator{
		policy:       policy,
		start:        start,
		end:          end,
		functions:    downsampleFunctionsByField(policy.Functions),
		points:       map[string]*downsamplePointState{},
		outputFields: downsampleOrderedOutputFields(policy.Functions),
	}
}

func downsampleFunctionsByField(
	functions []model.DownsampleFunction,
) map[string][]model.DownsampleFunction {
	out := make(map[string][]model.DownsampleFunction, len(functions))
	for _, function := range functions {
		out[function.Field] = append(out[function.Field], function)
	}
	return out
}

func downsampleOrderedOutputFields(functions []model.DownsampleFunction) []string {
	out := make([]string, 0, len(functions))
	for _, function := range functions {
		out = append(out, function.As)
	}
	return out
}

func (a *downsampleWindowAggregator) addColumn(column model.ColumnSeries) error {
	functions := a.functions[column.FieldName]
	if len(functions) == 0 {
		return nil
	}
	if len(column.Timestamps) != len(column.Values) {
		return fmt.Errorf("downsample column %q has mismatched samples", column.FieldName)
	}
	state := a.pointStateFor(column.Tags)
	for index, timestamp := range column.Timestamps {
		if timestamp < a.start || timestamp >= a.end {
			continue
		}
		if err := state.add(functions, timestamp, column.Values[index]); err != nil {
			return err
		}
	}
	return nil
}

func (a *downsampleWindowAggregator) addIterator(iter *columnIterator) error {
	if iter == nil {
		return nil
	}
	defer func() {
		_ = iter.Close()
	}()
	for iter.Next() {
		if err := a.addColumn(iter.Column()); err != nil {
			return err
		}
	}
	return iter.Err()
}

func (a *downsampleWindowAggregator) pointStateFor(tags map[string]string) *downsamplePointState {
	groupTags := downsampleGroupTags(tags, a.policy.GroupByTags)
	targetTags := downsampleTargetTags(groupTags, a.policy)
	key := downsamplePointKey(targetTags, a.start)
	state := a.points[key]
	if state != nil {
		return state
	}
	state = &downsamplePointState{
		tags:      targetTags,
		timestamp: a.start,
		fields:    make(map[string]*downsampleAggregateState, len(a.policy.Functions)),
	}
	a.points[key] = state
	return state
}

func (s *downsamplePointState) add(
	functions []model.DownsampleFunction,
	timestamp int64,
	value model.FieldValue,
) error {
	for _, function := range functions {
		state := s.fields[function.As]
		if state == nil {
			state = &downsampleAggregateState{fn: function.Function}
			s.fields[function.As] = state
		}
		if err := state.add(timestamp, value); err != nil {
			return err
		}
	}
	return nil
}

func (a *downsampleWindowAggregator) write(
	ctx context.Context,
	engine *Engine,
	batchSize int,
) (int, error) {
	if batchSize <= 0 {
		batchSize = defaultDownsampleBatchSize
	}
	keys := a.sortedPointKeys()
	batch := make([]model.Point, 0, minInt(len(keys), batchSize))
	written := 0
	for _, key := range keys {
		point, ok, err := a.points[key].point(a.policy, a.outputFields)
		if err != nil {
			return written, err
		}
		if !ok {
			continue
		}
		batch = append(batch, point)
		if len(batch) < batchSize {
			continue
		}
		if err := engine.Write(ctx, batch, model.WriteOptions{}); err != nil {
			return written, err
		}
		written += len(batch)
		batch = batch[:0]
	}
	if len(batch) == 0 {
		return written, nil
	}
	if err := engine.Write(ctx, batch, model.WriteOptions{}); err != nil {
		return written, err
	}
	return written + len(batch), nil
}

func (a *downsampleWindowAggregator) sortedPointKeys() []string {
	keys := make([]string, 0, len(a.points))
	for key := range a.points {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (s *downsamplePointState) point(
	policy model.DownsamplePolicy,
	outputFields []string,
) (model.Point, bool, error) {
	fields := make(map[string]model.FieldValue, len(s.fields))
	for _, name := range outputFields {
		state := s.fields[name]
		if state == nil || state.count == 0 {
			continue
		}
		value, err := state.value()
		if err != nil {
			return model.Point{}, false, err
		}
		fields[name] = value
	}
	if len(fields) == 0 {
		return model.Point{}, false, nil
	}
	return model.Point{
		Database:        policy.TargetDatabase,
		RetentionPolicy: policy.TargetRetention,
		Measurement:     policy.TargetMeasurement,
		Tags:            cloneDownsampleTags(s.tags),
		Timestamp:       s.timestamp,
		Fields:          fields,
	}, true, nil
}

func (s *downsampleAggregateState) add(timestamp int64, value model.FieldValue) error {
	if s.count == 0 {
		return s.initialize(timestamp, value)
	}
	if err := s.validate(value); err != nil {
		return err
	}
	s.count++
	s.addValue(timestamp, value)
	return nil
}

func (s *downsampleAggregateState) initialize(timestamp int64, value model.FieldValue) error {
	if s.requiresNumeric() && !downsampleNumericValue(value) {
		return fmt.Errorf("aggregate %q requires numeric values", s.fn)
	}
	s.valueType = value.Type
	s.count = 1
	s.firstTimestamp = timestamp
	s.lastTimestamp = timestamp
	s.firstValue = value
	s.lastValue = value
	s.minValue = value
	s.maxValue = value
	s.addValue(timestamp, value)
	return nil
}

func (s *downsampleAggregateState) validate(value model.FieldValue) error {
	if value.Type != s.valueType {
		return fmt.Errorf("aggregate %q received mixed field types", s.fn)
	}
	if s.requiresNumeric() && !downsampleNumericValue(value) {
		return fmt.Errorf("aggregate %q requires numeric values", s.fn)
	}
	return nil
}

func (s *downsampleAggregateState) addValue(timestamp int64, value model.FieldValue) {
	s.addNumeric(value)
	s.addMinMax(value)
	s.addFirstLast(timestamp, value)
	s.addMode(value)
	if s.requiresSeries() {
		s.series = append(s.series, downsampleSample{timestamp: timestamp, value: value})
	}
}

func (s *downsampleAggregateState) addNumeric(value model.FieldValue) {
	if !downsampleNumericValue(value) {
		return
	}
	number := downsampleValueAsFloat(value)
	s.sumFloat += number
	if value.Type == model.FieldInt64 {
		s.sumInt += value.Int64
	}
	delta := number - s.mean
	s.mean += delta / float64(s.count)
	s.m2 += delta * (number - s.mean)
}

func (s *downsampleAggregateState) addMinMax(value model.FieldValue) {
	if s.count == 1 || downsampleFieldLess(value, s.minValue) {
		s.minValue = value
	}
	if s.count == 1 || downsampleFieldLess(s.maxValue, value) {
		s.maxValue = value
	}
}

func (s *downsampleAggregateState) addFirstLast(timestamp int64, value model.FieldValue) {
	if timestamp < s.firstTimestamp {
		s.firstTimestamp = timestamp
		s.firstValue = value
	}
	if timestamp >= s.lastTimestamp {
		s.lastTimestamp = timestamp
		s.lastValue = value
	}
}

func (s *downsampleAggregateState) addMode(value model.FieldValue) {
	if s.fn != "mode" {
		return
	}
	if s.modeCounts == nil {
		s.modeCounts = map[string]int{}
	}
	key := downsampleFieldValueKey(value)
	s.modeCounts[key]++
	if s.modeCounts[key] > s.modeCount {
		s.modeCount = s.modeCounts[key]
		s.modeValue = value
	}
}

func (s *downsampleAggregateState) value() (model.FieldValue, error) {
	switch s.fn {
	case "count":
		return model.Int64Value(s.count), nil
	case "sum":
		return s.sumValue(), nil
	case "avg":
		return model.Float64Value(s.sumFloat / float64(s.count)), nil
	case "min", "bottom":
		return s.minValue, nil
	case "max", "top":
		return s.maxValue, nil
	case "first":
		return s.firstValue, nil
	case "last":
		return s.lastValue, nil
	case "spread":
		return s.spreadValue(), nil
	case "mode":
		return s.modeValue, nil
	case "stddev", "stdvar":
		return s.stdValue(), nil
	default:
		return s.seriesValue()
	}
}

func (s *downsampleAggregateState) sumValue() model.FieldValue {
	if s.valueType == model.FieldInt64 {
		return model.Int64Value(s.sumInt)
	}
	return model.Float64Value(s.sumFloat)
}

func (s *downsampleAggregateState) spreadValue() model.FieldValue {
	if s.valueType == model.FieldInt64 {
		return model.Int64Value(s.maxValue.Int64 - s.minValue.Int64)
	}
	return model.Float64Value(s.maxValue.Float64 - s.minValue.Float64)
}

func (s *downsampleAggregateState) stdValue() model.FieldValue {
	variance := s.m2 / float64(s.count)
	if s.fn == "stddev" {
		return model.Float64Value(math.Sqrt(variance))
	}
	return model.Float64Value(variance)
}

func (s *downsampleAggregateState) seriesValue() (model.FieldValue, error) {
	samples := s.sortedSeries()
	if len(samples) < 2 {
		return model.FieldValue{}, fmt.Errorf("aggregate %q requires at least two values", s.fn)
	}
	switch s.fn {
	case "rate":
		return downsampleRate(samples, false)
	case "irate":
		return downsampleRate(samples[len(samples)-2:], false)
	case "increase":
		return downsampleIncreaseValue(samples)
	case "delta", "difference":
		return downsampleDeltaValue(samples)
	case "derivative":
		return downsampleDerivativeValue(samples)
	case "median":
		return downsampleMedianValue(samples)
	default:
		return model.FieldValue{}, fmt.Errorf("unsupported aggregate function %q", s.fn)
	}
}

func (s *downsampleAggregateState) sortedSeries() []downsampleSample {
	samples := append([]downsampleSample(nil), s.series...)
	sort.SliceStable(samples, func(i int, j int) bool {
		return samples[i].timestamp < samples[j].timestamp
	})
	return samples
}

func (s *downsampleAggregateState) requiresNumeric() bool {
	switch s.fn {
	case "sum", "avg", "min", "max", "spread", "stddev", "stdvar",
		"top", "bottom", "rate", "irate", "increase", "delta",
		"difference", "derivative", "median":
		return true
	default:
		return false
	}
}

func (s *downsampleAggregateState) requiresSeries() bool {
	switch s.fn {
	case "rate", "irate", "increase", "delta", "difference", "derivative", "median":
		return true
	default:
		return false
	}
}

func downsampleRate(samples []downsampleSample, _ bool) (model.FieldValue, error) {
	increase, err := downsampleCounterIncrease(samples)
	if err != nil {
		return model.FieldValue{}, err
	}
	seconds := float64(samples[len(samples)-1].timestamp-samples[0].timestamp) / float64(time.Second)
	if seconds <= 0 {
		return model.FieldValue{}, fmt.Errorf("rate requires increasing timestamps")
	}
	return model.Float64Value(increase / seconds), nil
}

func downsampleIncreaseValue(samples []downsampleSample) (model.FieldValue, error) {
	increase, err := downsampleCounterIncrease(samples)
	if err != nil {
		return model.FieldValue{}, err
	}
	return model.Float64Value(increase), nil
}

func downsampleCounterIncrease(samples []downsampleSample) (float64, error) {
	var total float64
	for index := 1; index < len(samples); index++ {
		previous := downsampleValueAsFloat(samples[index-1].value)
		current := downsampleValueAsFloat(samples[index].value)
		if current >= previous {
			total += current - previous
		} else {
			total += current
		}
	}
	return total, nil
}

func downsampleDeltaValue(samples []downsampleSample) (model.FieldValue, error) {
	first := samples[0].value
	last := samples[len(samples)-1].value
	if first.Type == model.FieldInt64 && last.Type == model.FieldInt64 {
		return model.Int64Value(last.Int64 - first.Int64), nil
	}
	return model.Float64Value(downsampleValueAsFloat(last) - downsampleValueAsFloat(first)), nil
}

func downsampleDerivativeValue(samples []downsampleSample) (model.FieldValue, error) {
	delta, err := downsampleDeltaValue(samples)
	if err != nil {
		return model.FieldValue{}, err
	}
	seconds := float64(samples[len(samples)-1].timestamp-samples[0].timestamp) / float64(time.Second)
	if seconds <= 0 {
		return model.FieldValue{}, fmt.Errorf("derivative requires increasing timestamps")
	}
	return model.Float64Value(downsampleValueAsFloat(delta) / seconds), nil
}

func downsampleMedianValue(samples []downsampleSample) (model.FieldValue, error) {
	values := make([]float64, 0, len(samples))
	for _, sample := range samples {
		values = append(values, downsampleValueAsFloat(sample.value))
	}
	sort.Float64s(values)
	mid := len(values) / 2
	if len(values)%2 == 1 {
		return model.Float64Value(values[mid]), nil
	}
	return model.Float64Value((values[mid-1] + values[mid]) / 2), nil
}

func downsampleGroupTags(tags map[string]string, names []string) map[string]string {
	if len(names) == 0 {
		return nil
	}
	out := make(map[string]string, len(names))
	for _, name := range names {
		out[name] = tags[name]
	}
	return out
}

func downsampleNumericValue(value model.FieldValue) bool {
	return value.Type == model.FieldFloat64 || value.Type == model.FieldInt64
}

func downsampleValueAsFloat(value model.FieldValue) float64 {
	if value.Type == model.FieldInt64 {
		return float64(value.Int64)
	}
	return value.Float64
}

func downsampleFieldLess(left model.FieldValue, right model.FieldValue) bool {
	if downsampleNumericValue(left) && downsampleNumericValue(right) {
		return downsampleValueAsFloat(left) < downsampleValueAsFloat(right)
	}
	return downsampleFieldValueKey(left) < downsampleFieldValueKey(right)
}

func downsampleFieldValueKey(value model.FieldValue) string {
	switch value.Type {
	case model.FieldFloat64:
		return "f:" + strconv.FormatFloat(value.Float64, 'g', -1, 64)
	case model.FieldInt64:
		return "i:" + strconv.FormatInt(value.Int64, 10)
	case model.FieldString:
		return "s:" + value.String
	case model.FieldBool:
		return "b:" + strconv.FormatBool(value.Bool)
	default:
		return "unknown"
	}
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func estimateDownsampleGroupKey(tags map[string]string, names []string) string {
	groupTags := downsampleGroupTags(tags, names)
	if len(groupTags) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, name := range sortedTagNames(groupTags) {
		builder.WriteString(name)
		builder.WriteByte('=')
		builder.WriteString(groupTags[name])
		builder.WriteByte(0)
	}
	return builder.String()
}
