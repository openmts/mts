package sstable

import (
	"sort"

	"github.com/openmts/mts/internal/model"
)

// SeriesBatchReader caches decoded index rows for repeated compaction batches.
type SeriesBatchReader struct {
	part  *Part
	query Query
	rows  []indexRow
	ids   []uint64
}

func NewSeriesBatchReader(part *Part, query Query) (*SeriesBatchReader, error) {
	if query.End < query.Start || !partMatches(part.metadata.Part, part.metaRows, query) {
		return &SeriesBatchReader{part: part, query: query}, nil
	}
	rows, err := part.loadIndexRows()
	if err != nil {
		return nil, err
	}
	reader := &SeriesBatchReader{
		part:  part,
		query: query,
		rows:  rows,
		ids:   make([]uint64, 0, len(rows)),
	}
	var previous uint64
	havePrevious := false
	for _, row := range rows {
		if !rowMatches(row, query) {
			continue
		}
		if havePrevious && row.SeriesID == previous {
			continue
		}
		reader.ids = append(reader.ids, row.SeriesID)
		previous = row.SeriesID
		havePrevious = true
	}
	return reader, nil
}

func (p *Part) NewSeriesBatchReader(query Query) (*SeriesBatchReader, error) {
	return NewSeriesBatchReader(p, query)
}

func (p *Part) QuerySeriesIDs(query Query, seriesIDs []uint64) ([]model.ColumnData, error) {
	if query.End < query.Start || len(seriesIDs) == 0 {
		return []model.ColumnData{}, nil
	}
	if !partMatches(p.metadata.Part, p.metaRows, query) {
		return []model.ColumnData{}, nil
	}
	return p.querySeriesIndexRows(query, seriesIDs)
}

func (r *SeriesBatchReader) SeriesIDs() []uint64 {
	if r == nil || len(r.ids) == 0 {
		return []uint64{}
	}
	return append([]uint64(nil), r.ids...)
}

func (r *SeriesBatchReader) SeriesCount() int {
	if r == nil {
		return 0
	}
	return len(r.ids)
}

func (r *SeriesBatchReader) AppendSeriesIDs(dst []uint64) []uint64 {
	if r == nil || len(r.ids) == 0 {
		return dst
	}
	return append(dst, r.ids...)
}

func (r *SeriesBatchReader) QuerySeriesIDs(seriesIDs []uint64) ([]model.ColumnData, error) {
	if r == nil || len(seriesIDs) == 0 || len(r.rows) == 0 {
		return []model.ColumnData{}, nil
	}
	columns := make([]model.ColumnData, 0)
	for _, row := range r.rows {
		if !rowMatches(row, r.query) || !containsSortedSeriesID(seriesIDs, row.SeriesID) {
			continue
		}
		got, err := r.part.queryRow(row, Query{
			FieldIDs: r.query.FieldIDs,
			Start:    r.query.Start,
			End:      r.query.End,
		})
		if err != nil {
			return nil, err
		}
		columns = append(columns, got...)
	}
	sortColumns(columns)
	return columns, nil
}

func (r *SeriesBatchReader) QuerySeriesID(seriesID uint64) ([]model.ColumnData, error) {
	if r == nil || len(r.rows) == 0 {
		return []model.ColumnData{}, nil
	}
	columns := make([]model.ColumnData, 0)
	for _, row := range r.rows {
		if row.SeriesID < seriesID {
			continue
		}
		if row.SeriesID > seriesID {
			break
		}
		if !rowMatches(row, r.query) {
			continue
		}
		got, err := r.part.queryRow(row, Query{
			FieldIDs: r.query.FieldIDs,
			Start:    r.query.Start,
			End:      r.query.End,
		})
		if err != nil {
			return nil, err
		}
		columns = append(columns, got...)
	}
	sortColumns(columns)
	return columns, nil
}

func containsSortedSeriesID(ids []uint64, id uint64) bool {
	index := sort.Search(len(ids), func(index int) bool {
		return ids[index] >= id
	})
	return index < len(ids) && ids[index] == id
}

func containsSortedSeriesIDOrAll(ids []uint64, id uint64) bool {
	if len(ids) == 0 {
		return true
	}
	return containsSortedSeriesID(ids, id)
}
