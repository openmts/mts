package sstable

import (
	"fmt"

	"github.com/openmts/mts/internal/model"
)

func readFloatIntValues(
	reader *blockReader,
	codecID byte,
	count int,
) ([]model.FieldValue, error) {
	var (
		intValues []model.FieldValue
		err       error
	)
	switch codecID {
	case compressionDelta:
		intValues, err = readDeltaIntValues(reader, codecID, count)
	case compressionRLE:
		intValues, err = readDeltaRLEIntValues(reader, codecID, count)
	default:
		return nil, fmt.Errorf("unknown float-int compression %d", codecID)
	}
	if err != nil {
		return nil, err
	}
	values := make([]model.FieldValue, count)
	for index, value := range intValues {
		values[index] = model.Float64Value(float64(value.Int64))
	}
	return values, nil
}

func readFloatIntSampleValues(
	reader *blockReader,
	codecID byte,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	var (
		intSamples []model.VersionedSample
		err        error
	)
	switch codecID {
	case compressionDelta:
		intSamples, err = readDeltaIntSampleValues(reader, codecID, timestamps, writeSeqs, query)
	case compressionRLE:
		intSamples, err = readDeltaRLEIntSampleValues(reader, codecID, timestamps, writeSeqs, query)
	default:
		return nil, fmt.Errorf("unknown float-int compression %d", codecID)
	}
	if err != nil {
		return nil, err
	}
	for index := range intSamples {
		intSamples[index].Value = model.Float64Value(float64(intSamples[index].Value.Int64))
	}
	return intSamples, nil
}
