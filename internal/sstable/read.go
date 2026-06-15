package sstable

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"codeberg.org/mts/mts/internal/model"
)

func OpenPart(path string) (*Part, error) {
	meta, err := loadPartMetadata(path)
	if err != nil {
		return nil, err
	}
	metaRows, err := loadPartMetaIndex(path, meta.MetaIndexRef)
	if err != nil {
		return nil, err
	}
	return &Part{
		path:     filepath.Clean(path),
		metadata: meta,
		metaRows: metaRows,
	}, nil
}

func (p *Part) Meta() PartMeta {
	return p.metadata.Part
}

func (p *Part) Query(query Query) ([]model.ColumnData, error) {
	if query.End < query.Start {
		return []model.ColumnData{}, nil
	}
	if !partMatches(p.metadata.Part, p.metaRows, query) {
		return []model.ColumnData{}, nil
	}
	rows, err := p.loadIndexRows()
	if err != nil {
		return nil, err
	}
	columns := make([]model.ColumnData, 0)
	for _, row := range rows {
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

func (p *Part) loadIndexRows() ([]indexRow, error) {
	payload, err := readBlock(filepath.Join(p.path, indexFile), p.metadata.IndexRef)
	if err != nil {
		return nil, err
	}
	rows, err := decodeIndexRows(payload)
	if err != nil {
		return nil, fmt.Errorf("decode part index: %w", err)
	}
	return rows, nil
}

func (p *Part) queryRow(row indexRow, query Query) ([]model.ColumnData, error) {
	timeBlock, err := p.readTimeBlock(row.TimeRef)
	if err != nil {
		return nil, err
	}
	columns := make([]model.ColumnData, 0, len(row.Columns))
	for _, ref := range row.Columns {
		if !containsField(query.FieldIDs, ref.FieldID) {
			continue
		}
		column, err := p.readValueColumn(row.SeriesID, ref, timeBlock.Timestamps, query)
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
	if p.stats != nil {
		p.stats.TimeBlocksRead++
	}
	payload, err := readBlock(filepath.Join(p.path, timestampsFile), ref)
	if err != nil {
		return timeBlock{}, err
	}
	timestamps, err := unmarshalTimeBlock(payload)
	if err != nil {
		return timeBlock{}, fmt.Errorf("decode time block: %w", err)
	}
	return timeBlockFromTimestamps(timestamps), nil
}

func (p *Part) readValueColumn(
	seriesID uint64,
	ref columnRef,
	rowTimestamps []int64,
	query Query,
) (model.ColumnData, error) {
	if p.stats != nil {
		p.stats.ValueBlocksRead++
	}
	payload, err := readBlock(filepath.Join(p.path, valuesFile), ref.ValueRef)
	if err != nil {
		return model.ColumnData{}, err
	}
	block, err := unmarshalValueBlockWithTimestamps(payload, rowTimestamps, query)
	if err != nil {
		return model.ColumnData{}, fmt.Errorf("decode value block: %w", err)
	}
	return columnFromBlock(seriesID, block), nil
}

func (p *Part) resetReadStatsForTest() *readStats {
	p.stats = &readStats{}
	return p.stats
}

func partMatches(meta PartMeta, rows []metaIndexRow, query Query) bool {
	if meta.MaxTime < query.Start || meta.MinTime > query.End {
		return false
	}
	if !partSeriesMatches(meta, query.SeriesIDs) {
		return false
	}
	return partFieldsMatch(rows, query.FieldIDs)
}

func partSeriesMatches(meta PartMeta, filter map[uint64]struct{}) bool {
	if len(filter) == 0 {
		return true
	}
	for seriesID := range filter {
		if seriesID >= meta.MinSeriesID && seriesID <= meta.MaxSeriesID {
			return true
		}
	}
	return false
}

func partFieldsMatch(rows []metaIndexRow, filter map[uint32]struct{}) bool {
	if len(filter) == 0 || len(rows) == 0 {
		return true
	}
	for _, row := range rows {
		for _, fieldID := range row.FieldIDs {
			if _, ok := filter[fieldID]; ok {
				return true
			}
		}
	}
	return false
}

func loadPartMetadata(path string) (metadata, error) {
	data, err := os.ReadFile(filepath.Join(path, metadataFile))
	if err != nil {
		if os.IsNotExist(err) {
			return metadata{}, rejectLegacyMetadata(path)
		}
		return metadata{}, fmt.Errorf("read part metadata: %w", err)
	}
	meta, err := decodeMetadata(data)
	if err != nil {
		return metadata{}, fmt.Errorf("decode part metadata: %w", err)
	}
	if meta.FormatVersion != partFormatVersion {
		return metadata{}, fmt.Errorf("unsupported part metadata format version %d", meta.FormatVersion)
	}
	return meta, nil
}

func rejectLegacyMetadata(path string) error {
	legacyPath := filepath.Join(path, legacyMetadataFile)
	if _, err := os.Stat(legacyPath); err == nil {
		return fmt.Errorf("part metadata legacy JSON format is unsupported: %s", legacyPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat legacy part metadata: %w", err)
	}
	return fmt.Errorf("read part metadata: %w", os.ErrNotExist)
}

func loadPartMetaIndex(path string, ref blockRef) ([]metaIndexRow, error) {
	payload, err := readBlock(filepath.Join(path, metaindexFile), ref)
	if err != nil {
		return nil, err
	}
	rows, err := decodeMetaIndexRows(payload)
	if err != nil {
		return nil, fmt.Errorf("decode part metaindex: %w", err)
	}
	return rows, nil
}

func timeBlockFromTimestamps(timestamps []int64) timeBlock {
	block := timeBlock{Encoding: "binary-v2", Timestamps: append([]int64{}, timestamps...)}
	if len(timestamps) > 0 {
		block.MinTime = timestamps[0]
		block.MaxTime = timestamps[len(timestamps)-1]
	}
	return block
}

func columnFromBlock(seriesID uint64, block valueBlock) model.ColumnData {
	column := model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   block.FieldID,
		FieldType: block.FieldType,
		Samples:   block.Samples,
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
