package queryexec

import (
	"errors"
	"sort"

	"github.com/openmts/mts/internal/model"
)

type mergeColumnDataStream struct {
	sources     []mergeSource
	current     model.ColumnData
	err         error
	initialized bool
	closed      bool
}

type mergeSource struct {
	stream ColumnDataStream
	column model.ColumnData
	valid  bool
}

func MergeColumnDataStreams(streams ...ColumnDataStream) ColumnDataStream {
	sources := make([]mergeSource, 0, len(streams))
	for _, stream := range streams {
		if stream != nil {
			sources = append(sources, mergeSource{stream: stream})
		}
	}
	return &mergeColumnDataStream{sources: sources}
}

func (s *mergeColumnDataStream) Next() bool {
	if s.closed || s.err != nil {
		return false
	}
	if !s.initialized {
		s.initialize()
	}
	if s.err != nil {
		return false
	}
	group, ok := s.nextGroup()
	if !ok {
		return false
	}
	s.current = mergeColumnGroup(group)
	return true
}

func (s *mergeColumnDataStream) ColumnData() model.ColumnData {
	return s.current
}

func (s *mergeColumnDataStream) Err() error {
	return s.err
}

func (s *mergeColumnDataStream) Close() error {
	s.closed = true
	var err error
	for index := range s.sources {
		err = errors.Join(err, s.sources[index].stream.Close())
	}
	return err
}

func (s *mergeColumnDataStream) initialize() {
	s.initialized = true
	for index := range s.sources {
		s.advance(index)
	}
}

func (s *mergeColumnDataStream) nextGroup() ([]model.ColumnData, bool) {
	index, ok := s.minSourceIndex()
	if !ok {
		return nil, false
	}
	key := columnKeyOf(s.sources[index].column)
	group := make([]model.ColumnData, 0, len(s.sources))
	for sourceIndex := range s.sources {
		source := &s.sources[sourceIndex]
		if !source.valid || columnKeyOf(source.column) != key {
			continue
		}
		group = append(group, source.column)
		s.advance(sourceIndex)
	}
	return group, s.err == nil
}

func (s *mergeColumnDataStream) minSourceIndex() (int, bool) {
	minIndex := -1
	for index, source := range s.sources {
		if !source.valid {
			continue
		}
		if minIndex < 0 || columnLess(source.column, s.sources[minIndex].column) {
			minIndex = index
		}
	}
	return minIndex, minIndex >= 0
}

func (s *mergeColumnDataStream) advance(index int) {
	if s.err != nil {
		return
	}
	source := &s.sources[index]
	if !source.stream.Next() {
		source.valid = false
		if err := source.stream.Err(); err != nil {
			s.err = err
		}
		return
	}
	source.column = source.stream.ColumnData()
	source.valid = true
}

func mergeColumnGroup(columns []model.ColumnData) model.ColumnData {
	if len(columns) == 0 {
		return model.ColumnData{}
	}
	out := model.ColumnData{
		SeriesID:  columns[0].SeriesID,
		FieldID:   columns[0].FieldID,
		FieldType: columns[0].FieldType,
		Samples:   mergeSamples(columns),
	}
	return out
}

func mergeSamples(columns []model.ColumnData) []model.VersionedSample {
	total := 0
	for _, column := range columns {
		total += len(column.Samples)
	}
	samples := make([]model.VersionedSample, 0, total)
	for _, column := range columns {
		samples = append(samples, column.Samples...)
	}
	sort.Slice(samples, func(i, j int) bool {
		if samples[i].Timestamp != samples[j].Timestamp {
			return samples[i].Timestamp < samples[j].Timestamp
		}
		return samples[i].WriteSeq > samples[j].WriteSeq
	})
	return compactMergedSamples(samples)
}

func compactMergedSamples(samples []model.VersionedSample) []model.VersionedSample {
	write := 0
	for _, sample := range samples {
		if write > 0 && samples[write-1].Timestamp == sample.Timestamp {
			continue
		}
		samples[write] = sample
		write++
	}
	return samples[:write]
}

func columnLess(left model.ColumnData, right model.ColumnData) bool {
	leftKey := columnKeyOf(left)
	rightKey := columnKeyOf(right)
	return leftKey.less(rightKey)
}

func columnKeyOf(column model.ColumnData) streamColumnKey {
	return streamColumnKey{seriesID: column.SeriesID, fieldID: column.FieldID}
}

type streamColumnKey struct {
	seriesID uint64
	fieldID  uint32
}

func (k streamColumnKey) less(other streamColumnKey) bool {
	if k.seriesID != other.seriesID {
		return k.seriesID < other.seriesID
	}
	return k.fieldID < other.fieldID
}
