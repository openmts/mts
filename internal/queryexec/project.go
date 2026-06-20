package queryexec

import "github.com/openmts/mts/internal/model"

type projectedColumnStream struct {
	source  ColumnStream
	fields  map[string]struct{}
	current model.ColumnSeries
	closed  bool
}

func NewProjectedColumnStream(source ColumnStream, fields []string) ColumnStream {
	if len(fields) == 0 {
		return source
	}
	fieldSet := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		fieldSet[field] = struct{}{}
	}
	return &projectedColumnStream{source: source, fields: fieldSet}
}

func (s *projectedColumnStream) Next() bool {
	if s.closed || s.source == nil {
		return false
	}
	for s.source.Next() {
		column := s.source.Column()
		if _, ok := s.fields[column.FieldName]; !ok {
			continue
		}
		s.current = column
		return true
	}
	return false
}

func (s *projectedColumnStream) Column() model.ColumnSeries {
	return s.current
}

func (s *projectedColumnStream) Err() error {
	if s.source == nil {
		return nil
	}
	return s.source.Err()
}

func (s *projectedColumnStream) Close() error {
	s.closed = true
	if s.source == nil {
		return nil
	}
	return s.source.Close()
}
