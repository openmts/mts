package sstable

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/storagefs"
)

const valueBlockPageSamples = 256

type WriteOptions struct {
	Compression model.CompressionOptions
}

func WritePart(root string, level int, id string, columns []model.ColumnData) (PartMeta, error) {
	return WritePartWithOptions(root, level, id, columns, WriteOptions{})
}

func WritePartWithOptions(
	root string,
	level int,
	id string,
	columns []model.ColumnData,
	opts WriteOptions,
) (PartMeta, error) {
	if len(columns) == 0 {
		return PartMeta{}, fmt.Errorf("columns are empty")
	}
	partPath := filepath.Join(root, id)
	if err := storagefs.MkdirAll(partPath); err != nil {
		return PartMeta{}, err
	}
	files, err := openPartFiles(partPath)
	if err != nil {
		return PartMeta{}, err
	}
	rows, meta, writeErr := writeColumns(files, level, id, columns, opts)
	closeErr := files.close()
	if writeErr != nil {
		return PartMeta{}, writeErr
	}
	if closeErr != nil {
		return PartMeta{}, closeErr
	}
	if err := writePartIndexes(partPath, &meta, rows); err != nil {
		return PartMeta{}, err
	}
	if err := ensureStringsFile(partPath); err != nil {
		return PartMeta{}, err
	}
	meta.Part.Path = partPath
	return meta.Part, nil
}

type partFiles struct {
	timestamps  *os.File
	values      *os.File
	timeBlocks  *blockWriter
	valueBlocks *blockWriter
}

func openPartFiles(path string) (*partFiles, error) {
	timestamps, err := openWritable(filepath.Join(path, timestampsFile))
	if err != nil {
		return nil, err
	}
	values, err := openWritable(filepath.Join(path, valuesFile))
	if err != nil {
		closeErr := timestamps.Close()
		return nil, fmt.Errorf("open values file: %w close timestamps: %v", err, closeErr)
	}
	timeBlocks, err := newBlockWriter(timestamps)
	if err != nil {
		indexErr := timestamps.Close()
		valueErr := values.Close()
		return nil, fmt.Errorf("open block writer: %w close timestamps: %v close values: %v", err, indexErr, valueErr)
	}
	valueBlocks, err := newBlockWriter(values)
	if err != nil {
		indexErr := timestamps.Close()
		valueErr := values.Close()
		return nil, fmt.Errorf("open value block writer: %w close timestamps: %v close values: %v", err, indexErr, valueErr)
	}
	return &partFiles{
		timestamps:  timestamps,
		values:      values,
		timeBlocks:  timeBlocks,
		valueBlocks: valueBlocks,
	}, nil
}

func openWritable(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, storagefs.FileMode)
	if err != nil {
		return nil, fmt.Errorf("open writable file: %w", err)
	}
	return file, nil
}

func (f *partFiles) close() error {
	if err := f.timestamps.Close(); err != nil {
		closeErr := f.values.Close()
		return fmt.Errorf("close timestamps file: %w close values: %v", err, closeErr)
	}
	if err := f.values.Close(); err != nil {
		return fmt.Errorf("close values file: %w", err)
	}
	return nil
}

func writeColumns(
	files *partFiles,
	level int,
	id string,
	columns []model.ColumnData,
	opts WriteOptions,
) ([]indexRow, metadata, error) {
	groups := groupColumnRuns(columns)
	rows := make([]indexRow, 0, len(groups))
	meta := newMetadata(level, id)
	for _, group := range groups {
		row, err := writeSeries(files, group.seriesID, group.columns, opts)
		if err != nil {
			return nil, metadata{}, err
		}
		rows = append(rows, row)
		updateMeta(&meta.Part, row, group.columns)
	}
	return rows, meta, nil
}

func writeSeries(
	files *partFiles,
	seriesID uint64,
	columns []model.ColumnData,
	opts WriteOptions,
) (indexRow, error) {
	sort.Slice(columns, func(i, j int) bool {
		return columns[i].FieldID < columns[j].FieldID
	})
	timestamps := collectTimestamps(columns)
	timePayload := marshalTimeBlock(nil, timestamps)
	timeRef, err := files.timeBlocks.write(timePayload)
	if err != nil {
		return indexRow{}, err
	}
	row := indexRow{
		SeriesID: seriesID,
		MinTime:  timestamps[0],
		MaxTime:  timestamps[len(timestamps)-1],
		TimeRef:  timeRef,
		Columns:  make([]columnRef, 0, len(columns)),
	}
	for _, column := range columns {
		ref, err := writeValueBlock(files.valueBlocks, column, timestamps, opts)
		if err != nil {
			return indexRow{}, err
		}
		row.Columns = append(row.Columns, ref)
	}
	return row, nil
}

