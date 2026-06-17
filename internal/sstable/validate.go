package sstable

import (
	"fmt"
	"path/filepath"

	"codeberg.org/mts/mts/internal/storagefs"
)

func validatePartForOpen(part *Part) error {
	if err := validatePartComponents(part.path, part.metadata.Components); err != nil {
		return err
	}
	if err := validateMetadataRefs(part.path, part.metadata); err != nil {
		return err
	}
	rows, err := part.loadIndexRows()
	if err != nil {
		return fmt.Errorf("validate part index: %w", err)
	}
	if err := validateSeriesIndexRefs(part, part.seriesRows); err != nil {
		return err
	}
	return validateIndexRows(part, rows)
}

func validatePartComponents(path string, components []string) error {
	for _, name := range metadataComponents(components) {
		componentPath := filepath.Join(path, name)
		info, err := storagefs.Stat(componentPath)
		if err != nil {
			return fmt.Errorf("validate part component %s: %w", name, err)
		}
		if info.IsDir() {
			return fmt.Errorf("validate part component %s: component is directory", name)
		}
	}
	return nil
}

func validateMetadataRefs(path string, meta metadata) error {
	checks := []struct {
		name string
		ref  blockRef
	}{
		{name: indexFile, ref: meta.IndexRef},
		{name: metaindexFile, ref: meta.MetaIndexRef},
		{name: seriesIndexFile, ref: meta.SeriesIndexRef},
	}
	for _, check := range checks {
		if err := validateBlockRefWithinFile(filepath.Join(path, check.name), check.ref); err != nil {
			return fmt.Errorf("validate metadata ref %s offset=%d size=%d: %w",
				check.name, check.ref.Offset, check.ref.Size, err)
		}
	}
	return nil
}

func validateSeriesIndexRefs(part *Part, rows []seriesIndexRow) error {
	for _, row := range rows {
		if err := validateBlockRefWithinFile(filepath.Join(part.path, indexFile), row.IndexRef); err != nil {
			return fmt.Errorf("validate series index ref part=%s series=%d offset=%d size=%d: %w",
				part.metadata.Part.ID, row.SeriesID, row.IndexRef.Offset, row.IndexRef.Size, err)
		}
		if _, err := part.readIndexRowBlock(row.IndexRef); err != nil {
			return fmt.Errorf("validate series index block part=%s series=%d offset=%d: %w",
				part.metadata.Part.ID, row.SeriesID, row.IndexRef.Offset, err)
		}
	}
	return nil
}

func validateIndexRows(part *Part, rows []indexRow) error {
	for _, row := range rows {
		if err := validateTimeRef(part, row); err != nil {
			return err
		}
		if err := validateColumnRefs(part, row); err != nil {
			return err
		}
	}
	return nil
}

func validateTimeRef(part *Part, row indexRow) error {
	if err := validateBlockRefWithinFile(filepath.Join(part.path, timestampsFile), row.TimeRef); err != nil {
		return fmt.Errorf("validate time ref part=%s series=%d offset=%d size=%d: %w",
			part.metadata.Part.ID, row.SeriesID, row.TimeRef.Offset, row.TimeRef.Size, err)
	}
	payload, err := part.readBlockPayload(timestampsFile, row.TimeRef)
	if err != nil {
		return fmt.Errorf("validate time block part=%s series=%d offset=%d: %w",
			part.metadata.Part.ID, row.SeriesID, row.TimeRef.Offset, err)
	}
	_, decodeErr := unmarshalTimeBlock(payload.Bytes())
	payload.Release()
	if decodeErr != nil {
		return fmt.Errorf("validate time block part=%s series=%d offset=%d: %w",
			part.metadata.Part.ID, row.SeriesID, row.TimeRef.Offset, decodeErr)
	}
	return nil
}

func validateColumnRefs(part *Part, row indexRow) error {
	for _, column := range row.Columns {
		if err := validateBlockRefWithinFile(filepath.Join(part.path, valuesFile), column.ValueRef); err != nil {
			return fmt.Errorf("validate value index ref part=%s series=%d field=%d offset=%d size=%d: %w",
				part.metadata.Part.ID, row.SeriesID, column.FieldID, column.ValueRef.Offset, column.ValueRef.Size, err)
		}
		if err := validateValueIndex(part, row.SeriesID, column); err != nil {
			return err
		}
	}
	return nil
}

func validateValueIndex(part *Part, seriesID uint64, column columnRef) error {
	payload, err := part.readBlockPayload(valuesFile, column.ValueRef)
	if err != nil {
		return fmt.Errorf("validate value index part=%s series=%d field=%d offset=%d: %w",
			part.metadata.Part.ID, seriesID, column.FieldID, column.ValueRef.Offset, err)
	}
	index, decodeErr := unmarshalValuePageIndex(payload.Bytes())
	payload.Release()
	if decodeErr != nil {
		return fmt.Errorf("validate value index part=%s series=%d field=%d offset=%d: %w",
			part.metadata.Part.ID, seriesID, column.FieldID, column.ValueRef.Offset, decodeErr)
	}
	return validateValuePages(part, seriesID, column.FieldID, index.Pages)
}

func validateValuePages(part *Part, seriesID uint64, fieldID uint32, pages []valuePageRef) error {
	for _, page := range pages {
		if err := validateBlockRefWithinFile(filepath.Join(part.path, valuesFile), page.Ref); err != nil {
			return fmt.Errorf("validate value page ref part=%s series=%d field=%d offset=%d size=%d: %w",
				part.metadata.Part.ID, seriesID, fieldID, page.Ref.Offset, page.Ref.Size, err)
		}
		payload, err := part.readBlockPayload(valuesFile, page.Ref)
		if err != nil {
			return fmt.Errorf("validate value page part=%s series=%d field=%d offset=%d: %w",
				part.metadata.Part.ID, seriesID, fieldID, page.Ref.Offset, err)
		}
		payload.Release()
	}
	return nil
}

func validateBlockRefWithinFile(path string, ref blockRef) error {
	if ref.Offset < 0 || ref.Size < 0 {
		return fmt.Errorf("block ref contains negative values")
	}
	info, err := storagefs.Stat(path)
	if err != nil {
		return err
	}
	if ref.Size == 0 {
		return fmt.Errorf("block ref size is zero")
	}
	if ref.Offset > info.Size() || ref.Size > info.Size()-ref.Offset {
		return fmt.Errorf("block ref exceeds file size %d", info.Size())
	}
	return nil
}
