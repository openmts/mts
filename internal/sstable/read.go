package sstable

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"codeberg.org/mts/mts/internal/model"
)

func OpenPart(path string) (*Part, error) {
	data, err := os.ReadFile(filepath.Join(path, metadataFile))
	if err != nil {
		return nil, fmt.Errorf("read part metadata: %w", err)
	}
	var meta metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("decode part metadata: %w", err)
	}
	payload, err := readBlock(filepath.Join(path, indexFile), meta.IndexRef)
	if err != nil {
		return nil, err
	}
	var rows []indexRow
	if err := json.Unmarshal(payload, &rows); err != nil {
		return nil, fmt.Errorf("decode part index: %w", err)
	}
	return &Part{
		path:     filepath.Clean(path),
		metadata: meta,
		rows:     rows,
	}, nil
}

func (p *Part) Meta() PartMeta {
	return p.metadata.Part
}

func (p *Part) Query(query Query) ([]model.ColumnData, error) {
	if query.End < query.Start {
		return []model.ColumnData{}, nil
	}
	columns := make([]model.ColumnData, 0)
	for _, row := range p.rows {
		if !rowMatches(row, query) {
			continue
		}
		got, err := p.queryRow(row, query)
		if err != nil {
			return nil, err
		}
		columns = append(columns, got...)
	}
	sortColumns(columns)
	return columns, nil
}

func (p *Part) queryRow(row indexRow, query Query) ([]model.ColumnData, error) {
	if _, err := p.readTimeBlock(row.TimeRef); err != nil {
		return nil, err
	}
	columns := make([]model.ColumnData, 0, len(row.Columns))
	for _, ref := range row.Columns {
		if !containsField(query.FieldIDs, ref.FieldID) {
			continue
		}
		column, err := p.readValueColumn(row.SeriesID, ref, query)
		if err != nil {
			return nil, err
		}
		if len(column.Samples) > 0 {
			columns = append(columns, column)
		}
	}
	return columns, nil
}

func (p *Part) readTimeBlock(ref blockRef) (timeBlock, error) {
	payload, err := readBlock(filepath.Join(p.path, timestampsFile), ref)
	if err != nil {
		return timeBlock{}, err
	}
	var block timeBlock
	if err := json.Unmarshal(payload, &block); err != nil {
		return timeBlock{}, fmt.Errorf("decode time block: %w", err)
	}
	if block.Encoding != "plain-int64-v1" {
		return timeBlock{}, fmt.Errorf("unknown time encoding")
	}
	return block, nil
}

func (p *Part) readValueColumn(
	seriesID uint64,
	ref columnRef,
	query Query,
) (model.ColumnData, error) {
	payload, err := readBlock(filepath.Join(p.path, valuesFile), ref.ValueRef)
	if err != nil {
		return model.ColumnData{}, err
	}
	var block valueBlock
	if err := json.Unmarshal(payload, &block); err != nil {
		return model.ColumnData{}, fmt.Errorf("decode value block: %w", err)
	}
	if block.Encoding != "plain-json-v1" {
		return model.ColumnData{}, fmt.Errorf("unknown value encoding")
	}
	return filterSamples(seriesID, block, query), nil
}

func filterSamples(seriesID uint64, block valueBlock, query Query) model.ColumnData {
	column := model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   block.FieldID,
		FieldType: block.FieldType,
		Samples:   make([]model.VersionedSample, 0, len(block.Samples)),
	}
	for _, sample := range block.Samples {
		if sample.Timestamp < query.Start || sample.Timestamp > query.End {
			continue
		}
		column.Samples = append(column.Samples, sample)
	}
	sortSamples(column.Samples)
	return column
}

func rowMatches(row indexRow, query Query) bool {
	if !containsSeries(query.SeriesIDs, row.SeriesID) {
		return false
	}
	return row.MaxTime >= query.Start && row.MinTime <= query.End
}

func containsSeries(filter map[uint64]struct{}, seriesID uint64) bool {
	if len(filter) == 0 {
		return true
	}
	_, ok := filter[seriesID]
	return ok
}

func containsField(filter map[uint32]struct{}, fieldID uint32) bool {
	if len(filter) == 0 {
		return true
	}
	_, ok := filter[fieldID]
	return ok
}

func sortColumns(columns []model.ColumnData) {
	sort.Slice(columns, func(i, j int) bool {
		if columns[i].SeriesID != columns[j].SeriesID {
			return columns[i].SeriesID < columns[j].SeriesID
		}
		return columns[i].FieldID < columns[j].FieldID
	})
}
