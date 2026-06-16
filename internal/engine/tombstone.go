package engine

import "codeberg.org/mts/mts/internal/model"

func applyTombstones(
	columns []model.ColumnData,
	tombstones []model.Tombstone,
) []model.ColumnData {
	if len(columns) == 0 || len(tombstones) == 0 {
		return columns
	}
	out := columns[:0]
	for _, column := range columns {
		column.Samples = filterTombstonedSamples(column, tombstones)
		if len(column.Samples) > 0 {
			out = append(out, column)
		}
	}
	return out
}

func filterTombstonedSamples(
	column model.ColumnData,
	tombstones []model.Tombstone,
) []model.VersionedSample {
	samples := column.Samples[:0]
	for _, sample := range column.Samples {
		if !sampleDeleted(column, sample, tombstones) {
			samples = append(samples, sample)
		}
	}
	return samples
}

func sampleDeleted(
	column model.ColumnData,
	sample model.VersionedSample,
	tombstones []model.Tombstone,
) bool {
	for _, tombstone := range tombstones {
		if tombstoneMatches(column, sample, tombstone) {
			return true
		}
	}
	return false
}

func tombstoneMatches(
	column model.ColumnData,
	sample model.VersionedSample,
	tombstone model.Tombstone,
) bool {
	if sample.Timestamp < tombstone.StartTime || sample.Timestamp > tombstone.EndTime {
		return false
	}
	if sample.WriteSeq > tombstoneWriteSeq(tombstone) {
		return false
	}
	return idMatches(tombstone.SeriesIDs, column.SeriesID) && id32Matches(tombstone.FieldIDs, column.FieldID)
}

func tombstoneWriteSeq(tombstone model.Tombstone) uint64 {
	if tombstone.WriteSeq == 0 {
		return ^uint64(0)
	}
	return tombstone.WriteSeq
}

func idMatches(filter []uint64, id uint64) bool {
	if len(filter) == 0 {
		return true
	}
	for _, value := range filter {
		if value == id {
			return true
		}
	}
	return false
}

func id32Matches(filter []uint32, id uint32) bool {
	if len(filter) == 0 {
		return true
	}
	for _, value := range filter {
		if value == id {
			return true
		}
	}
	return false
}
