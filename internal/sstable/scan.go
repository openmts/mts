package sstable

import (
	"fmt"
	"sort"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryexec"
)

type partColumnDataStream struct {
	part      *Part
	query     Query
	rows      *indexRowStream
	payload   blockPayload
	seriesIDs []uint64
	refs      []columnRef
	pending   []model.ColumnData
	current   model.ColumnData
	err       error
	pendingAt int
	seriesAt  int
	rowAt     int
	closed    bool
}

func newPartColumnDataStream(part *Part, query Query) (queryexec.ColumnDataStream, error) {
	if err := queryContextErr(query); err != nil {
		return queryexec.NewErrorColumnDataStream(err), nil
	}
	if query.End < query.Start || !partMatches(part.metadata.Part, part.metaRows, query) {
		recordPartSkipped(query)
		return queryexec.NewSliceColumnDataStream(nil), nil
	}
	recordPartScanned(query)
	if len(query.SeriesIDs) > 0 {
		seriesIDs := sortedQuerySeriesIDs(query.SeriesIDs)
		if len(seriesIDs) == 0 || len(part.seriesRows) == 0 {
			return queryexec.NewSliceColumnDataStream(nil), nil
		}
		return &partColumnDataStream{
			part:      part,
			query:     query,
			seriesIDs: seriesIDs,
			refs:      make([]columnRef, 0, 16),
		}, nil
	}
	rows, payload, err := part.openIndexRowStream()
	if err != nil {
		return nil, err
	}
	return &partColumnDataStream{
		part:    part,
		query:   query,
		rows:    rows,
		payload: payload,
		refs:    make([]columnRef, 0, 16),
	}, nil
}

func (s *partColumnDataStream) Next() bool {
	if s.closed || s.err != nil {
		return false
	}
	if err := queryContextErr(s.query); err != nil {
		s.err = err
		return false
	}
	for {
		if s.nextPending() {
			return true
		}
		if !s.loadNextRow() {
			return false
		}
	}
}

func (s *partColumnDataStream) ColumnData() model.ColumnData {
	return s.current
}

func (s *partColumnDataStream) Err() error {
	return s.err
}

func (s *partColumnDataStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	s.payload.Release()
	s.pending = nil
	s.refs = nil
	return nil
}

func (s *partColumnDataStream) nextPending() bool {
	if s.pendingAt >= len(s.pending) {
		return false
	}
	s.current = s.pending[s.pendingAt]
	s.pendingAt++
	return true
}

func (s *partColumnDataStream) loadNextRow() bool {
	if len(s.seriesIDs) > 0 {
		return s.loadNextSeriesRow()
	}
	for {
		if err := queryContextErr(s.query); err != nil {
			s.err = err
			return false
		}
		header, ok, err := s.rows.nextHeader()
		if err != nil {
			s.err = fmt.Errorf("decode part index: %w", err)
			return false
		}
		if !ok {
			s.finishRows()
			return false
		}
		if s.loadRowColumns(header) {
			return true
		}
		if s.err != nil {
			return false
		}
	}
}

func (s *partColumnDataStream) loadRowColumns(header indexRowHeader) bool {
	if err := queryContextErr(s.query); err != nil {
		s.err = err
		return false
	}
	if !rowHeaderMatches(header, s.query) {
		recordIndexRowSkipped(s.query)
		s.err = skipIndexColumnRefs(s.rows)
		return false
	}
	recordIndexRowRead(s.query)
	refs, err := s.rows.appendFilteredColumnRefs(s.refs[:0], s.query.FieldIDs)
	if err != nil {
		s.err = fmt.Errorf("decode part index: %w", err)
		return false
	}
	s.refs = refs
	if len(refs) == 0 {
		return false
	}
	columns, err := s.part.queryRow(header.indexRow(refs), Query{
		FieldIDs: s.query.FieldIDs,
		Start:    s.query.Start,
		End:      s.query.End,
	})
	if err != nil {
		s.err = err
		return false
	}
	s.pending = columns
	s.pendingAt = 0
	return len(s.pending) > 0
}

func (s *partColumnDataStream) loadNextSeriesRow() bool {
	for {
		row, ok := s.nextSeriesIndexRow()
		if !ok {
			return false
		}
		recordIndexRowRead(s.query)
		indexRow, err := s.part.readIndexRowBlock(row.IndexRef)
		if err != nil {
			s.err = err
			return false
		}
		if !rowMatches(indexRow, s.query) {
			recordIndexRowSkipped(s.query)
			continue
		}
		columns, err := s.part.queryRow(indexRow, s.query)
		if err != nil {
			s.err = err
			return false
		}
		s.pending = columns
		s.pendingAt = 0
		if len(s.pending) > 0 {
			return true
		}
	}
}

func (s *partColumnDataStream) nextSeriesIndexRow() (seriesIndexRow, bool) {
	for s.seriesAt < len(s.seriesIDs) {
		seriesID := s.seriesIDs[s.seriesAt]
		s.seriesAt++
		for s.rowAt < len(s.part.seriesRows) && s.part.seriesRows[s.rowAt].SeriesID < seriesID {
			s.rowAt++
		}
		if s.rowAt >= len(s.part.seriesRows) {
			return seriesIndexRow{}, false
		}
		row := s.part.seriesRows[s.rowAt]
		if row.SeriesID != seriesID || !seriesIndexRowMatches(row, s.query) {
			continue
		}
		return row, true
	}
	return seriesIndexRow{}, false
}

func sortedQuerySeriesIDs(seriesSet map[uint64]struct{}) []uint64 {
	seriesIDs := make([]uint64, 0, len(seriesSet))
	for seriesID := range seriesSet {
		seriesIDs = append(seriesIDs, seriesID)
	}
	sort.Slice(seriesIDs, func(i int, j int) bool {
		return seriesIDs[i] < seriesIDs[j]
	})
	return seriesIDs
}

func queryContextErr(query Query) error {
	if query.Context == nil {
		return nil
	}
	return query.Context.Err()
}

func (s *partColumnDataStream) finishRows() {
	if err := s.rows.done(); err != nil {
		s.err = fmt.Errorf("decode part index: %w", err)
	}
	closeErr := s.Close()
	if s.err == nil {
		s.err = closeErr
	}
}

func skipIndexColumnRefs(stream *indexRowStream) error {
	if err := stream.skipColumnRefs(); err != nil {
		return fmt.Errorf("decode part index: %w", err)
	}
	return nil
}
