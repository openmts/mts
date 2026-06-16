package sstable

import (
	"errors"
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
	files, err := openPartReadFiles(path)
	if err != nil {
		return nil, err
	}
	metaRows, err := loadPartMetaIndex(path, meta.MetaIndexRef)
	if err != nil {
		closeErr := files.close()
		if closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return &Part{
		path:     filepath.Clean(path),
		metadata: meta,
		metaRows: metaRows,
		files:    files,
	}, nil
}

func openPartReadFiles(path string) (*partReadFiles, error) {
	clean := filepath.Clean(path)
	index, err := os.Open(filepath.Join(clean, indexFile))
	if err != nil {
		return nil, fmt.Errorf("open part index: %w", err)
	}
	timestamps, err := os.Open(filepath.Join(clean, timestampsFile))
	if err != nil {
		closeErr := index.Close()
		return nil, errors.Join(fmt.Errorf("open part timestamps: %w", err), closeErr)
	}
	values, err := os.Open(filepath.Join(clean, valuesFile))
	if err != nil {
		indexErr := index.Close()
		timeErr := timestamps.Close()
		return nil, errors.Join(fmt.Errorf("open part values: %w", err), indexErr, timeErr)
	}
	return &partReadFiles{
		index:      index,
		timestamps: timestamps,
		values:     values,
	}, nil
}

func (f *partReadFiles) close() error {
	if f == nil {
		return nil
	}
	indexErr := closeFile(f.index, "part index")
	timeErr := closeFile(f.timestamps, "part timestamps")
	valueErr := closeFile(f.values, "part values")
	f.index = nil
	f.timestamps = nil
	f.values = nil
	return errors.Join(indexErr, timeErr, valueErr)
}

func closeFile(file *os.File, name string) error {
	if file == nil {
		return nil
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", name, err)
	}
	return nil
}

func (p *Part) Close() error {
	if p == nil {
		return nil
	}
	if err := p.files.close(); err != nil {
		return err
	}
	p.files = nil
	return nil
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
	payload, err := p.readBlockPayload(timestampsFile, ref)
	if err != nil {
		return timeBlock{}, err
	}
	timestamps, err := unmarshalTimeBlock(payload.Bytes())
	payload.Release()
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
	payload, err := p.readBlockPayload(valuesFile, ref.ValueRef)
	if err != nil {
		return model.ColumnData{}, err
	}
	if len(payload.Bytes()) > 0 && payload.Bytes()[0] == valueEncodingV4 {
		index, err := unmarshalValuePageIndex(payload.Bytes())
		payload.Release()
		if err != nil {
			return model.ColumnData{}, fmt.Errorf("decode value page index: %w", err)
		}
		return p.readValuePages(seriesID, index, rowTimestamps, query)
	}
	block, err := unmarshalValueBlockWithTimestamps(payload.Bytes(), rowTimestamps, query)
	payload.Release()
	if err != nil {
		return model.ColumnData{}, fmt.Errorf("decode value block: %w", err)
	}
	return columnFromBlock(seriesID, block), nil
}

func (p *Part) readValuePages(
	seriesID uint64,
	index valuePageIndex,
	rowTimestamps []int64,
	query Query,
) (model.ColumnData, error) {
	capacity := matchingValuePageSampleCapacity(index, query)
	column := model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   index.FieldID,
		FieldType: index.FieldType,
		Samples:   make([]model.VersionedSample, 0, capacity),
	}
	for _, page := range index.Pages {
		if page.MaxTime < query.Start || page.MinTime > query.End {
			continue
		}
		payload, err := p.readBlockPayload(valuesFile, page.Ref)
		if err != nil {
			return model.ColumnData{}, err
		}
		if p.stats != nil {
			p.stats.ValuePagesRead++
		}
		block, err := unmarshalValueBlockWithTimestamps(payload.Bytes(), rowTimestamps, query)
		payload.Release()
		if err != nil {
			return model.ColumnData{}, fmt.Errorf("decode value page: %w", err)
		}
		column.Samples = append(column.Samples, block.Samples...)
	}
	if !samplesSorted(column.Samples) {
		sortSamples(column.Samples)
	}
	return column, nil
}

func matchingValuePageSampleCapacity(index valuePageIndex, query Query) int {
	if len(index.Pages) == 0 || index.Count == 0 {
		return 0
	}
	matches := 0
	for _, page := range index.Pages {
		if page.MaxTime >= query.Start && page.MinTime <= query.End {
			matches++
		}
	}
	if matches == 0 {
		return 0
	}
	pageAverage := (index.Count + len(index.Pages) - 1) / len(index.Pages)
	capacity := matches * pageAverage
	if capacity > index.Count {
		return index.Count
	}
	return capacity
}

func (p *Part) readBlock(name string, ref blockRef) ([]byte, error) {
	if p.files == nil {
		return readBlock(filepath.Join(p.path, name), ref)
	}
	switch name {
	case indexFile:
		return readBlockFrom(p.files.index, ref)
	case timestampsFile:
		return readBlockFrom(p.files.timestamps, ref)
	case valuesFile:
		return readBlockFrom(p.files.values, ref)
	default:
		return nil, fmt.Errorf("unsupported part block file %q", name)
	}
}

func (p *Part) readBlockPayload(name string, ref blockRef) (blockPayload, error) {
	if p.files == nil {
		data, err := readBlock(filepath.Join(p.path, name), ref)
		if err != nil {
			return blockPayload{}, err
		}
		return blockPayload{data: data}, nil
	}
	switch name {
	case indexFile:
		return readBlockPayloadFrom(p.files.index, ref)
	case timestampsFile:
		return readBlockPayloadFrom(p.files.timestamps, ref)
	case valuesFile:
		return readBlockPayloadFrom(p.files.values, ref)
	default:
		return blockPayload{}, fmt.Errorf("unsupported part block file %q", name)
	}
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
	if !samplesSorted(column.Samples) {
		sortSamples(column.Samples)
	}
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
