package mts

import (
	storageengine "github.com/openmts/mts/internal/engine"
	"github.com/openmts/mts/internal/model"
)

func fromStorageMemorySnapshot(snapshot storageengine.StorageMemorySnapshot) StorageMemorySnapshot {
	return StorageMemorySnapshot{
		CurrentBytes:          snapshot.CurrentBytes,
		PeakBytes:             snapshot.PeakBytes,
		ActiveBytes:           snapshot.ActiveBytes,
		MemTableBytes:         snapshot.MemTableBytes,
		WALBytes:              snapshot.WALBytes,
		ReservationBytes:      snapshot.ReservationBytes,
		WriteBytes:            snapshot.WriteBytes,
		FlushBytes:            snapshot.FlushBytes,
		QueryBytes:            snapshot.QueryBytes,
		CompactionBytes:       snapshot.CompactionBytes,
		CompressionBytes:      snapshot.CompressionBytes,
		SoftBytesLimit:        snapshot.SoftBytesLimit,
		HardBytesLimit:        snapshot.HardBytesLimit,
		RejectedWrites:        snapshot.RejectedWrites,
		RejectedReservations:  snapshot.RejectedReservations,
		FlushTriggered:        snapshot.FlushTriggered,
		QueryBytesLimit:       snapshot.QueryBytesLimit,
		FlushBytesLimit:       snapshot.FlushBytesLimit,
		CompactionBytesLimit:  snapshot.CompactionBytesLimit,
		CompressionBytesLimit: snapshot.CompressionBytesLimit,
		RuntimeHeapAllocBytes: snapshot.RuntimeHeapAllocBytes,
		RuntimeRSSBytes:       snapshot.RuntimeRSSBytes,
		RuntimeGapBytes:       snapshot.RuntimeGapBytes,
	}
}

func fromCompactionTaskStatus(status storageengine.CompactionTaskStatus) CompactionTaskStatus {
	return CompactionTaskStatus{
		ID:          status.ID,
		State:       status.State,
		Level:       status.Level,
		OutputLevel: status.OutputLevel,
		Reason:      status.Reason,
		Score:       status.Score,
		StartedAt:   status.StartedAt,
		FinishedAt:  status.FinishedAt,
		Duration:    status.Duration,
		InputParts:  status.InputParts,
		OutputParts: status.OutputParts,
		InputBytes:  status.InputBytes,
		OutputBytes: status.OutputBytes,
		DroppedRows: status.DroppedRows,
		Error:       status.Error,
	}
}

func fromCompactionStats(stats storageengine.CompactionStats) CompactionStats {
	return CompactionStats{
		Active:          stats.Active,
		Backlog:         stats.Backlog,
		Skipped:         stats.Skipped,
		Total:           stats.Total,
		Success:         stats.Success,
		Failure:         stats.Failure,
		InputParts:      stats.InputParts,
		OutputParts:     stats.OutputParts,
		InputBytes:      stats.InputBytes,
		OutputBytes:     stats.OutputBytes,
		DroppedRows:     stats.DroppedRows,
		OverlapCount:    stats.OverlapCount,
		MaxScore:        stats.MaxScore,
		LastReason:      stats.LastReason,
		LastLevel:       stats.LastLevel,
		LastOutputLevel: stats.LastOutputLevel,
		LastDuration:    stats.LastDuration,
		LastError:       stats.LastError,
		LastSkipReason:  stats.LastSkipReason,
		LastTask:        fromCompactionTaskStatus(stats.LastTask),
		SafeDeleteParts: stats.SafeDeleteParts,
	}
}

func fromCompactionResult(result storageengine.CompactionResult) CompactionResult {
	return CompactionResult{
		State:       result.State,
		Duration:    result.Duration,
		Shards:      result.Shards,
		InputParts:  result.InputParts,
		OutputParts: result.OutputParts,
		InputBytes:  result.InputBytes,
		OutputBytes: result.OutputBytes,
		DroppedRows: result.DroppedRows,
		Error:       result.Error,
		LastTask:    fromCompactionTaskStatus(result.LastTask),
	}
}

func toModelRetentionPolicy(policy RetentionPolicy) model.RetentionPolicy {
	return model.RetentionPolicy{
		Name:     policy.Name,
		Duration: policy.Duration,
	}
}

func fromModelRetentionPolicy(policy model.RetentionPolicy) RetentionPolicy {
	return RetentionPolicy{
		Name:     policy.Name,
		Duration: policy.Duration,
	}
}

func fromModelRetentionPolicies(policies []model.RetentionPolicy) []RetentionPolicy {
	out := make([]RetentionPolicy, len(policies))
	for index, policy := range policies {
		out[index] = fromModelRetentionPolicy(policy)
	}
	return out
}

func fromModelFieldSchema(field model.FieldSchema) FieldSchema {
	return FieldSchema{
		Measurement: field.Measurement,
		Name:        field.Name,
		Type:        fromModelFieldType(field.Type),
	}
}

func fromModelFieldSchemas(fields []model.FieldSchema) []FieldSchema {
	out := make([]FieldSchema, len(fields))
	for index, field := range fields {
		out[index] = fromModelFieldSchema(field)
	}
	return out
}

func fromModelSeries(series model.Series) Series {
	return Series{
		ID:          series.ID,
		Measurement: series.Measurement,
		Tags:        cloneStringMap(series.Tags),
	}
}

func fromModelSeriesList(series []model.Series) []Series {
	out := make([]Series, len(series))
	for index, item := range series {
		out[index] = fromModelSeries(item)
	}
	return out
}

func fromModelColumnSeries(column model.ColumnSeries) ColumnSeries {
	values := make([]FieldValue, len(column.Values))
	for index, value := range column.Values {
		values[index] = fromModelFieldValue(value)
	}
	return ColumnSeries{
		SeriesID:    column.SeriesID,
		Measurement: column.Measurement,
		Tags:        cloneStringMap(column.Tags),
		FieldID:     column.FieldID,
		FieldName:   column.FieldName,
		FieldType:   fromModelFieldType(column.FieldType),
		Timestamps:  append([]int64(nil), column.Timestamps...),
		Values:      values,
	}
}

func fromModelColumnSeriesList(columns []model.ColumnSeries) []ColumnSeries {
	out := make([]ColumnSeries, len(columns))
	for index, column := range columns {
		out[index] = fromModelColumnSeries(column)
	}
	return out
}

func fromModelRow(row model.Row) Row {
	fields := make(map[string]FieldValue, len(row.Fields))
	for name, value := range row.Fields {
		fields[name] = fromModelFieldValue(value)
	}
	return Row{
		SeriesID:    row.SeriesID,
		Measurement: row.Measurement,
		Tags:        cloneStringMap(row.Tags),
		Timestamp:   row.Timestamp,
		Fields:      fields,
	}
}

func fromModelRows(rows []model.Row) []Row {
	out := make([]Row, len(rows))
	for index, row := range rows {
		out[index] = fromModelRow(row)
	}
	return out
}
