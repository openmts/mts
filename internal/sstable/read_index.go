package sstable

import (
	"fmt"
	"sort"

	"github.com/openmts/mts/internal/model"
)

func (p *Part) SeriesIDs(query Query) ([]uint64, error) {
	if query.End < query.Start {
		return []uint64{}, nil
	}
	if !partMatches(p.metadata.Part, p.metaRows, query) {
		return []uint64{}, nil
	}
	stream, payload, err := p.openIndexRowStream()
	if err != nil {
		return nil, err
	}
	ids, readErr := collectSeriesIDsFromStream(stream, query)
	payload.Release()
	if readErr != nil {
		return nil, readErr
	}
	return ids, nil
}

func (p *Part) queryIndexRows(query Query, seriesIDs []uint64) ([]model.ColumnData, error) {
	stream, payload, err := p.openIndexRowStream()
	if err != nil {
		return nil, err
	}
	columns, readErr := p.queryIndexRowStream(stream, query, seriesIDs)
	payload.Release()
	if readErr != nil {
		return nil, readErr
	}
	sortColumns(columns)
	return columns, nil
}

func (p *Part) querySeriesIndexRows(query Query, seriesIDs []uint64) ([]model.ColumnData, error) {
	rows := p.matchingSeriesIndexRows(query, seriesIDs)
	if len(rows) == 0 {
		return []model.ColumnData{}, nil
	}
	columns := make([]model.ColumnData, 0, len(rows))
	for _, row := range rows {
		recordIndexRowRead(query)
		indexRow, err := p.readIndexRowBlock(row.IndexRef)
		if err != nil {
			return nil, err
		}
		if !rowMatches(indexRow, query) {
			recordIndexRowSkipped(query)
			continue
		}
		got, err := p.queryRow(indexRow, query)
		if err != nil {
			return nil, err
		}
		columns = append(columns, got...)
	}
	sortColumns(columns)
	return columns, nil
}

func (p *Part) matchingSeriesIndexRows(query Query, seriesIDs []uint64) []seriesIndexRow {
	if len(seriesIDs) == 0 || len(p.seriesRows) == 0 {
		return nil
	}
	rows := make([]seriesIndexRow, 0, len(seriesIDs))
	for _, seriesID := range seriesIDs {
		index := sort.Search(len(p.seriesRows), func(index int) bool {
			return p.seriesRows[index].SeriesID >= seriesID
		})
		if index >= len(p.seriesRows) || p.seriesRows[index].SeriesID != seriesID {
			continue
		}
		row := p.seriesRows[index]
		if !seriesIndexRowMatches(row, query) {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func seriesIndexRowMatches(row seriesIndexRow, query Query) bool {
	if row.MaxTime < query.Start || row.MinTime > query.End {
		return false
	}
	if len(query.FieldIDs) == 0 {
		return true
	}
	for _, fieldID := range row.FieldIDs {
		if _, ok := query.FieldIDs[fieldID]; ok {
			return true
		}
	}
	return false
}

func (p *Part) readIndexRowBlock(ref blockRef) (indexRow, error) {
	payload, err := p.readBlockPayload(indexFile, ref)
	if err != nil {
		return indexRow{}, err
	}
	if p.stats != nil {
		p.stats.IndexRowsRead++
	}
	rows, err := decodeIndexRows(payload.Bytes())
	payload.Release()
	if err != nil {
		return indexRow{}, fmt.Errorf("decode part index row: %w", err)
	}
	if len(rows) != 1 {
		return indexRow{}, fmt.Errorf("decode part index row: got %d rows, want 1", len(rows))
	}
	return rows[0], nil
}

func (p *Part) openIndexRowStream() (*indexRowStream, blockPayload, error) {
	payload, err := p.readBlockPayload(indexFile, p.metadata.IndexRef)
	if err != nil {
		return nil, blockPayload{}, err
	}
	stream, err := newIndexRowStream(payload.Bytes())
	if err != nil {
		payload.Release()
		return nil, blockPayload{}, fmt.Errorf("decode part index: %w", err)
	}
	return stream, payload, nil
}

func collectSeriesIDsFromStream(stream *indexRowStream, query Query) ([]uint64, error) {
	ids := make([]uint64, 0)
	for {
		header, ok, err := stream.nextHeader()
		if err != nil {
			return nil, fmt.Errorf("decode part index: %w", err)
		}
		if !ok {
			break
		}
		if rowHeaderMatches(header, query) {
			ids = append(ids, header.seriesID)
		}
		if err := stream.skipColumnRefs(); err != nil {
			return nil, fmt.Errorf("decode part index: %w", err)
		}
	}
	if err := stream.done(); err != nil {
		return nil, fmt.Errorf("decode part index: %w", err)
	}
	return ids, nil
}

func (p *Part) queryIndexRowStream(
	stream *indexRowStream,
	query Query,
	seriesIDs []uint64,
) ([]model.ColumnData, error) {
	columns := make([]model.ColumnData, 0)
	refs := make([]columnRef, 0, 16)
	for {
		header, ok, err := stream.nextHeader()
		if err != nil {
			return nil, fmt.Errorf("decode part index: %w", err)
		}
		if !ok {
			break
		}
		if !rowHeaderMatches(header, query) || !containsSortedSeriesIDOrAll(seriesIDs, header.seriesID) {
			recordIndexRowSkipped(query)
			if err := stream.skipColumnRefs(); err != nil {
				return nil, fmt.Errorf("decode part index: %w", err)
			}
			continue
		}
		recordIndexRowRead(query)
		got, nextRefs, err := p.queryIndexRowFromStream(stream, header, query, refs)
		if err != nil {
			return nil, err
		}
		refs = nextRefs
		columns = append(columns, got...)
	}
	if err := stream.done(); err != nil {
		return nil, fmt.Errorf("decode part index: %w", err)
	}
	return columns, nil
}

func (p *Part) queryIndexRowFromStream(
	stream *indexRowStream,
	header indexRowHeader,
	query Query,
	refs []columnRef,
) ([]model.ColumnData, []columnRef, error) {
	refs, err := stream.appendFilteredColumnRefs(refs, query.FieldIDs)
	if err != nil {
		return nil, refs, fmt.Errorf("decode part index: %w", err)
	}
	if len(refs) == 0 {
		return nil, refs, nil
	}
	columns, err := p.queryRow(header.indexRow(refs), Query{
		FieldIDs: query.FieldIDs,
		Start:    query.Start,
		End:      query.End,
	})
	return columns, refs, err
}

func (p *Part) loadIndexRows() ([]indexRow, error) {
	payload, err := p.readBlockPayload(indexFile, p.metadata.IndexRef)
	if err != nil {
		return nil, err
	}
	rows, err := decodeIndexRows(payload.Bytes())
	payload.Release()
	if err != nil {
		return nil, fmt.Errorf("decode part index: %w", err)
	}
	return rows, nil
}

func (p *Part) queryRow(row indexRow, query Query) ([]model.ColumnData, error) {
	columns := make([]model.ColumnData, 0, len(row.Columns))
	var (
		rowTimestamps []int64
		timeLoaded    bool
	)
	for _, ref := range row.Columns {
		if !containsField(query.FieldIDs, ref.FieldID) {
			continue
		}
		column, loaded, err := p.readValueColumnLazyTime(row.SeriesID, ref, row.TimeRef, &rowTimestamps, &timeLoaded, query)
		if err != nil {
			return nil, err
		}
		_ = loaded
		if len(column.Samples) > 0 {
			recordSamplesRead(query, len(column.Samples))
			columns = append(columns, column)
		}
	}
	return columns, nil
}
