package wal

import (
	"encoding/binary"
	"fmt"

	"github.com/openmts/mts/internal/model"
)

func encodeTombstones(tombstones []model.Tombstone) ([]byte, error) {
	dst := make([]byte, 0, estimateTombstonesSize(tombstones))
	dst = binary.AppendUvarint(dst, uint64(len(tombstones)))
	for _, tombstone := range tombstones {
		dst = appendTombstone(dst, tombstone)
	}
	return dst, nil
}

func estimateTombstonesSize(tombstones []model.Tombstone) int {
	size := uvarintSize(uint64(len(tombstones)))
	for _, tombstone := range tombstones {
		size += varintSize(tombstone.StartTime) + varintSize(tombstone.EndTime)
		size += uvarintSize(tombstone.WriteSeq)
		size += uvarintSize(uint64(len(tombstone.SeriesIDs))) + len(tombstone.SeriesIDs)*10
		size += uvarintSize(uint64(len(tombstone.FieldIDs))) + len(tombstone.FieldIDs)*5
	}
	return size
}

func appendTombstone(dst []byte, tombstone model.Tombstone) []byte {
	dst = binary.AppendVarint(dst, tombstone.StartTime)
	dst = binary.AppendVarint(dst, tombstone.EndTime)
	dst = binary.AppendUvarint(dst, tombstone.WriteSeq)
	dst = binary.AppendUvarint(dst, uint64(len(tombstone.SeriesIDs)))
	for _, seriesID := range tombstone.SeriesIDs {
		dst = binary.AppendUvarint(dst, seriesID)
	}
	dst = binary.AppendUvarint(dst, uint64(len(tombstone.FieldIDs)))
	for _, fieldID := range tombstone.FieldIDs {
		dst = binary.AppendUvarint(dst, uint64(fieldID))
	}
	return dst
}

func decodeTombstones(payload []byte) ([]model.Tombstone, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("empty wal tombstones")
	}
	reader := newBatchReader(payload)
	count, err := reader.intCount("tombstone count")
	if err != nil {
		return nil, err
	}
	tombstones := make([]model.Tombstone, 0, count)
	for range count {
		tombstone, err := readTombstone(reader)
		if err != nil {
			return nil, err
		}
		tombstones = append(tombstones, tombstone)
	}
	if err := reader.done("wal tombstones"); err != nil {
		return nil, err
	}
	return tombstones, nil
}

func readTombstone(reader *batchReader) (model.Tombstone, error) {
	start, err := reader.varint("tombstone start")
	if err != nil {
		return model.Tombstone{}, err
	}
	end, err := reader.varint("tombstone end")
	if err != nil {
		return model.Tombstone{}, err
	}
	writeSeq, err := reader.uvarint("tombstone write seq")
	if err != nil {
		return model.Tombstone{}, err
	}
	seriesIDs, err := readUint64s(reader, "tombstone series")
	if err != nil {
		return model.Tombstone{}, err
	}
	fieldIDs, err := readUint32s(reader, "tombstone fields")
	return model.Tombstone{
		SeriesIDs: seriesIDs,
		FieldIDs:  fieldIDs,
		StartTime: start,
		EndTime:   end,
		WriteSeq:  writeSeq,
	}, err
}

func readUint64s(reader *batchReader, name string) ([]uint64, error) {
	count, err := reader.intCount(name + " count")
	if err != nil {
		return nil, err
	}
	values := make([]uint64, count)
	for index := range count {
		values[index], err = reader.uvarint(name + " value")
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

func readUint32s(reader *batchReader, name string) ([]uint32, error) {
	count, err := reader.intCount(name + " count")
	if err != nil {
		return nil, err
	}
	values := make([]uint32, count)
	for index := range count {
		value, err := reader.uvarint(name + " value")
		if err != nil {
			return nil, err
		}
		values[index], err = uint32Value(name+" value", value)
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}
