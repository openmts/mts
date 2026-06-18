package sstable

import (
	"sort"

	"github.com/openmts/mts/internal/model"
)

type columnGroup struct {
	seriesID uint64
	columns  []model.ColumnData
}

func groupColumns(columns []model.ColumnData) map[uint64][]model.ColumnData {
	cloned := append([]model.ColumnData(nil), columns...)
	groups := groupColumnRuns(cloned)
	grouped := make(map[uint64][]model.ColumnData)
	for _, group := range groups {
		grouped[group.seriesID] = group.columns
	}
	return grouped
}

func groupColumnRuns(columns []model.ColumnData) []columnGroup {
	if len(columns) == 0 {
		return nil
	}
	sortColumnsForWrite(columns)
	for index := range columns {
		if !samplesSorted(columns[index].Samples) {
			columns[index].Samples = cloneSamples(columns[index].Samples)
			sortSamples(columns[index].Samples)
		}
	}
	groups := make([]columnGroup, 0, len(columns))
	for start := 0; start < len(columns); {
		end := start + 1
		for end < len(columns) && columns[end].SeriesID == columns[start].SeriesID {
			end++
		}
		groups = append(groups, columnGroup{
			seriesID: columns[start].SeriesID,
			columns:  columns[start:end],
		})
		start = end
	}
	return groups
}

func sortColumnsForWrite(columns []model.ColumnData) {
	sort.Slice(columns, func(i, j int) bool {
		if columns[i].SeriesID != columns[j].SeriesID {
			return columns[i].SeriesID < columns[j].SeriesID
		}
		return columns[i].FieldID < columns[j].FieldID
	})
}

func sortedSeriesIDs(grouped map[uint64][]model.ColumnData) []uint64 {
	ids := make([]uint64, 0, len(grouped))
	for id := range grouped {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return ids
}

func collectTimestamps(columns []model.ColumnData) []int64 {
	if timestamps, ok := alignedTimestamps(columns); ok {
		return timestamps
	}
	if columnsSamplesSorted(columns) {
		return mergeColumnTimestamps(columns)
	}
	return collectTimestampsFallback(columns)
}

func mergeColumnTimestamps(columns []model.ColumnData) []int64 {
	positions := make([]int, len(columns))
	timestamps := make([]int64, 0, totalSamples(columns))
	for {
		timestamp, ok := nextTimestamp(columns, positions)
		if !ok {
			return timestamps
		}
		timestamps = append(timestamps, timestamp)
		for columnIndex, column := range columns {
			for positions[columnIndex] < len(column.Samples) &&
				column.Samples[positions[columnIndex]].Timestamp == timestamp {
				positions[columnIndex]++
			}
		}
	}
}

func nextTimestamp(columns []model.ColumnData, positions []int) (int64, bool) {
	var timestamp int64
	found := false
	for index, column := range columns {
		if positions[index] >= len(column.Samples) {
			continue
		}
		current := column.Samples[positions[index]].Timestamp
		if !found || current < timestamp {
			timestamp = current
			found = true
		}
	}
	return timestamp, found
}

func totalSamples(columns []model.ColumnData) int {
	total := 0
	for _, column := range columns {
		total += len(column.Samples)
	}
	return total
}

func collectTimestampsFallback(columns []model.ColumnData) []int64 {
	seen := make(map[int64]struct{})
	for _, column := range columns {
		for _, sample := range column.Samples {
			seen[sample.Timestamp] = struct{}{}
		}
	}
	timestamps := make([]int64, 0, len(seen))
	for timestamp := range seen {
		timestamps = append(timestamps, timestamp)
	}
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i] < timestamps[j]
	})
	return timestamps
}

func columnsSamplesSorted(columns []model.ColumnData) bool {
	for _, column := range columns {
		if !samplesSorted(column.Samples) {
			return false
		}
	}
	return true
}

func alignedTimestamps(columns []model.ColumnData) ([]int64, bool) {
	if len(columns) == 0 {
		return []int64{}, true
	}
	first := columns[0].Samples
	for _, column := range columns[1:] {
		if len(column.Samples) != len(first) {
			return nil, false
		}
		for index, sample := range column.Samples {
			if sample.Timestamp != first[index].Timestamp {
				return nil, false
			}
		}
	}
	timestamps := make([]int64, len(first))
	for index, sample := range first {
		timestamps[index] = sample.Timestamp
	}
	return timestamps, true
}

func timeBlockFrom(timestamps []int64) timeBlock {
	return timeBlock{
		Encoding:   "plain-int64",
		MinTime:    timestamps[0],
		MaxTime:    timestamps[len(timestamps)-1],
		Timestamps: append([]int64{}, timestamps...),
	}
}

func updateMeta(meta *PartMeta, row indexRow, columns []model.ColumnData) {
	if meta.RowsCount == 0 || row.MinTime < meta.MinTime {
		meta.MinTime = row.MinTime
	}
	if row.MaxTime > meta.MaxTime {
		meta.MaxTime = row.MaxTime
	}
	if meta.SeriesCount == 0 || row.SeriesID < meta.MinSeriesID {
		meta.MinSeriesID = row.SeriesID
	}
	if row.SeriesID > meta.MaxSeriesID {
		meta.MaxSeriesID = row.SeriesID
	}
	meta.SeriesCount++
	meta.BlockCount++
	for _, column := range columns {
		meta.RowsCount += len(column.Samples)
		for _, sample := range column.Samples {
			if sample.WriteSeq > meta.MaxWriteSeq {
				meta.MaxWriteSeq = sample.WriteSeq
			}
		}
	}
}

func metaIndexFromRows(meta PartMeta, ref blockRef, rows []indexRow) metaIndexRow {
	return metaIndexRow{
		MinSeriesID: meta.MinSeriesID,
		MaxSeriesID: meta.MaxSeriesID,
		MinTime:     meta.MinTime,
		MaxTime:     meta.MaxTime,
		FieldIDs:    collectFieldIDs(rows),
		IndexRef:    ref,
	}
}

func seriesIndexFromRows(rows []indexRow, refs []blockRef) []seriesIndexRow {
	out := make([]seriesIndexRow, 0, len(rows))
	for index, row := range rows {
		out = append(out, seriesIndexRow{
			SeriesID: row.SeriesID,
			MinTime:  row.MinTime,
			MaxTime:  row.MaxTime,
			FieldIDs: collectRowFieldIDs(row),
			IndexRef: refs[index],
		})
	}
	return out
}

func collectRowFieldIDs(row indexRow) []uint32 {
	ids := make([]uint32, 0, len(row.Columns))
	for _, column := range row.Columns {
		ids = append(ids, column.FieldID)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return ids
}

func collectFieldIDs(rows []indexRow) []uint32 {
	seen := make(map[uint32]struct{})
	for _, row := range rows {
		for _, column := range row.Columns {
			seen[column.FieldID] = struct{}{}
		}
	}
	ids := make([]uint32, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return ids
}

func sortSamples(samples []model.VersionedSample) {
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].Timestamp < samples[j].Timestamp
	})
}

func samplesSorted(samples []model.VersionedSample) bool {
	for index := 1; index < len(samples); index++ {
		if samples[index-1].Timestamp > samples[index].Timestamp {
			return false
		}
	}
	return true
}

func cloneSamples(samples []model.VersionedSample) []model.VersionedSample {
	out := make([]model.VersionedSample, len(samples))
	copy(out, samples)
	return out
}
