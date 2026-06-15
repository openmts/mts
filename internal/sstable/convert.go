package sstable

import (
	"sort"

	"codeberg.org/mts/mts/internal/model"
)

func groupColumns(columns []model.ColumnData) map[uint64][]model.ColumnData {
	grouped := make(map[uint64][]model.ColumnData)
	for _, column := range columns {
		column.Samples = cloneSamples(column.Samples)
		sortSamples(column.Samples)
		grouped[column.SeriesID] = append(grouped[column.SeriesID], column)
	}
	return grouped
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

func timeBlockFrom(timestamps []int64) timeBlock {
	return timeBlock{
		Encoding:   "plain-int64-v1",
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

func cloneSamples(samples []model.VersionedSample) []model.VersionedSample {
	out := make([]model.VersionedSample, len(samples))
	copy(out, samples)
	return out
}
