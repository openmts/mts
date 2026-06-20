package queryexec

import "github.com/openmts/mts/internal/model"

type exprFilteredRowStream struct {
	source  RowStream
	expr    model.QueryExpr
	current model.Row
	closed  bool
}

func NewExprFilteredRowStream(source RowStream, expr model.QueryExpr) RowStream {
	if expr.Kind == model.QueryExprNone {
		return source
	}
	return &exprFilteredRowStream{source: source, expr: expr}
}

func (s *exprFilteredRowStream) Next() bool {
	if s.closed || s.source == nil {
		return false
	}
	for s.source.Next() {
		row := s.source.Row()
		if rowMatchesExpr(row, s.expr) {
			s.current = row
			return true
		}
	}
	return false
}

func (s *exprFilteredRowStream) Row() model.Row {
	return s.current
}

func (s *exprFilteredRowStream) Err() error {
	if s.source == nil {
		return nil
	}
	return s.source.Err()
}

func (s *exprFilteredRowStream) Close() error {
	s.closed = true
	if s.source == nil {
		return nil
	}
	return s.source.Close()
}

func rowMatchesExpr(row model.Row, expr model.QueryExpr) bool {
	switch expr.Kind {
	case model.QueryExprNone:
		return true
	case model.QueryExprPredicate:
		return rowMatchesExprPredicate(row, expr.Predicate)
	case model.QueryExprAnd:
		for _, child := range expr.Children {
			if !rowMatchesExpr(row, child) {
				return false
			}
		}
		return true
	case model.QueryExprOr:
		for _, child := range expr.Children {
			if rowMatchesExpr(row, child) {
				return true
			}
		}
		return false
	case model.QueryExprNot:
		if len(expr.Children) == 0 {
			return true
		}
		return !rowMatchesExpr(row, expr.Children[0])
	default:
		return false
	}
}

func rowMatchesExprPredicate(row model.Row, predicate model.QueryPredicate) bool {
	switch predicate.Kind {
	case model.QueryPredicateTimeRange:
		return row.Timestamp >= predicate.Start && row.Timestamp <= predicate.End
	case model.QueryPredicateTagEq:
		value, ok := row.Tags[predicate.Name]
		return ok && len(predicate.StringValues) > 0 && value == predicate.StringValues[0]
	case model.QueryPredicateTagNe:
		value, ok := row.Tags[predicate.Name]
		return !ok || len(predicate.StringValues) == 0 || value != predicate.StringValues[0]
	case model.QueryPredicateTagExists:
		_, ok := row.Tags[predicate.Name]
		return ok
	case model.QueryPredicateTagIn:
		value, ok := row.Tags[predicate.Name]
		return ok && exprStringIn(value, predicate.StringValues)
	default:
		value, ok := row.Fields[predicate.Name]
		return ok && valueMatchesPredicate(value, predicate)
	}
}

func exprStringIn(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}
