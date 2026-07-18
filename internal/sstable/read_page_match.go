package sstable

import "github.com/openmts/mts/internal/model"

func scanValuePageIndexCoverage(
	payload []byte,
	query Query,
) (valuePageIndexHeader, []valuePageRef, bool, int, error) {
	reader := newBlockReader(payload)
	header, err := readValuePageIndexHeader(reader)
	if err != nil {
		return valuePageIndexHeader{}, nil, false, 0, err
	}
	pages := make([]valuePageRef, 0, header.pageCount)
	matches := 0
	fullRange := header.pageCount > 0
	for range header.pageCount {
		page, err := readValuePageRef(reader, header.fieldType)
		if err != nil {
			return valuePageIndexHeader{}, nil, false, 0, err
		}
		if valuePageMatchesQuery(page, header.fieldID, header.fieldType, query) {
			matches++
			if fullRange {
				pages = append(pages, page)
			}
			continue
		}
		fullRange = false
		pages = nil
	}
	if err := reader.done("value page index"); err != nil {
		return valuePageIndexHeader{}, nil, false, 0, err
	}
	if !fullRange {
		pages = nil
	}
	return header, pages, fullRange, matches, nil
}

func matchingValuePageIndexHeader(payload []byte, query Query) (valuePageIndexHeader, int, error) {
	reader := newBlockReader(payload)
	header, err := readValuePageIndexHeader(reader)
	if err != nil {
		return valuePageIndexHeader{}, 0, err
	}
	matches := 0
	for range header.pageCount {
		page, err := readValuePageRef(reader, header.fieldType)
		if err != nil {
			return valuePageIndexHeader{}, 0, err
		}
		if valuePageMatchesQuery(page, header.fieldID, header.fieldType, query) {
			matches++
		}
	}
	if err := reader.done("value page index"); err != nil {
		return valuePageIndexHeader{}, 0, err
	}
	return header, matches, nil
}

func matchingBoundaryPageRefs(
	payload []byte,
	fullPages []valuePageRef,
	fullRange bool,
	query Query,
) ([]valuePageRef, error) {
	if fullRange {
		return fullPages, nil
	}
	return matchingValuePageRefs(payload, query)
}

func matchingValuePageRefs(payload []byte, query Query) ([]valuePageRef, error) {
	reader := newBlockReader(payload)
	header, err := readValuePageIndexHeader(reader)
	if err != nil {
		return nil, err
	}
	pages := make([]valuePageRef, 0, header.pageCount)
	for range header.pageCount {
		page, err := readValuePageRef(reader, header.fieldType)
		if err != nil {
			return nil, err
		}
		if valuePageMatchesQuery(page, header.fieldID, header.fieldType, query) {
			pages = append(pages, page)
		}
	}
	if err := reader.done("value page index"); err != nil {
		return nil, err
	}
	return pages, nil
}

func valuePageMatchesQuery(
	page valuePageRef,
	fieldID uint32,
	fieldType model.FieldType,
	query Query,
) bool {
	if page.MaxTime < query.Start || page.MinTime > query.End {
		return false
	}
	predicates := query.FieldPredicates[fieldID]
	if len(predicates) == 0 || !page.Stats.HasNumeric {
		return true
	}
	for _, predicate := range predicates {
		if !numericPageMayMatchPredicate(page.Stats, fieldType, predicate) {
			return false
		}
	}
	return true
}

func numericPageMayMatchPredicate(
	stats valuePageStats,
	fieldType model.FieldType,
	predicate model.QueryPredicate,
) bool {
	switch fieldType {
	case model.FieldFloat64:
		return floatPageMayMatchPredicate(stats.MinFloat64, stats.MaxFloat64, predicate)
	case model.FieldInt64:
		return intPageMayMatchPredicate(stats.MinInt64, stats.MaxInt64, predicate)
	default:
		return true
	}
}

func floatPageMayMatchPredicate(minValue float64, maxValue float64, predicate model.QueryPredicate) bool {
	value := queryPredicateFloatValue(predicate)
	switch predicate.Kind {
	case model.QueryPredicateFieldEq:
		return value >= minValue && value <= maxValue
	case model.QueryPredicateFieldNe:
		return minValue != maxValue || value != minValue
	case model.QueryPredicateFieldGT:
		return maxValue > value
	case model.QueryPredicateFieldGTE:
		return maxValue >= value
	case model.QueryPredicateFieldLT:
		return minValue < value
	case model.QueryPredicateFieldLTE:
		return minValue <= value
	default:
		return true
	}
}

func intPageMayMatchPredicate(minValue int64, maxValue int64, predicate model.QueryPredicate) bool {
	value := queryPredicateIntValue(predicate)
	switch predicate.Kind {
	case model.QueryPredicateFieldEq:
		return value >= minValue && value <= maxValue
	case model.QueryPredicateFieldNe:
		return minValue != maxValue || value != minValue
	case model.QueryPredicateFieldGT:
		return maxValue > value
	case model.QueryPredicateFieldGTE:
		return maxValue >= value
	case model.QueryPredicateFieldLT:
		return minValue < value
	case model.QueryPredicateFieldLTE:
		return minValue <= value
	default:
		return true
	}
}

func queryPredicateFloatValue(predicate model.QueryPredicate) float64 {
	if predicate.Value.Type == model.FieldFloat64 {
		return predicate.Value.Float64
	}
	return float64(predicate.Value.Int64)
}

func queryPredicateIntValue(predicate model.QueryPredicate) int64 {
	if predicate.Value.Type == model.FieldInt64 {
		return predicate.Value.Int64
	}
	return int64(predicate.Value.Float64)
}

func selectBoundaryPageRefs(pages []valuePageRef, mode model.QueryBoundaryMode) []valuePageRef {
	if len(pages) == 0 {
		return nil
	}
	switch mode {
	case model.QueryBoundaryFirst:
		return pages[:1]
	case model.QueryBoundaryLast:
		return pages[len(pages)-1:]
	case model.QueryBoundaryBoth:
		return firstAndLastPageRefs(pages)
	default:
		return pages
	}
}

func firstAndLastPageRefs(pages []valuePageRef) []valuePageRef {
	if len(pages) == 1 {
		return pages[:1]
	}
	return []valuePageRef{pages[0], pages[len(pages)-1]}
}

func boundaryModeEnabled(mode model.QueryBoundaryMode) bool {
	return mode == model.QueryBoundaryFirst || mode == model.QueryBoundaryLast || mode == model.QueryBoundaryBoth
}

func matchingValuePageCapacity(header valuePageIndexHeader, matches int) int {
	if header.pageCount == 0 || header.count == 0 || matches == 0 {
		return 0
	}
	pageAverage := (header.count + header.pageCount - 1) / header.pageCount
	capacity := matches * pageAverage
	if capacity > header.count {
		return header.count
	}
	return capacity
}
