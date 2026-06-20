package queryexec

import "github.com/openmts/mts/internal/model"

type filteredColumnStream struct {
	source     ColumnStream
	predicates []model.QueryPredicate
	current    model.ColumnSeries
	closed     bool
}

type filteredRowStream struct {
	source     RowStream
	predicates []model.QueryPredicate
	current    model.Row
	closed     bool
}

func NewFilteredColumnStream(source ColumnStream, predicates []model.QueryPredicate) ColumnStream {
	if len(predicates) == 0 {
		return source
	}
	return &filteredColumnStream{source: source, predicates: cloneQueryPredicates(predicates)}
}

func NewFilteredRowStream(source RowStream, predicates []model.QueryPredicate) RowStream {
	if len(predicates) == 0 {
		return source
	}
	return &filteredRowStream{source: source, predicates: cloneQueryPredicates(predicates)}
}

func (s *filteredColumnStream) Next() bool {
	if s.closed || s.source == nil {
		return false
	}
	for s.source.Next() {
		column := filterColumnByPredicates(s.source.Column(), s.predicates)
		if len(column.Values) == 0 {
			continue
		}
		s.current = column
		return true
	}
	return false
}

func (s *filteredColumnStream) Column() model.ColumnSeries {
	return s.current
}

func (s *filteredColumnStream) Err() error {
	if s.source == nil {
		return nil
	}
	return s.source.Err()
}

func (s *filteredColumnStream) Close() error {
	s.closed = true
	if s.source == nil {
		return nil
	}
	return s.source.Close()
}

func (s *filteredRowStream) Next() bool {
	if s.closed || s.source == nil {
		return false
	}
	for s.source.Next() {
		row := s.source.Row()
		if rowMatchesPredicates(row, s.predicates) {
			s.current = row
			return true
		}
	}
	return false
}

func (s *filteredRowStream) Row() model.Row {
	return s.current
}

func (s *filteredRowStream) Err() error {
	if s.source == nil {
		return nil
	}
	return s.source.Err()
}

func (s *filteredRowStream) Close() error {
	s.closed = true
	if s.source == nil {
		return nil
	}
	return s.source.Close()
}

func filterColumnByPredicates(
	column model.ColumnSeries,
	predicates []model.QueryPredicate,
) model.ColumnSeries {
	relevant := fieldPredicatesFor(column.FieldName, predicates)
	if len(relevant) == 0 {
		return column
	}
	write := 0
	for index, value := range column.Values {
		if valueMatchesPredicates(value, relevant) {
			column.Values[write] = value
			column.Timestamps[write] = column.Timestamps[index]
			write++
		}
	}
	column.Values = column.Values[:write]
	column.Timestamps = column.Timestamps[:write]
	return column
}

func fieldPredicatesFor(name string, predicates []model.QueryPredicate) []model.QueryPredicate {
	out := make([]model.QueryPredicate, 0, len(predicates))
	for _, predicate := range predicates {
		if predicate.Name == name {
			out = append(out, predicate)
		}
	}
	return out
}

func rowMatchesPredicates(row model.Row, predicates []model.QueryPredicate) bool {
	for _, predicate := range predicates {
		value, ok := row.Fields[predicate.Name]
		if !ok || !valueMatchesPredicate(value, predicate) {
			return false
		}
	}
	return true
}

func valueMatchesPredicates(value model.FieldValue, predicates []model.QueryPredicate) bool {
	for _, predicate := range predicates {
		if !valueMatchesPredicate(value, predicate) {
			return false
		}
	}
	return true
}

func valueMatchesPredicate(value model.FieldValue, predicate model.QueryPredicate) bool {
	switch predicate.Kind {
	case model.QueryPredicateFieldEq:
		return fieldValuesCompare(value, predicate.Value) == 0
	case model.QueryPredicateFieldNe:
		return fieldValuesCompare(value, predicate.Value) != 0
	case model.QueryPredicateFieldGT:
		return fieldValuesCompare(value, predicate.Value) > 0
	case model.QueryPredicateFieldGTE:
		return fieldValuesCompare(value, predicate.Value) >= 0
	case model.QueryPredicateFieldLT:
		return fieldValuesCompare(value, predicate.Value) < 0
	case model.QueryPredicateFieldLTE:
		return fieldValuesCompare(value, predicate.Value) <= 0
	default:
		return true
	}
}

func fieldValuesCompare(left model.FieldValue, right model.FieldValue) int {
	if isNumericFieldValue(left) && isNumericFieldValue(right) {
		return compareFloat(fieldValueAsFloat(left), fieldValueAsFloat(right))
	}
	if left.Type != right.Type {
		return -1
	}
	switch left.Type {
	case model.FieldString:
		return compareString(left.String, right.String)
	case model.FieldBool:
		return compareBool(left.Bool, right.Bool)
	default:
		return -1
	}
}

func isNumericFieldValue(value model.FieldValue) bool {
	return value.Type == model.FieldFloat64 || value.Type == model.FieldInt64
}

func fieldValueAsFloat(value model.FieldValue) float64 {
	if value.Type == model.FieldFloat64 {
		return value.Float64
	}
	return float64(value.Int64)
}

func compareFloat(left float64, right float64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareString(left string, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareBool(left bool, right bool) int {
	switch {
	case left == right:
		return 0
	case !left && right:
		return -1
	default:
		return 1
	}
}

func cloneQueryPredicates(predicates []model.QueryPredicate) []model.QueryPredicate {
	out := make([]model.QueryPredicate, len(predicates))
	copy(out, predicates)
	for index := range out {
		out[index].StringValues = append([]string(nil), out[index].StringValues...)
	}
	return out
}
