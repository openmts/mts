package mts_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

func TestDefaultOptionsOpenWriteAndQuery(t *testing.T) {
	ctx := context.Background()
	engine, err := mts.Open(ctx, mts.DefaultOptions(t.TempDir()))
	if err != nil {
		t.Fatalf("Open(DefaultOptions()) error = %v", err)
	}
	if err := engine.Write(ctx, []mts.Point{{
		Measurement: "cpu",
		Timestamp:   10,
		Fields: map[string]mts.FieldValue{
			"usage": mts.Float64Value(0.7),
		},
	}}, mts.WriteOptions{}); err != nil {
		closeErr := engine.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	rows, err := engine.QueryRows(ctx, mts.Query{
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     20,
	})
	if err != nil {
		closeErr := engine.Close(ctx)
		t.Fatalf("QueryRows() error = %v close = %v", err, closeErr)
	}
	if len(rows) != 1 || rows[0].Fields["usage"].Float64 != 0.7 {
		closeErr := engine.Close(ctx)
		t.Fatalf("rows = %#v, want one usage row close = %v", rows, closeErr)
	}
	if err := engine.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestOptionsValidateRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		opts mts.Options
	}{
		{
			name: "empty path",
			opts: mts.Options{},
		},
		{
			name: "negative shard duration",
			opts: mts.Options{Path: "data", ShardDuration: -time.Second},
		},
		{
			name: "negative retention",
			opts: mts.Options{Path: "data", Retention: -time.Second},
		},
		{
			name: "negative memtable samples",
			opts: mts.Options{Path: "data", MemTableMaxSamples: -1},
		},
		{
			name: "negative memory limit",
			opts: mts.Options{
				Path:          "data",
				StorageMemory: mts.StorageMemoryOptions{HardBytesLimit: -1},
			},
		},
		{
			name: "soft memory greater than hard memory",
			opts: mts.Options{
				Path: "data",
				StorageMemory: mts.StorageMemoryOptions{
					SoftBytesLimit: 20,
					HardBytesLimit: 10,
				},
			},
		},
		{
			name: "soft samples greater than hard samples",
			opts: mts.Options{
				Path: "data",
				StorageMemory: mts.StorageMemoryOptions{
					SoftSampleLimit: 20,
					HardSampleLimit: 10,
				},
			},
		},
		{
			name: "negative wal segment bytes",
			opts: mts.Options{
				Path: "data",
				WAL:  mts.WALOptions{SegmentBytes: -1},
			},
		},
		{
			name: "negative wal batch records",
			opts: mts.Options{
				Path: "data",
				WAL:  mts.WALOptions{BatchRecords: -1},
			},
		},
		{
			name: "negative wal batch bytes",
			opts: mts.Options{
				Path: "data",
				WAL:  mts.WALOptions{BatchBytes: -1},
			},
		},
		{
			name: "negative wal batch interval",
			opts: mts.Options{
				Path: "data",
				WAL:  mts.WALOptions{BatchInterval: -time.Second},
			},
		},
		{
			name: "negative compaction level0 part limit",
			opts: mts.Options{
				Path:       "data",
				Compaction: mts.CompactionOptions{Level0PartLimit: -1},
			},
		},
		{
			name: "negative compaction max cascade steps",
			opts: mts.Options{
				Path:       "data",
				Compaction: mts.CompactionOptions{MaxCascadeSteps: -1},
			},
		},
		{
			name: "negative read amplification part limit",
			opts: mts.Options{
				Path: "data",
				Compaction: mts.CompactionOptions{
					ReadAmplificationPartLimit: -1,
				},
			},
		},
		{
			name: "negative backlog degraded threshold",
			opts: mts.Options{
				Path: "data",
				Compaction: mts.CompactionOptions{
					BacklogDegradedThreshold: -1,
				},
			},
		},
		{
			name: "negative compaction byte fields",
			opts: mts.Options{
				Path: "data",
				Compaction: mts.CompactionOptions{
					Level0SizeLimit:       -1,
					MaxOutputPartBytes:    -1,
					DiskSpaceReserveBytes: -1,
					MinFreeBytes:          -1,
				},
			},
		},
		{
			name: "negative compaction level",
			opts: mts.Options{
				Path: "data",
				Compaction: mts.CompactionOptions{
					Levels: []mts.CompactionLevelOptions{{Level: -1}},
				},
			},
		},
		{
			name: "negative compaction level part limit",
			opts: mts.Options{
				Path: "data",
				Compaction: mts.CompactionOptions{
					Levels: []mts.CompactionLevelOptions{{PartLimit: -1}},
				},
			},
		},
		{
			name: "negative compaction level size limit",
			opts: mts.Options{
				Path: "data",
				Compaction: mts.CompactionOptions{
					Levels: []mts.CompactionLevelOptions{{SizeLimit: -1}},
				},
			},
		},
		{
			name: "negative compaction level output bytes",
			opts: mts.Options{
				Path: "data",
				Compaction: mts.CompactionOptions{
					Levels: []mts.CompactionLevelOptions{{MaxOutputPartBytes: -1}},
				},
			},
		},
		{
			name: "negative compression min page values",
			opts: mts.Options{
				Path: "data",
				Compaction: mts.CompactionOptions{
					Levels: []mts.CompactionLevelOptions{{
						Compression: mts.CompressionOptions{MinPageValues: -1},
					}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.opts.Validate(); !errors.Is(err, mts.ErrInvalidOptions) {
				t.Fatalf("Validate() error = %v, want ErrInvalidOptions", err)
			}
		})
	}
}

func TestOptionsValidateAcceptsDefaultOptions(t *testing.T) {
	if err := mts.DefaultOptions("data").Validate(); err != nil {
		t.Fatalf("DefaultOptions().Validate() error = %v", err)
	}
}

func TestNilLoggerDefaultsToNop(t *testing.T) {
	ctx := context.Background()
	opts := mts.DefaultOptions(t.TempDir())
	// 不设置 Logger，应使用 nopLogger
	engine, err := mts.Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = engine.Close(ctx) }()
	// 引擎正常工作即可验证 nil logger 不 panic
	if err := engine.Write(ctx, []mts.Point{{
		Measurement: "cpu",
		Timestamp:   1,
		Fields:      map[string]mts.FieldValue{"v": mts.Float64Value(1)},
	}}, mts.WriteOptions{}); err != nil {
		t.Fatalf("Write() with nil logger error = %v", err)
	}
}

func TestCustomLoggerPropagates(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	opts := mts.DefaultOptions(t.TempDir())
	opts.Logger = logger
	engine, err := mts.Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = engine.Close(ctx) }()
	output := buf.String()
	if !strings.Contains(output, "engine opened") {
		t.Fatalf("expected 'engine opened' log, got %q", output)
	}
}
