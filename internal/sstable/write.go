package sstable

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/storagefs"
)

func WritePart(root string, level int, id string, columns []model.ColumnData) (PartMeta, error) {
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
	rows, meta, writeErr := writeColumns(files, level, id, columns)
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
	timestamps *os.File
	values     *os.File
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
	return &partFiles{
		timestamps: timestamps,
		values:     values,
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
) ([]indexRow, metadata, error) {
	grouped := groupColumns(columns)
	rows := make([]indexRow, 0, len(grouped))
	meta := newMetadata(level, id)
	for _, seriesID := range sortedSeriesIDs(grouped) {
		row, err := writeSeries(files, seriesID, grouped[seriesID])
		if err != nil {
			return nil, metadata{}, err
		}
		rows = append(rows, row)
		updateMeta(&meta.Part, row, grouped[seriesID])
	}
	return rows, meta, nil
}

func writeSeries(
	files *partFiles,
	seriesID uint64,
	columns []model.ColumnData,
) (indexRow, error) {
	sort.Slice(columns, func(i, j int) bool {
		return columns[i].FieldID < columns[j].FieldID
	})
	timestamps := collectTimestamps(columns)
	timePayload, err := json.Marshal(timeBlockFrom(timestamps))
	if err != nil {
		return indexRow{}, fmt.Errorf("encode time block: %w", err)
	}
	timeRef, err := writeBlock(files.timestamps, timePayload)
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
		ref, err := writeValueBlock(files.values, column)
		if err != nil {
			return indexRow{}, err
		}
		row.Columns = append(row.Columns, ref)
	}
	return row, nil
}

func writeValueBlock(file *os.File, column model.ColumnData) (columnRef, error) {
	payload, err := json.Marshal(valueBlock{
		Encoding:  "plain-json-v1",
		FieldID:   column.FieldID,
		FieldType: column.FieldType,
		Samples:   cloneSamples(column.Samples),
	})
	if err != nil {
		return columnRef{}, fmt.Errorf("encode value block: %w", err)
	}
	ref, err := writeBlock(file, payload)
	if err != nil {
		return columnRef{}, err
	}
	return columnRef{
		FieldID:   column.FieldID,
		FieldType: column.FieldType,
		ValueRef:  ref,
	}, nil
}

func writePartIndexes(path string, meta *metadata, rows []indexRow) error {
	indexRef, err := writeJSONBlock(filepath.Join(path, indexFile), rows)
	if err != nil {
		return err
	}
	meta.IndexRef = indexRef
	metaIndex := []metaIndexRow{metaIndexFromRows(meta.Part, indexRef, rows)}
	metaIndexRef, err := writeJSONBlock(filepath.Join(path, metaindexFile), metaIndex)
	if err != nil {
		return err
	}
	meta.MetaIndexRef = metaIndexRef
	return writeMetadata(path, *meta)
}

func writeJSONBlock(path string, value any) (blockRef, error) {
	file, err := openWritable(path)
	if err != nil {
		return blockRef{}, err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		closeErr := file.Close()
		return blockRef{}, fmt.Errorf("encode json block: %w close: %v", err, closeErr)
	}
	ref, writeErr := writeBlock(file, payload)
	closeErr := file.Close()
	if writeErr != nil {
		return blockRef{}, writeErr
	}
	if closeErr != nil {
		return blockRef{}, fmt.Errorf("close json block file: %w", closeErr)
	}
	return ref, nil
}

func writeMetadata(path string, meta metadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
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
		FormatVersion: 1,
		CreatedUnix:   time.Now().Unix(),
		Part: PartMeta{
			ID:    id,
			Level: level,
		},
	}
}
