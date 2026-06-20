package queryexec

import (
	"time"

	"github.com/openmts/mts/internal/model"
)

type profiledRowStream struct {
	source  RowStream
	profile *OperatorProfile
	start   time.Time
	current model.Row
	done    bool
}

type profiledColumnStream struct {
	source  ColumnStream
	profile *OperatorProfile
	start   time.Time
	current model.ColumnSeries
	done    bool
}

func NewProfiledRowStream(source RowStream, profile *OperatorProfile) RowStream {
	if profile == nil {
		return source
	}
	start := time.Now()
	profile.StartedUnixNanos = start.UnixNano()
	return &profiledRowStream{source: source, profile: profile, start: start}
}

func NewProfiledColumnStream(source ColumnStream, profile *OperatorProfile) ColumnStream {
	if profile == nil {
		return source
	}
	start := time.Now()
	profile.StartedUnixNanos = start.UnixNano()
	return &profiledColumnStream{source: source, profile: profile, start: start}
}

func (s *profiledRowStream) Next() bool {
	if s.done || s.source == nil {
		return false
	}
	if !s.source.Next() {
		s.finish(s.source.Err())
		return false
	}
	s.current = s.source.Row()
	s.profile.RowsOut++
	s.profile.BytesOut += estimateRowBytes(s.current)
	return true
}

func (s *profiledRowStream) Row() model.Row {
	return s.current
}

func (s *profiledRowStream) Err() error {
	if s.source == nil {
		return nil
	}
	return s.source.Err()
}

func (s *profiledRowStream) Close() error {
	if s.source == nil {
		s.finish(nil)
		return nil
	}
	err := s.source.Close()
	s.finish(err)
	return err
}

func (s *profiledRowStream) finish(err error) {
	if s.done {
		return
	}
	s.done = true
	finished := time.Now()
	s.profile.FinishedUnixNanos = finished.UnixNano()
	s.profile.Duration = finished.Sub(s.start)
	if err != nil {
		s.profile.Error = err.Error()
	}
}

func (s *profiledColumnStream) Next() bool {
	if s.done || s.source == nil {
		return false
	}
	if !s.source.Next() {
		s.finish(s.source.Err())
		return false
	}
	s.current = s.source.Column()
	s.profile.ColumnsOut++
	s.profile.SamplesOut += len(s.current.Values)
	s.profile.BytesOut += estimateColumnSeriesBytes(s.current)
	return true
}

func (s *profiledColumnStream) Column() model.ColumnSeries {
	return s.current
}

func (s *profiledColumnStream) Err() error {
	if s.source == nil {
		return nil
	}
	return s.source.Err()
}

func (s *profiledColumnStream) Close() error {
	if s.source == nil {
		s.finish(nil)
		return nil
	}
	err := s.source.Close()
	s.finish(err)
	return err
}

func (s *profiledColumnStream) finish(err error) {
	if s.done {
		return
	}
	s.done = true
	finished := time.Now()
	s.profile.FinishedUnixNanos = finished.UnixNano()
	s.profile.Duration = finished.Sub(s.start)
	if err != nil {
		s.profile.Error = err.Error()
	}
}

func estimateRowBytes(row model.Row) int64 {
	total := int64(64 + len(row.Measurement))
	for key, value := range row.Tags {
		total += int64(len(key) + len(value) + 32)
	}
	for key, value := range row.Fields {
		total += int64(len(key)) + estimateFieldValueBytes(value)
	}
	return total
}

func estimateColumnSeriesBytes(column model.ColumnSeries) int64 {
	total := int64(64 + len(column.Measurement) + len(column.FieldName))
	for key, value := range column.Tags {
		total += int64(len(key) + len(value) + 32)
	}
	total += int64(len(column.Timestamps)) * 8
	for _, value := range column.Values {
		total += estimateFieldValueBytes(value)
	}
	return total
}

func estimateFieldValueBytes(value model.FieldValue) int64 {
	switch value.Type {
	case model.FieldFloat64, model.FieldInt64:
		return 8
	case model.FieldString:
		return int64(16 + len(value.String))
	case model.FieldBool:
		return 1
	default:
		return 0
	}
}
