package sstable

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestMarshalTimeBlockConstStep(t *testing.T) {
	timestamps := make([]int64, 4096)
	for i := range timestamps {
		timestamps[i] = int64(i) * 1_000_000_000
	}
	payload := marshalTimeBlock(nil, timestamps)
	if len(payload) > 24 {
		t.Fatalf("const-step time block size=%d, want <=24", len(payload))
	}
	if payload[0] != timeEncodingConstStep {
		t.Fatalf("encoding=%d want const-step", payload[0])
	}
	got, err := unmarshalTimeBlock(payload)
	if err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}
	if !reflect.DeepEqual(got, timestamps) {
		t.Fatalf("timestamps mismatch first=%d last=%d", got[0], got[len(got)-1])
	}
}

func TestMarshalTimeBlockFallsBackDelta(t *testing.T) {
	timestamps := []int64{1, 3, 8, 9, 20}
	payload := marshalTimeBlock(nil, timestamps)
	if payload[0] != timeEncodingDelta {
		t.Fatalf("encoding=%d want delta", payload[0])
	}
	got, err := unmarshalTimeBlock(payload)
	if err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}
	if !reflect.DeepEqual(got, timestamps) {
		t.Fatalf("got=%v want=%v", got, timestamps)
	}
}

func TestMarshalTimeBlockEmptyAndSingle(t *testing.T) {
	if got := marshalTimeBlock(nil, nil); len(got) < 2 {
		t.Fatalf("empty payload too short: %v", got)
	}
	payload := marshalTimeBlock(nil, []int64{42})
	got, err := unmarshalTimeBlock(payload)
	if err != nil || !reflect.DeepEqual(got, []int64{42}) {
		t.Fatalf("single = %v err=%v", got, err)
	}
}

func TestUnmarshalTimeBlockErrors(t *testing.T) {
	if _, err := unmarshalTimeBlock([]byte{99, 0}); err == nil {
		t.Fatal("unknown encoding error = nil")
	}
	if _, err := unmarshalTimeBlock([]byte{timeEncodingConstStep}); err == nil {
		t.Fatal("truncated const-step error = nil")
	}
	payload := []byte{timeEncodingConstStep, 2}
	payload = append(payload, make([]byte, 8)...)
	if _, err := unmarshalTimeBlock(payload); err == nil {
		t.Fatal("missing step error = nil")
	}
}

func TestWritePartTimestampsConstStepSavesSpace(t *testing.T) {
	dir := t.TempDir()
	n := 8192
	samples := make([]model.VersionedSample, n)
	for i := range samples {
		samples[i] = model.VersionedSample{
			Timestamp: int64(i),
			WriteSeq:  0,
			Value:     model.Int64Value(int64(i)),
		}
	}
	columns := []model.ColumnData{{
		SeriesID: 1, FieldID: 1, FieldType: model.FieldInt64, Samples: samples,
	}}
	meta, err := WritePartWithOptions(dir, 1, "sst-time", columns, WriteOptions{
		Compression: model.CompressionOptions{
			Enabled: true, Algorithm: "zstd", MinPageValues: 1,
			ValuePageSamples: 4096, OmitWriteSeq: true,
		},
	})
	if err != nil {
		t.Fatalf("WritePartWithOptions error = %v", err)
	}
	info, err := os.Stat(filepath.Join(meta.Path, timestampsFile))
	if err != nil {
		t.Fatalf("stat timestamps: %v", err)
	}
	// 单 series 8192 点，const-step 帧头 + 载荷应远小于旧 delta ~40KB
	if info.Size() > 64 {
		t.Fatalf("timestamps.bin size=%d, want compact const-step", info.Size())
	}
}
