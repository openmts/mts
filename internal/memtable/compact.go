package memtable

import (
	"cmp"
	"slices"
	"sort"

	"github.com/openmts/mts/internal/model"
)

func compactSamples(column *columnBuffer, query Query) []model.VersionedSample {
	if column.count == 0 {
		return []model.VersionedSample{}
	}
	if columnTimesSortedUnique(column) {
		return materializeSortedColumnSamples(column, query)
	}
	matches := countMatchingSamples(column, query)
	if matches == 0 {
		return []model.VersionedSample{}
	}
	samples := make([]model.VersionedSample, 0, matches)
	for index := range column.count {
		sample := column.sampleAt(index)
		samples = appendMatchingSample(samples, sample, query)
	}
	return compactMaterializedSamples(samples)
}

func compactSamplesInto(
	dst []model.VersionedSample,
	column *columnBuffer,
	query Query,
) []model.VersionedSample {
	if column.count == 0 {
		return dst[:0]
	}
	if columnTimesSortedUnique(column) {
		return materializeSortedColumnSamplesInto(dst, column, query)
	}
	matches := countMatchingSamples(column, query)
	if matches == 0 {
		return dst[:0]
	}
	if cap(dst) < matches {
		dst = make([]model.VersionedSample, 0, matches)
	} else {
		dst = dst[:0]
	}
	for index := range column.count {
		sample := column.sampleAt(index)
		dst = appendMatchingSample(dst, sample, query)
	}
	return compactMaterializedSamples(dst)
}

func materializeSortedColumnSamples(column *columnBuffer, query Query) []model.VersionedSample {
	start, end := sortedRangeBounds(column.times[:column.count], query)
	samples := make([]model.VersionedSample, end-start)
	for index := start; index < end; index++ {
		samples[index-start] = column.sampleAt(index)
	}
	return samples
}

func materializeSortedColumnSamplesInto(
	dst []model.VersionedSample,
	column *columnBuffer,
	query Query,
) []model.VersionedSample {
	start, end := sortedRangeBounds(column.times[:column.count], query)
	needed := end - start
	if cap(dst) < needed {
		dst = make([]model.VersionedSample, needed)
	} else {
		dst = dst[:needed]
	}
	for index := start; index < end; index++ {
		dst[index-start] = column.sampleAt(index)
	}
	return dst
}

func sortedRangeBounds(times []int64, query Query) (int, int) {
	start := sort.Search(len(times), func(index int) bool {
		return times[index] >= query.Start
	})
	end := sort.Search(len(times), func(index int) bool {
		return times[index] > query.End
	})
	if end < start {
		return start, start
	}
	return start, end
}

func columnTimesSortedUnique(column *columnBuffer) bool {
	var previous int64
	for index := range column.count {
		timestamp := column.times[index]
		if index > 0 && timestamp <= previous {
			return false
		}
		previous = timestamp
	}
	return true
}

func countMatchingSamples(column *columnBuffer, query Query) int {
	count := 0
	for index := range column.count {
		timestamp := column.times[index]
		if timestamp >= query.Start && timestamp <= query.End {
			count++
		}
	}
	return count
}

func compactMaterializedSamples(samples []model.VersionedSample) []model.VersionedSample {
	if len(samples) <= 1 {
		return samples
	}
	slices.SortFunc(samples, func(left model.VersionedSample, right model.VersionedSample) int {
		if left.Timestamp != right.Timestamp {
			return cmp.Compare(left.Timestamp, right.Timestamp)
		}
		return cmp.Compare(right.WriteSeq, left.WriteSeq)
	})
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

func appendMatchingSample(
	dst []model.VersionedSample,
	sample model.VersionedSample,
	query Query,
) []model.VersionedSample {
	if sample.Timestamp < query.Start || sample.Timestamp > query.End {
		return dst
	}
	return append(dst, sample)
}

func sortColumns(columns []model.ColumnData) {
	slices.SortFunc(columns, func(left model.ColumnData, right model.ColumnData) int {
		if left.SeriesID != right.SeriesID {
			return cmp.Compare(left.SeriesID, right.SeriesID)
		}
		return cmp.Compare(left.FieldID, right.FieldID)
	})
}

func containsSeries(filter map[uint64]struct{}, seriesID uint64) bool {
	if len(filter) == 0 {
		return true
	}
	_, ok := filter[seriesID]
	return ok
}

func containsField(filter map[uint32]struct{}, fieldID uint32) bool {
	if len(filter) == 0 {
		return true
	}
	_, ok := filter[fieldID]
	return ok
}
