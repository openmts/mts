package sstable

import (
	"encoding/binary"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestWindowedIntDecodersCoverDeltaAndRLE(t *testing.T) {
	intSamples := intWindowSamples(10, 12, 15, 19)
	timestamps := []int64{100, 200, 300, 400}
	writeSeqs := []uint64{1, 2, 3, 4}
	query := Query{Start: 200, End: 300}

	delta := appendDeltaIntValues(nil, intSamples)
	got, err := readWindowedIntValues(
		newBlockReader(delta),
		compressionDelta,
		len(intSamples),
		1,
		timestamps[1:3],
		writeSeqs[1:3],
		query,
	)
	if err != nil {
		t.Fatalf("readWindowedIntValues(delta) error = %v", err)
	}
	assertWindowIntValues(t, got, []int64{12, 15})

	fromStart, err := readWindowedDeltaInt(
		newBlockReader(delta),
		len(intSamples),
		0,
		timestamps[:2],
		writeSeqs[:2],
		Query{Start: 0, End: 150},
	)
	if err != nil {
		t.Fatalf("readWindowedDeltaInt(start=0) error = %v", err)
	}
	assertWindowIntValues(t, fromStart, []int64{10})

	rleSamples := intWindowSamples(20, 22, 24, 26)
	rle := appendDeltaRLEIntValues(nil, rleSamples)
	got, err = readWindowedIntValues(
		newBlockReader(rle),
		compressionRLE,
		len(rleSamples),
		1,
		timestamps[1:4],
		writeSeqs[1:4],
		Query{Start: 200, End: 400},
	)
	if err != nil {
		t.Fatalf("readWindowedIntValues(rle) error = %v", err)
	}
	assertWindowIntValues(t, got, []int64{22, 24, 26})

	rleFromStart, err := readWindowedDeltaRLEInt(
		newBlockReader(rle),
		len(rleSamples),
		0,
		timestamps[:1],
		writeSeqs[:1],
		Query{Start: 0, End: 150},
	)
	if err != nil {
		t.Fatalf("readWindowedDeltaRLEInt(start=0) error = %v", err)
	}
	assertWindowIntValues(t, rleFromStart, []int64{20})

	if empty, err := readWindowedDeltaInt(newBlockReader(nil), 0, 0, nil, nil, query); err != nil || empty != nil {
		t.Fatalf("readWindowedDeltaInt(empty) = %#v, %v; want nil", empty, err)
	}
	if empty, err := readWindowedDeltaRLEInt(newBlockReader(nil), 0, 0, nil, nil, query); err != nil || empty != nil {
		t.Fatalf("readWindowedDeltaRLEInt(empty) = %#v, %v; want nil", empty, err)
	}
}

func TestWindowedFloatDecodersCoverConstStepAndIntegerCodecs(t *testing.T) {
	floatSamples := []model.VersionedSample{
		{Value: model.Float64Value(1.5)},
		{Value: model.Float64Value(2.0)},
		{Value: model.Float64Value(2.5)},
		{Value: model.Float64Value(3.0)},
	}
	payload, ok := encodeFloatConstStepPayload(floatSamples)
	if !ok {
		t.Fatal("encodeFloatConstStepPayload() ok = false")
	}
	got, err := readWindowedFloatValues(
		newBlockReader(payload),
		compressionConstStep,
		len(floatSamples),
		1,
		[]int64{20, 30},
		[]uint64{2, 3},
		Query{Start: 20, End: 30},
	)
	if err != nil {
		t.Fatalf("readWindowedFloatValues(const-step) error = %v", err)
	}
	if len(got) != 2 || got[0].Value.Float64 != 2 || got[1].Value.Float64 != 2.5 {
		t.Fatalf("const-step values = %#v, want [2 2.5]", got)
	}
	filtered, err := readWindowedFloatConstStep(
		newBlockReader(payload),
		len(floatSamples),
		1,
		[]int64{20, 30},
		[]uint64{2, 3},
		Query{Start: 25, End: 35},
	)
	if err != nil || len(filtered) != 1 || filtered[0].Value.Float64 != 2.5 {
		t.Fatalf("readWindowedFloatConstStep(filtered) = %#v, %v", filtered, err)
	}
	if empty, err := readWindowedFloatConstStep(newBlockReader(nil), 0, 0, nil, nil, Query{}); err != nil || empty != nil {
		t.Fatalf("readWindowedFloatConstStep(empty) = %#v, %v; want nil", empty, err)
	}

	integerPayload := appendDeltaIntValues(nil, intWindowSamples(3, 5, 8))
	converted, err := readWindowedFloatValues(
		newBlockReader(integerPayload),
		compressionDelta,
		3,
		1,
		[]int64{20, 30},
		[]uint64{2, 3},
		Query{Start: 20, End: 30},
	)
	if err != nil {
		t.Fatalf("readWindowedFloatValues(delta) error = %v", err)
	}
	if len(converted) != 2 || converted[0].Value.Float64 != 5 || converted[1].Value.Float64 != 8 {
		t.Fatalf("integer-backed floats = %#v, want [5 8]", converted)
	}
}

func TestWindowedDecodersRejectMalformedPayloads(t *testing.T) {
	query := Query{Start: 0, End: 10}
	if _, err := readWindowedIntValues(newBlockReader(nil), 99, 1, 0, []int64{1}, []uint64{1}, query); err == nil {
		t.Fatal("readWindowedIntValues(unsupported) error = nil")
	}
	if _, err := readWindowedFloatValues(newBlockReader(nil), 99, 1, 0, []int64{1}, []uint64{1}, query); err == nil {
		t.Fatal("readWindowedFloatValues(unsupported) error = nil")
	}
	if _, err := readWindowedFloatIntCodec(newBlockReader(nil), compressionDelta, 1, 0, []int64{1}, []uint64{1}, query); err == nil {
		t.Fatal("readWindowedFloatIntCodec(truncated) error = nil")
	}
	if _, err := readWindowedFloatConstStep(newBlockReader(nil), 1, 0, []int64{1}, []uint64{1}, query); err == nil {
		t.Fatal("readWindowedFloatConstStep(truncated) error = nil")
	}
	if _, err := readWindowedDeltaInt(newBlockReader(nil), 1, 0, []int64{1}, []uint64{1}, query); err == nil {
		t.Fatal("readWindowedDeltaInt(first value truncated) error = nil")
	}
	firstOnly := binary.AppendVarint(nil, 10)
	if _, err := readWindowedDeltaInt(newBlockReader(firstOnly), 2, 0, []int64{1, 2}, []uint64{1, 2}, query); err == nil {
		t.Fatal("readWindowedDeltaInt(delta truncated) error = nil")
	}
	if _, err := readWindowedDeltaRLEInt(newBlockReader(nil), 1, 0, []int64{1}, []uint64{1}, query); err == nil {
		t.Fatal("readWindowedDeltaRLEInt(first value truncated) error = nil")
	}
	zeroRun := binary.AppendVarint(nil, 10)
	zeroRun = binary.AppendUvarint(zeroRun, 0)
	if _, err := readWindowedDeltaRLEInt(newBlockReader(zeroRun), 2, 0, []int64{1, 2}, []uint64{1, 2}, query); err == nil {
		t.Fatal("readWindowedDeltaRLEInt(zero run) error = nil")
	}
	missingDelta := binary.AppendVarint(nil, 10)
	missingDelta = binary.AppendUvarint(missingDelta, 1)
	if _, err := readWindowedDeltaRLEInt(newBlockReader(missingDelta), 2, 0, []int64{1, 2}, []uint64{1, 2}, query); err == nil {
		t.Fatal("readWindowedDeltaRLEInt(missing delta) error = nil")
	}
	overflow := binary.AppendVarint(nil, 10)
	overflow = binary.AppendUvarint(overflow, 3)
	overflow = binary.AppendUvarint(overflow, zigZag64(1))
	if _, err := readWindowedDeltaRLEInt(newBlockReader(overflow), 2, 0, []int64{1, 2}, []uint64{1, 2}, query); err == nil {
		t.Fatal("readWindowedDeltaRLEInt(overflow) error = nil")
	}
}

func TestQueryWindowEdgeCases(t *testing.T) {
	if start, end := constStepWindow(5, 0, 3, Query{Start: 5, End: 5}); start != 0 || end != 3 {
		t.Fatalf("constStepWindow(zero step hit) = [%d,%d), want [0,3)", start, end)
	}
	if start, end := constStepWindow(5, 0, 3, Query{Start: 6, End: 7}); start != 0 || end != 0 {
		t.Fatalf("constStepWindow(zero step miss) = [%d,%d), want empty", start, end)
	}
	if start, end := constStepWindow(5, -1, 3, Query{Start: 0, End: 10}); start != 0 || end != 3 {
		t.Fatalf("constStepWindow(negative step) = [%d,%d), want full", start, end)
	}
	if start, end := constStepWindow(0, 10, 3, Query{Start: 40, End: 50}); start != 0 || end != 0 {
		t.Fatalf("constStepWindow(after series) = [%d,%d), want empty", start, end)
	}
	if got := materializeConstStepTimestamps(10, 2, 3, 3); got != nil {
		t.Fatalf("materializeConstStepTimestamps(empty) = %v, want nil", got)
	}
	if got := materializeConstStepTimestamps(10, 2, 1, 4); len(got) != 3 || got[0] != 12 || got[2] != 16 {
		t.Fatalf("materializeConstStepTimestamps() = %v, want [12 14 16]", got)
	}
	if lo, hi := sortedTimestampWindow(nil, Query{}); lo != 0 || hi != 0 {
		t.Fatalf("sortedTimestampWindow(nil) = [%d,%d)", lo, hi)
	}
	if lo, hi := sortedTimestampWindow([]int64{1, 3, 5, 7}, Query{Start: 2, End: 6}); lo != 1 || hi != 3 {
		t.Fatalf("sortedTimestampWindow() = [%d,%d), want [1,3)", lo, hi)
	}
}

func intWindowSamples(values ...int64) []model.VersionedSample {
	samples := make([]model.VersionedSample, len(values))
	for index, value := range values {
		samples[index] = model.VersionedSample{Value: model.Int64Value(value)}
	}
	return samples
}

func assertWindowIntValues(t *testing.T, samples []model.VersionedSample, want []int64) {
	t.Helper()
	if len(samples) != len(want) {
		t.Fatalf("sample count = %d, want %d: %#v", len(samples), len(want), samples)
	}
	for index, expected := range want {
		if samples[index].Value.Int64 != expected {
			t.Fatalf("sample[%d] = %d, want %d", index, samples[index].Value.Int64, expected)
		}
	}
}
