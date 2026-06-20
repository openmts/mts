package queryexec

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/openmts/mts/internal/model"
)

type groupAggregateColumnStream struct {
	source  ColumnStream
	specs   []model.AggregateSpec
	group   model.QueryGroup
	window  time.Duration
	pending []model.ColumnSeries
	current model.ColumnSeries
	index   int
	err     error
	loaded  bool
	closed  bool
}

type groupAccumulator struct {
	spec        model.AggregateSpec
	measurement string
	fieldName   string
	tags        map[string]string
	window      int64
	state       *incrementalAggregateState
	windows     map[int64]*incrementalAggregateState
	points      []aggregatePoint
}

func NewGroupAggregateColumnStream(
	source ColumnStream,
	specs []model.AggregateSpec,
	group model.QueryGroup,
	window time.Duration,
) ColumnStream {
	if len(group.Tags) == 0 {
		return NewAggregateColumnStream(source, specs, window)
	}
	if group.Window > 0 {
		window = group.Window
	}
	return &groupAggregateColumnStream{
		source: source,
		specs:  append([]model.AggregateSpec(nil), specs...),
		group:  group,
		window: window,
	}
}

func (s *groupAggregateColumnStream) Next() bool {
	if s.closed || s.err != nil {
		return false
	}
	if !s.loaded {
		s.load()
	}
	if s.err != nil || s.index >= len(s.pending) {
		return false
	}
	s.current = s.pending[s.index]
	s.index++
	return true
}

func (s *groupAggregateColumnStream) Column() model.ColumnSeries {
	return s.current
}

func (s *groupAggregateColumnStream) Err() error {
	return s.err
}

func (s *groupAggregateColumnStream) Close() error {
	s.closed = true
	s.pending = nil
	if s.source == nil {
		return nil
	}
	return s.source.Close()
}

func (s *groupAggregateColumnStream) load() {
	s.loaded = true
	accumulators := map[string]*groupAccumulator{}
	for s.source != nil && s.source.Next() {
		column := s.source.Column()
		if len(column.Timestamps) != len(column.Values) {
			s.err = fmt.Errorf(
				"column %s has %d timestamps and %d values",
				column.FieldName,
				len(column.Timestamps),
				len(column.Values),
			)
			return
		}
		if err := s.appendColumn(accumulators, column); err != nil {
			s.err = err
			return
		}
	}
	if s.source != nil {
		s.err = s.source.Err()
	}
	if s.err != nil {
		return
	}
	s.pending, s.err = materializeGroupAccumulators(accumulators, s.window)
}

func (s *groupAggregateColumnStream) appendColumn(
	accumulators map[string]*groupAccumulator,
	column model.ColumnSeries,
) error {
	for _, spec := range s.specs {
		if spec.Field != "" && spec.Field != column.FieldName {
			continue
		}
		spec.Function = strings.ToLower(spec.Function)
		tags := projectGroupTags(column.Tags, s.group.Tags)
		key := groupAccumulatorKey(spec, column.FieldName, s.group.Tags, tags)
		accumulator := accumulators[key]
		if accumulator == nil {
			accumulator = newGroupAccumulator(spec, column, tags, s.window)
			accumulators[key] = accumulator
		}
		for index, timestamp := range column.Timestamps {
			if err := accumulator.add(timestamp, column.Values[index]); err != nil {
				return err
			}
		}
	}
	return nil
}

func projectGroupTags(tags map[string]string, names []string) map[string]string {
	out := make(map[string]string, len(names))
	for _, name := range names {
		out[name] = tags[name]
	}
	return out
}

func groupAccumulatorKey(
	spec model.AggregateSpec,
	fieldName string,
	tagNames []string,
	tags map[string]string,
) string {
	var builder strings.Builder
	builder.WriteString(spec.Function)
	builder.WriteByte(0)
	builder.WriteString(fieldName)
	for _, name := range tagNames {
		builder.WriteByte(0)
		builder.WriteString(name)
		builder.WriteByte('=')
		builder.WriteString(tags[name])
	}
	return builder.String()
}

func materializeGroupAccumulators(
	accumulators map[string]*groupAccumulator,
	window time.Duration,
) ([]model.ColumnSeries, error) {
	keys := make([]string, 0, len(accumulators))
	for key := range accumulators {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]model.ColumnSeries, 0, len(keys))
	for _, key := range keys {
		column, err := materializeGroupAccumulator(accumulators[key], window)
		if err != nil {
			return nil, err
		}
		out = append(out, column)
	}
	return out, nil
}

func materializeGroupAccumulator(
	accumulator *groupAccumulator,
	window time.Duration,
) (model.ColumnSeries, error) {
	if accumulator.state != nil || len(accumulator.windows) > 0 {
		return accumulator.materialize()
	}
	sort.SliceStable(accumulator.points, func(i int, j int) bool {
		return accumulator.points[i].timestamp < accumulator.points[j].timestamp
	})
	out := model.ColumnSeries{
		Measurement: accumulator.measurement,
		Tags:        accumulator.tags,
		FieldName:   accumulator.spec.Function + "(" + accumulator.fieldName + ")",
	}
	if window <= 0 {
		return materializeWholeGroup(out, accumulator)
	}
	return materializeWindowedGroup(out, accumulator, int64(window))
}

func materializeWholeGroup(
	out model.ColumnSeries,
	accumulator *groupAccumulator,
) (model.ColumnSeries, error) {
	values := groupValues(accumulator.points)
	value, err := aggregateValuesByTime(
		values,
		groupTimestamps(accumulator.points),
		accumulator.spec.Function,
	)
	if err != nil {
		return model.ColumnSeries{}, err
	}
	out.FieldType = value.Type
	out.Timestamps = append(out.Timestamps, groupAggregateTimestamp(accumulator.points, accumulator.spec.Function))
	out.Values = append(out.Values, value)
	return out, nil
}

func materializeWindowedGroup(
	out model.ColumnSeries,
	accumulator *groupAccumulator,
	window int64,
) (model.ColumnSeries, error) {
	for start := 0; start < len(accumulator.points); {
		windowStart := (accumulator.points[start].timestamp / window) * window
		end := start + 1
		for end < len(accumulator.points) && accumulator.points[end].timestamp < windowStart+window {
			end++
		}
		value, err := aggregateValuesByTime(
			groupValues(accumulator.points[start:end]),
			groupTimestamps(accumulator.points[start:end]),
			accumulator.spec.Function,
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

func groupValues(points []aggregatePoint) []model.FieldValue {
	values := make([]model.FieldValue, 0, len(points))
	for _, point := range points {
		values = append(values, point.value)
	}
	return values
}

func groupTimestamps(points []aggregatePoint) []int64 {
	timestamps := make([]int64, 0, len(points))
	for _, point := range points {
		timestamps = append(timestamps, point.timestamp)
	}
	return timestamps
}

func groupAggregateTimestamp(points []aggregatePoint, fn string) int64 {
	if len(points) == 0 {
		return 0
	}
	if fn == "last" {
		return points[len(points)-1].timestamp
	}
	return points[0].timestamp
}
