package sstable

import (
	"errors"
	"fmt"
	"path/filepath"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/storagefs"
)

type PartWriter struct {
	path   string
	files  *partFiles
	opts   WriteOptions
	rows   []indexRow
	meta   metadata
	closed bool
}

func NewPartWriter(root string, level int, id string, opts WriteOptions) (*PartWriter, error) {
	partPath := filepath.Join(root, id)
	if err := storagefs.MkdirAll(partPath); err != nil {
		return nil, err
	}
	files, err := openPartFiles(partPath)
	if err != nil {
		removeErr := storagefs.RemoveAll(partPath)
		return nil, errors.Join(err, removeErr)
	}
	return &PartWriter{
		path:  partPath,
		files: files,
		opts:  opts,
		meta:  newMetadata(level, id),
	}, nil
}

func (w *PartWriter) AddSeries(columns []model.ColumnData) error {
	if w == nil || w.closed {
		return fmt.Errorf("part writer is closed")
	}
	if len(columns) == 0 {
		return nil
	}
	seriesID := columns[0].SeriesID
	for _, column := range columns {
		if column.SeriesID != seriesID {
			return fmt.Errorf("mixed series in streaming part writer")
		}
		if len(column.Samples) == 0 {
			return fmt.Errorf("empty column in streaming part writer")
		}
	}
	row, err := writeSeries(w.files, seriesID, columns, w.opts)
	if err != nil {
		return err
	}
	w.rows = append(w.rows, row)
	updateMeta(&w.meta.Part, row, columns)
	return nil
}

func (w *PartWriter) Close() (PartMeta, error) {
	if w == nil || w.closed {
		return PartMeta{}, fmt.Errorf("part writer is closed")
	}
	w.closed = true
	if len(w.rows) == 0 {
		closeErr := w.files.close()
		w.files = nil
		removeErr := storagefs.RemoveAll(w.path)
		return PartMeta{}, errors.Join(fmt.Errorf("part writer has no rows"), closeErr, removeErr)
	}
	closeErr := w.files.close()
	w.files = nil
	if closeErr != nil {
		removeErr := storagefs.RemoveAll(w.path)
		return PartMeta{}, errors.Join(closeErr, removeErr)
	}
	if err := writePartIndexes(w.path, &w.meta, w.rows); err != nil {
		removeErr := storagefs.RemoveAll(w.path)
		return PartMeta{}, errors.Join(err, removeErr)
	}
	if err := ensureStringsFile(w.path); err != nil {
		removeErr := storagefs.RemoveAll(w.path)
		return PartMeta{}, errors.Join(err, removeErr)
	}
	w.meta.Part.Path = w.path
	return w.meta.Part, nil
}

func (w *PartWriter) Abort() error {
	if w == nil || w.closed {
		return nil
	}
	w.closed = true
	var closeErr error
	if w.files != nil {
		closeErr = w.files.close()
		w.files = nil
	}
	return errors.Join(closeErr, storagefs.RemoveAll(w.path))
}
