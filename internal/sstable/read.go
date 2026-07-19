package sstable

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryexec"
	"github.com/openmts/mts/internal/storagefs"
)

func OpenPart(path string) (*Part, error) {
	return openPart(path, true, true)
}

func OpenPartTrusted(path string) (*Part, error) {
	// compact/flush 热路径：关闭 page 解码缓存，避免全量 series 扫描灌满缓存抬升 RSS。
	return openPart(path, false, false)
}

func openPart(path string, validateDeep bool, enablePageCache bool) (*Part, error) {
	meta, err := loadPartMetadata(path)
	if err != nil {
		return nil, err
	}
	files, err := openPartReadFiles(path)
	if err != nil {
		return nil, err
	}
	metaRows, err := loadPartMetaIndex(path, files, meta.MetaIndexRef)
	if err != nil {
		closeErr := files.close()
		if closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	seriesRows, err := loadPartSeriesIndex(path, files, meta.SeriesIndexRef)
	if err != nil {
		closeErr := files.close()
		if closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	componentSizes, err := loadPartComponentSizes(path, meta, files, validateDeep)
	if err != nil {
		closeErr := files.close()
		if closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	part := &Part{
		path:           filepath.Clean(path),
		metadata:       meta,
		metaRows:       metaRows,
		seriesRows:     seriesRows,
		files:          files,
		componentSizes: componentSizes,
		pageCache:      newPageCacheIfEnabled(enablePageCache),
	}
	if err := validateOpenedPart(part, validateDeep); err != nil {
		closeErr := files.close()
		if closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return part, nil
}

func loadPartComponentSizes(
	path string,
	meta metadata,
	files *partReadFiles,
	verifyMissing bool,
) (map[string]int64, error) {
	clean := filepath.Clean(path)
	names := metadataComponents(meta.Components)
	if len(meta.ComponentSizes) > 0 {
		sizes := make(map[string]int64, len(names))
		for _, name := range names {
			if name == metadataFile {
				if size, ok := meta.ComponentSizes[name]; ok {
					sizes[name] = size
				} else {
					sizes[name] = 0
				}
				continue
			}
			if size, ok := meta.ComponentSizes[name]; ok {
				if verifyMissing {
					if err := ensurePartComponentPresent(clean, name, files); err != nil {
						return nil, fmt.Errorf("validate part component %s: %w", name, err)
					}
				}
				sizes[name] = size
				continue
			}
			// 元数据缺 size 时回退 Stat。
			size, err := partComponentSize(clean, name, files)
			if err != nil {
				return nil, fmt.Errorf("validate part component %s: %w", name, err)
			}
			sizes[name] = size
		}
		return sizes, nil
	}
	sizes := make(map[string]int64, len(names))
	for _, name := range names {
		size, err := partComponentSize(clean, name, files)
		if err != nil {
			return nil, fmt.Errorf("validate part component %s: %w", name, err)
		}
		sizes[name] = size
	}
	return sizes, nil
}

func ensurePartComponentPresent(path string, name string, files *partReadFiles) error {
	if name == metadataFile {
		info, err := storagefs.Stat(filepath.Join(path, metadataFile))
		if err != nil {
			return err
		}
		if info.IsDir() {
			return fmt.Errorf("component is directory")
		}
		return nil
	}
	if sectionSize, ok := logicalComponentSize(files, name); ok {
		if sectionSize < 0 {
			return fmt.Errorf("component is directory")
		}
		return nil
	}
	info, err := storagefs.Stat(filepath.Join(path, name))
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("component is directory")
	}
	return nil
}

func partComponentSize(path string, name string, files *partReadFiles) (int64, error) {
	if name == metadataFile {
		info, err := storagefs.Stat(filepath.Join(path, metadataFile))
		if err != nil {
			return 0, err
		}
		if info.IsDir() {
			return 0, fmt.Errorf("component is directory")
		}
		return info.Size(), nil
	}
	if size, ok := logicalComponentSize(files, name); ok {
		return size, nil
	}
	info, err := storagefs.Stat(filepath.Join(path, name))
	if err != nil {
		return 0, err
	}
	if info.IsDir() {
		return 0, fmt.Errorf("component is directory")
	}
	return info.Size(), nil
}

func logicalComponentSize(files *partReadFiles, name string) (int64, bool) {
	if files == nil {
		return 0, false
	}
	if section, ok := files.sections[name]; ok {
		return section.Size, true
	}
	switch name {
	case indexFile:
		if files.index != nil {
			return files.index.Size(), true
		}
	case timestampsFile:
		if files.timestamps != nil {
			return files.timestamps.Size(), true
		}
	case valuesFile:
		if files.values != nil {
			return files.values.Size(), true
		}
	}
	return 0, false
}

func sectionReaderForComponent(name string, files *partReadFiles) *sectionReader {
	if files == nil {
		return nil
	}
	switch name {
	case indexFile:
		return files.index
	case timestampsFile:
		return files.timestamps
	case valuesFile:
		return files.values
	default:
		return nil
	}
}

func openPartReadFiles(path string) (*partReadFiles, error) {
	clean := filepath.Clean(path)
	packPath := filepath.Join(clean, packFile)
	if _, err := storagefs.Stat(packPath); err == nil {
		return openPartReadFilesFromPack(clean)
	}
	return openPartReadFilesLegacy(clean)
}

func openPartReadFilesFromPack(path string) (*partReadFiles, error) {
	pack, sections, err := openPartPack(path)
	if err != nil {
		return nil, err
	}
	index, err := packSectionFile(pack, sections, indexFile)
	if err != nil {
		closeErr := pack.Close()
		return nil, errors.Join(err, closeErr)
	}
	timestamps, err := packSectionFile(pack, sections, timestampsFile)
	if err != nil {
		closeErr := pack.Close()
		return nil, errors.Join(err, closeErr)
	}
	values, err := packSectionFile(pack, sections, valuesFile)
	if err != nil {
		closeErr := pack.Close()
		return nil, errors.Join(err, closeErr)
	}
	return &partReadFiles{
		pack:       pack,
		sections:   sections,
		index:      index,
		timestamps: timestamps,
		values:     values,
	}, nil
}

func openPartReadFilesLegacy(path string) (*partReadFiles, error) {
	indexFileHandle, err := storagefs.Open(filepath.Join(path, indexFile))
	if err != nil {
		return nil, fmt.Errorf("open part index: %w", err)
	}
	timestampsFileHandle, err := storagefs.Open(filepath.Join(path, timestampsFile))
	if err != nil {
		closeErr := indexFileHandle.Close()
		return nil, errors.Join(fmt.Errorf("open part timestamps: %w", err), closeErr)
	}
	valuesFileHandle, err := storagefs.Open(filepath.Join(path, valuesFile))
	if err != nil {
		indexErr := indexFileHandle.Close()
		timeErr := timestampsFileHandle.Close()
		return nil, errors.Join(fmt.Errorf("open part values: %w", err), indexErr, timeErr)
	}
	indexInfo, err := indexFileHandle.Stat()
	if err != nil {
		return nil, closeLegacyOpenError(err, indexFileHandle, timestampsFileHandle, valuesFileHandle)
	}
	timeInfo, err := timestampsFileHandle.Stat()
	if err != nil {
		return nil, closeLegacyOpenError(err, indexFileHandle, timestampsFileHandle, valuesFileHandle)
	}
	valueInfo, err := valuesFileHandle.Stat()
	if err != nil {
		return nil, closeLegacyOpenError(err, indexFileHandle, timestampsFileHandle, valuesFileHandle)
	}
	return &partReadFiles{
		legacy: []*os.File{indexFileHandle, timestampsFileHandle, valuesFileHandle},
		sections: map[string]packSection{
			indexFile:      {Name: indexFile, Offset: 0, Size: indexInfo.Size()},
			timestampsFile: {Name: timestampsFile, Offset: 0, Size: timeInfo.Size()},
			valuesFile:     {Name: valuesFile, Offset: 0, Size: valueInfo.Size()},
		},
		index:      &sectionReader{file: indexFileHandle, base: 0, size: indexInfo.Size()},
		timestamps: &sectionReader{file: timestampsFileHandle, base: 0, size: timeInfo.Size()},
		values:     &sectionReader{file: valuesFileHandle, base: 0, size: valueInfo.Size()},
	}, nil
}

func closeLegacyOpenError(err error, files ...*os.File) error {
	errs := []error{err}
	for _, file := range files {
		if file == nil {
			continue
		}
		if closeErr := file.Close(); closeErr != nil {
			errs = append(errs, closeErr)
		}
	}
	return errors.Join(errs...)
}

func (f *partReadFiles) close() error {
	if f == nil {
		return nil
	}
	var errs []error
	if f.pack != nil {
		if err := closeFile(f.pack, "part pack"); err != nil {
			errs = append(errs, err)
		}
	}
	for _, file := range f.legacy {
		if err := closeFile(file, "part legacy component"); err != nil {
			errs = append(errs, err)
		}
	}
	f.pack = nil
	f.legacy = nil
	f.sections = nil
	f.index = nil
	f.timestamps = nil
	f.values = nil
	return errors.Join(errs...)
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
	if len(query.SeriesIDs) > 0 {
		seriesIDs := make([]uint64, 0, len(query.SeriesIDs))
		for seriesID := range query.SeriesIDs {
			seriesIDs = append(seriesIDs, seriesID)
		}
		sort.Slice(seriesIDs, func(i int, j int) bool {
			return seriesIDs[i] < seriesIDs[j]
		})
		return p.querySeriesIndexRows(query, seriesIDs)
	}
	return p.queryIndexRows(query, nil)
}

func (p *Part) ScanColumns(query Query) (queryexec.ColumnDataStream, error) {
	return newPartColumnDataStream(p, query)
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

func (p *Part) readValueColumnLazyTime(
	seriesID uint64,
	ref columnRef,
	timeRef blockRef,
	rowTimestamps *[]int64,
	timeLoaded *bool,
	query Query,
) (model.ColumnData, bool, error) {
	if p.stats != nil {
		p.stats.ValueBlocksRead++
	}
	recordValueBlockRead(query)
	payload, err := p.readBlockPayload(valuesFile, ref.ValueRef)
	if err != nil {
		return model.ColumnData{}, false, err
	}
	data := payload.Bytes()
	if len(data) > 0 && data[0] == valueEncodingPageIndex {
		if err := p.ensureRowTimestamps(timeRef, rowTimestamps, timeLoaded, query); err != nil {
			payload.Release()
			return model.ColumnData{}, false, err
		}
		column, err := p.readValuePagesFromIndexPayload(seriesID, data, *rowTimestamps, query)
		payload.Release()
		if err != nil {
			return model.ColumnData{}, false, fmt.Errorf("decode value page index: %w", err)
		}
		return column, *timeLoaded, nil
	}
	// 压缩 value page 自带 timestamps，无需加载行级 time block。
	if len(data) > 0 && data[0] == valueEncodingPageCompressed {
		cacheKey := pageDecodeKey{
			offset: ref.ValueRef.Offset,
			size:   ref.ValueRef.Size,
			start:  query.Start,
			end:    query.End,
		}
		if samples, ok := p.pageCache.get(cacheKey); ok {
			payload.Release()
			return columnFromBlock(seriesID, valueBlock{Samples: samples}), *timeLoaded, nil
		}
		block, err := unmarshalCompressedValueBlock(data, query)
		payload.Release()
		if err != nil {
			return model.ColumnData{}, false, fmt.Errorf("decode value block: %w", err)
		}
		p.pageCache.put(cacheKey, block.Samples)
		return columnFromBlock(seriesID, block), *timeLoaded, nil
	}
	if err := p.ensureRowTimestamps(timeRef, rowTimestamps, timeLoaded, query); err != nil {
		payload.Release()
		return model.ColumnData{}, false, err
	}
	block, err := unmarshalValueBlockWithTimestamps(data, *rowTimestamps, query)
	payload.Release()
	if err != nil {
		return model.ColumnData{}, false, fmt.Errorf("decode value block: %w", err)
	}
	return columnFromBlock(seriesID, block), *timeLoaded, nil
}

func (p *Part) ensureRowTimestamps(
	timeRef blockRef,
	rowTimestamps *[]int64,
	timeLoaded *bool,
	query Query,
) error {
	if *timeLoaded {
		return nil
	}
	recordTimeBlockRead(query)
	timeBlock, err := p.readTimeBlock(timeRef)
	if err != nil {
		return err
	}
	*rowTimestamps = timeBlock.Timestamps
	*timeLoaded = true
	return nil
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
	recordValueBlockRead(query)
	payload, err := p.readBlockPayload(valuesFile, ref.ValueRef)
	if err != nil {
		return model.ColumnData{}, err
	}
	if len(payload.Bytes()) > 0 && payload.Bytes()[0] == valueEncodingPageIndex {
		column, err := p.readValuePagesFromIndexPayload(seriesID, payload.Bytes(), rowTimestamps, query)
		payload.Release()
		if err != nil {
			return model.ColumnData{}, fmt.Errorf("decode value page index: %w", err)
		}
		return column, nil
	}
	block, err := unmarshalValueBlockWithTimestamps(payload.Bytes(), rowTimestamps, query)
	payload.Release()
	if err != nil {
		return model.ColumnData{}, fmt.Errorf("decode value block: %w", err)
	}
	return columnFromBlock(seriesID, block), nil
}

func (p *Part) readValuePagesFromIndexPayload(
	seriesID uint64,
	payload []byte,
	rowTimestamps []int64,
	query Query,
) (model.ColumnData, error) {
	header, fullPages, fullRange, matches, err := scanValuePageIndexCoverage(payload, query)
	if err != nil {
		return model.ColumnData{}, err
	}
	capacity := matchingValuePageCapacity(header, matches)
	if fullRange {
		capacity = header.count
	}
	column := model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   header.fieldID,
		FieldType: header.fieldType,
		Samples:   make([]model.VersionedSample, 0, capacity),
	}
	if matches == 0 {
		recordValuePagesSkipped(query, header.pageCount)
		return column, nil
	}
	if boundaryModeEnabled(query.Boundary) {
		pages, err := matchingBoundaryPageRefs(payload, fullPages, fullRange, query)
		if err != nil {
			return model.ColumnData{}, err
		}
		selected := selectBoundaryPageRefs(pages, query.Boundary)
		recordValuePagesSkipped(query, header.pageCount-len(selected))
		column.Samples = make([]model.VersionedSample, 0, matchingValuePageCapacity(header, len(selected)))
		return p.readValuePagesFromRefs(column, selected, rowTimestamps, query)
	}
	recordValuePagesSkipped(query, header.pageCount-matches)
	if fullRange {
		return p.readValuePagesFromRefs(column, fullPages, rowTimestamps, query)
	}
	reader := newBlockReader(payload)
	if _, err := readValuePageIndexHeader(reader); err != nil {
		return model.ColumnData{}, err
	}
	for range header.pageCount {
		if err := queryContextErr(query); err != nil {
			return model.ColumnData{}, err
		}
		page, err := readValuePageRef(reader, header.fieldType)
		if err != nil {
			return model.ColumnData{}, err
		}
		if !valuePageMatchesQuery(page, header.fieldID, header.fieldType, query) {
			continue
		}
		block, err := p.readValuePage(page.Ref, rowTimestamps, query)
		if err != nil {
			return model.ColumnData{}, err
		}
		column.Samples = append(column.Samples, block.Samples...)
	}
	if err := reader.done("value page index"); err != nil {
		return model.ColumnData{}, err
	}
	if !samplesSorted(column.Samples) {
		sortSamples(column.Samples)
	}
	return column, nil
}

func (p *Part) readValuePagesFromRefs(
	column model.ColumnData,
	pages []valuePageRef,
	rowTimestamps []int64,
	query Query,
) (model.ColumnData, error) {
	for _, page := range pages {
		if err := queryContextErr(query); err != nil {
			return model.ColumnData{}, err
		}
		block, err := p.readValuePage(page.Ref, rowTimestamps, query)
		if err != nil {
			return model.ColumnData{}, err
		}
		column.Samples = append(column.Samples, block.Samples...)
	}
	if !samplesSorted(column.Samples) {
		sortSamples(column.Samples)
	}
	return column, nil
}

func (p *Part) readValuePage(ref blockRef, rowTimestamps []int64, query Query) (valueBlock, error) {
	cacheKey := pageDecodeKey{offset: ref.Offset, size: ref.Size, start: query.Start, end: query.End}
	if samples, ok := p.pageCache.get(cacheKey); ok {
		if p.stats != nil {
			p.stats.ValuePagesRead++
		}
		recordValuePageRead(query)
		return valueBlock{Samples: samples}, nil
	}
	payload, err := p.readBlockPayload(valuesFile, ref)
	if err != nil {
		return valueBlock{}, err
	}
	if p.stats != nil {
		p.stats.ValuePagesRead++
	}
	recordValuePageRead(query)
	block, err := unmarshalValueBlockWithTimestamps(payload.Bytes(), rowTimestamps, query)
	payload.Release()
	if err != nil {
		return valueBlock{}, fmt.Errorf("decode value page: %w", err)
	}
	p.pageCache.put(cacheKey, block.Samples)
	return block, nil
}

func (p *Part) readBlock(name string, ref blockRef) ([]byte, error) {
	if p.files == nil {
		return readLogicalComponentBlock(p.path, nil, name, ref)
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
		data, err := readLogicalComponentBlock(p.path, nil, name, ref)
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
	data, err := storagefs.ReadFile(filepath.Join(path, metadataFile))
	if err != nil {
		return metadata{}, fmt.Errorf("read part metadata: %w", err)
	}
	meta, err := decodeMetadata(data)
	if err != nil {
		return metadata{}, fmt.Errorf("decode part metadata: %w", err)
	}
	return meta, nil
}

func loadPartMetaIndex(path string, files *partReadFiles, ref blockRef) ([]metaIndexRow, error) {
	payload, err := readLogicalComponentBlock(path, files, metaindexFile, ref)
	if err != nil {
		return nil, err
	}
	rows, err := decodeMetaIndexRows(payload)
	if err != nil {
		return nil, fmt.Errorf("decode part metaindex: %w", err)
	}
	return rows, nil
}

func loadPartSeriesIndex(path string, files *partReadFiles, ref blockRef) ([]seriesIndexRow, error) {
	payload, err := readLogicalComponentBlock(path, files, seriesIndexFile, ref)
	if err != nil {
		return nil, err
	}
	rows, err := decodeSeriesIndexRows(payload)
	if err != nil {
		return nil, fmt.Errorf("decode part series index: %w", err)
	}
	return rows, nil
}

func readLogicalComponentBlock(path string, files *partReadFiles, name string, ref blockRef) ([]byte, error) {
	if files != nil && files.pack != nil {
		section, ok := files.sections[name]
		if !ok {
			return nil, fmt.Errorf("pack section %s missing", name)
		}
		reader := &sectionReader{file: files.pack, base: section.Offset, size: section.Size}
		return readBlockFrom(reader, ref)
	}
	packPath := filepath.Join(filepath.Clean(path), packFile)
	if _, err := storagefs.Stat(packPath); err == nil {
		pack, sections, openErr := openPartPack(path)
		if openErr != nil {
			return nil, openErr
		}
		defer func() { _ = pack.Close() }()
		section, ok := sections[name]
		if !ok {
			return nil, fmt.Errorf("pack section %s missing", name)
		}
		reader := &sectionReader{file: pack, base: section.Offset, size: section.Size}
		return readBlockFrom(reader, ref)
	}
	return readBlock(filepath.Join(path, name), ref)
}

func timeBlockFromTimestamps(timestamps []int64) timeBlock {
	block := timeBlock{Encoding: "binary-delta", Timestamps: append([]int64{}, timestamps...)}
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

func rowHeaderMatches(header indexRowHeader, query Query) bool {
	if !containsSeries(query.SeriesIDs, header.seriesID) {
		return false
	}
	return header.maxTime >= query.Start && header.minTime <= query.End
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