func writeValueBlock(
	writer *blockWriter,
	column model.ColumnData,
	rowTimestamps []int64,
	opts WriteOptions,
) (columnRef, error) {
	index, err := writeValuePages(writer, column, rowTimestamps, opts)
	if err != nil {
		return columnRef{}, err
	}
	payload, err := marshalValuePageIndex(nil, index)
	if err != nil {
		return columnRef{}, fmt.Errorf("encode value page index: %w", err)
	}
	ref, err := writer.write(payload)
	if err != nil {
		return columnRef{}, err
	}
	return columnRef{
		FieldID:   column.FieldID,
		FieldType: column.FieldType,
		ValueRef:  ref,
	}, nil
}

func writeValuePages(
	writer *blockWriter,
	column model.ColumnData,
	rowTimestamps []int64,
	opts WriteOptions,
) (valuePageIndex, error) {
	index := valuePageIndex{
		FieldID:   column.FieldID,
		FieldType: column.FieldType,
		Count:     len(column.Samples),
		Pages:     make([]valuePageRef, 0, valuePageCount(len(column.Samples))),
	}
	if len(column.Samples) == 0 {
		return index, nil
	}
	for start := 0; start < len(column.Samples); start += valueBlockPageSamples {
		end := start + valueBlockPageSamples
		if end > len(column.Samples) {
			end = len(column.Samples)
		}
		pageColumn := column
		pageColumn.Samples = column.Samples[start:end]
		payload, err := marshalValuePage(nil, pageColumn, rowTimestamps, opts.Compression)
		if err != nil {
			return valuePageIndex{}, fmt.Errorf("encode value page: %w", err)
		}
		ref, err := writer.write(payload)
		if err != nil {
			return valuePageIndex{}, err
		}
		index.Pages = append(index.Pages, valuePageRef{
			MinTime: pageColumn.Samples[0].Timestamp,
			MaxTime: pageColumn.Samples[len(pageColumn.Samples)-1].Timestamp,
			Ref:     ref,
		})
	}
	return index, nil
}

func valuePageCount(samples int) int {
	if samples == 0 {
		return 0
	}
	return (samples + valueBlockPageSamples - 1) / valueBlockPageSamples
}

func writePartIndexes(path string, meta *metadata, rows []indexRow) error {
	indexPayload, err := encodeIndexRows(rows)
	if err != nil {
		return err
	}
	indexRef, err := writeBinaryBlock(filepath.Join(path, indexFile), indexPayload)
	if err != nil {
		return err
	}
	meta.IndexRef = indexRef
	metaIndex := []metaIndexRow{metaIndexFromRows(meta.Part, indexRef, rows)}
	metaIndexPayload, err := encodeMetaIndexRows(metaIndex)
	if err != nil {
		return err
	}
	metaIndexRef, err := writeBinaryBlock(filepath.Join(path, metaindexFile), metaIndexPayload)
	if err != nil {
		return err
	}
	meta.MetaIndexRef = metaIndexRef
	return writeMetadata(path, *meta)
}

func writeBinaryBlock(path string, payload []byte) (blockRef, error) {
	file, err := openWritable(path)
	if err != nil {
		return blockRef{}, err
	}
	ref, writeErr := writeBlock(file, payload)
	closeErr := file.Close()
	if writeErr != nil {
		return blockRef{}, writeErr
	}
	if closeErr != nil {
		return blockRef{}, fmt.Errorf("close binary block file: %w", closeErr)
	}
	return ref, nil
}

func writeMetadata(path string, meta metadata) error {
	data, err := encodeMetadata(meta)
	if err != nil {
		return fmt.Errorf("encode part metadata: %w", err)
	}
	return storagefs.WriteFileAtomic(filepath.Join(path, metadataFile), data)
}

func ensureStringsFile(path string) error {
	file, err := openWritable(filepath.Join(path, stringsFile))
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close strings file: %w", err)
	}
	return nil
}

func newMetadata(level int, id string) metadata {
	return metadata{
		FormatVersion: partFormatVersion,
		CreatedUnix:   time.Now().Unix(),
		Part: PartMeta{
			ID:    id,
			Level: level,
		},
	}
}
