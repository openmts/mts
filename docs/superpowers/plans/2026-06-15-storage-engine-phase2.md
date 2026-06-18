# Storage Engine Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden `mts` storage engine before upper-layer work by replacing persistent JSON formats with compact binary formats, improving crash consistency, query pruning, compaction, and performance measurement.

**Architecture:** Add shared binary codec primitives first, then migrate Catalog, WAL, Manifest, Part metadata/index/metaindex, timestamp/value blocks onto versioned binary formats. Preserve the public embedded API while improving storage internals, keeping correctness first and using benchmark/pprof evidence for performance claims.

**Tech Stack:** Go 1.26.2, standard library only for production code, `encoding/binary`, CRC32C, `go test`, `go test -bench`, `pprof`, `goimports-reviser`, `golangci-lint`.

---

## Constraints

- 任务预计总耗时：8-14 小时；硬超时：24 小时；超过 5 分钟必须反馈当前进展。
- 生产持久化文件禁止使用 JSON、Gob、CSV、YAML 等通用文本/反射格式。
- 允许测试、文档、benchmark 报告使用文本格式。
- 新增目录权限必须为 `0700`，新增文件权限必须为 `0600`。
- 每个任务完成后更新本计划勾选状态和实现备注。
- 实现阶段必须在用户明确说“进入实现阶段”或“implement tasks”后开始。

## Research Notes

- Prometheus TSDB 使用 magic/version/CRC 的二进制 chunk/index 文件；chunk 使用 uvarint length、encoding byte、data、CRC32C；WAL 使用 segment 和二进制 record，sample 记录使用 series/timestamp delta。
- Prometheus index 使用 symbol table、series、postings、TOC 分区，并通过时间范围和引用跳过无关 chunk。
- VictoriaMetrics Part 使用 metadata、metaindex、timestamps、values、index 文件分离，并对时间戳和值做专门数组编码与索引块缓存。
- Phase 2 采用这些思想的保守子集：magic/version/envelope、varint/delta、typed value blocks、metaindex/index 剪枝、CRC 校验，不引入外部压缩库。

## File Structure

- Create: `internal/codec/envelope.go` - file/block envelope with magic, version, flags, payload length, CRC32C.
- Create: `internal/codec/binary.go` - varint, fixed-width, string, bool bitset, field value encode/decode helpers.
- Create: `internal/codec/codec_test.go` - codec roundtrip, corrupt payload, unsupported version tests.
- Modify: `internal/catalog/persist.go` - binary snapshot and catalog WAL.
- Modify: `internal/catalog/types.go` - binary record constants and persistence structs if needed.
- Modify: `internal/catalog/catalog_test.go` - ensure catalog persistence files are binary and reopen works.
- Modify: `internal/wal/wal.go` - binary WAL payload for write batches.
- Modify: `internal/wal/wal_test.go` - replay, corruption, file-size sanity, no JSON markers.
- Create: `internal/sstable/encoding.go` - typed SSTable v2 blocks.
- Create: `internal/sstable/encoding_test.go` - timestamp/value block roundtrip for all field types.
- Modify: `internal/sstable/types.go` - v2 metadata/index/metaindex structs and encoding constants.
- Modify: `internal/sstable/write.go` - write binary metadata/index/metaindex/timestamp/value/string files.
- Modify: `internal/sstable/read.go` - read binary metadata/index/metaindex/value blocks and prune before value reads.
- Modify: `internal/sstable/manifest.go` - binary manifest encoding.
- Modify: `internal/sstable/sstable_test.go` - v2 roundtrip, pruning, corrupt file behavior.
- Modify: `internal/engine/shard.go` - shard lifecycle lock, flush ordering, WAL truncation order.
- Modify: `internal/engine/lifecycle.go` - size-tiered compaction strategy and retention locking.
- Modify: `internal/model/types.go` - add `CompactionOptions` to `Options`.
- Create: `internal/bench/storage_bench_test.go` - storage benchmark baselines.
- Create e2e cases under `tests/e2e/`: `wal_recovery`, `flush_manifest_recovery`, `compaction_integrity`, `retention`, `query_pruning`.
- Modify: `tests/pprof/storage_engine/main.go` - add workload modes for write/query/compact/replay.

## Task 1: Baseline Benchmarks And Binary-Only Guardrails

**Files:**
- Create: `internal/bench/storage_bench_test.go`
- Create: `tests/e2e/no_json_storage/main.go`
- Modify: `docs/superpowers/plans/2026-06-15-storage-engine-phase2.md`

- [x] **Step 1: Add benchmark package skeleton**

Create `internal/bench/storage_bench_test.go`:

```go
package bench

import (
	"context"
	"fmt"
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

func BenchmarkEngineWriteBatch(b *testing.B) {
	ctx := context.Background()
	for _, points := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("points=%d", points), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				dir := b.TempDir()
				eng, err := mts.Open(ctx, mts.Options{Path: dir, ShardDuration: time.Hour, MemTableMaxSamples: points + 1})
				if err != nil {
					b.Fatalf("Open() error = %v", err)
				}
				if err := eng.Write(ctx, makeBenchPoints(points, 100), mts.WriteOptions{Sync: true}); err != nil {
					closeErr := eng.Close(ctx)
					b.Fatalf("Write() error = %v close = %v", err, closeErr)
				}
				if err := eng.Close(ctx); err != nil {
					b.Fatalf("Close() error = %v", err)
				}
			}
		})
	}
}

func makeBenchPoints(count int, series int) []mts.Point {
	points := make([]mts.Point, 0, count)
	for index := range count {
		host := fmt.Sprintf("host-%04d", index%series)
		points = append(points, mts.Point{
			Measurement: "bench",
			Tags:        map[string]string{"host": host},
			Timestamp:   int64(index),
			Fields: map[string]mts.FieldValue{
				"value":  mts.Float64Value(float64(index)),
				"count":  mts.Int64Value(int64(index)),
				"active": mts.BoolValue(index%2 == 0),
				"state":  mts.StringValue("ok"),
			},
		})
	}
	return points
}
```

- [x] **Step 2: Run benchmark once to capture baseline**

Run:

```bash
go test ./internal/bench -bench=. -benchmem -count=1 -timeout 600s
```

Expected: PASS with benchmark rows for `BenchmarkEngineWriteBatch`.

- [x] **Step 3: Add binary-only e2e guard**

Create `tests/e2e/no_json_storage/main.go` that writes data, flushes, then scans storage files and fails if it finds JSON object markers in production storage files:

```go
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	mts "github.com/openmts/mts"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("no_json_storage failed: %v", err)
	}
	log.Print("no_json_storage passed")
}

func run() (err error) {
	dir, err := os.MkdirTemp("", "mts-e2e-no-json-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("chmod temp dir: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(dir))
	}()
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{Path: dir, ShardDuration: time.Hour, MemTableMaxSamples: 1})
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	point := mts.Point{
		Measurement: "bin",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   1,
		Fields:      map[string]mts.FieldValue{"value": mts.Float64Value(1), "state": mts.StringValue("ok")},
	}
	if err := eng.Write(ctx, []mts.Point{point}, mts.WriteOptions{Sync: true}); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("write point: %w", err), closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("flush: %w", err), closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		return fmt.Errorf("close engine: %w", err)
	}
	return assertNoJSONStorage(dir)
}

func assertNoJSONStorage(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if bytes.Contains(data, []byte(`{"`)) || bytes.Contains(data, []byte(`":`)) {
			return fmt.Errorf("file %s appears to contain JSON payload", path)
		}
		return nil
	})
}
```

- [x] **Step 4: Run the guard and observe failure before binary migration**

Run:

```bash
cd tests/e2e/no_json_storage && go build -o no_json_storage . && ./no_json_storage; status=$?; rm -f no_json_storage; exit $status
```

Expected: FAIL on current implementation because catalog, manifest, metadata, index or values contain JSON.

实现备注：已新增 `internal/bench/storage_bench_test.go` 和 `tests/e2e/no_json_storage`。基线命令 `go test ./internal/bench -bench=. -benchmem -count=1 -timeout 600s` 通过，结果为 1K 写入 `22243271 ns/op, 12209360 B/op, 77621 allocs/op`，10K 写入 `76501112 ns/op, 61665505 B/op, 264745 allocs/op`。no-json guard 已按预期失败，失败文件为 `catalog/catalog.wal`，证明当前生产落盘仍包含 JSON。

## Task 2: Shared Binary Codec

**Files:**
- Create: `internal/codec/envelope.go`
- Create: `internal/codec/binary.go`
- Create: `internal/codec/codec_test.go`

- [x] **Step 1: Add codec tests**

Create `internal/codec/codec_test.go`:

```go
package codec

import (
	"bytes"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestEnvelopeRoundTripAndCorruption(t *testing.T) {
	payload := []byte{1, 2, 3, 4}
	frame := MarshalEnvelope(nil, Magic("MTSTST2"), 2, 0, payload)
	got, err := UnmarshalEnvelope(frame, Magic("MTSTST2"), 2)
	if err != nil {
		t.Fatalf("UnmarshalEnvelope() error = %v", err)
	}
	if !bytes.Equal(got.Payload, payload) {
		t.Fatalf("payload = %v, want %v", got.Payload, payload)
	}
	frame[len(frame)-1] ^= 0xff
	if _, err := UnmarshalEnvelope(frame, Magic("MTSTST2"), 2); err == nil {
		t.Fatal("UnmarshalEnvelope(corrupt) error = nil, want error")
	}
}

func TestFieldValueRoundTrip(t *testing.T) {
	values := []model.FieldValue{
		model.Float64Value(1.5),
		model.Int64Value(-2),
		model.StringValue("ok"),
		model.BoolValue(true),
	}
	buf := make([]byte, 0)
	for _, value := range values {
		buf = AppendFieldValue(buf, value)
	}
	rest := buf
	for _, want := range values {
		got, next, err := ReadFieldValue(rest)
		if err != nil {
			t.Fatalf("ReadFieldValue() error = %v", err)
		}
		if got != want {
			t.Fatalf("value = %#v, want %#v", got, want)
		}
		rest = next
	}
	if len(rest) != 0 {
		t.Fatalf("remaining bytes = %d, want 0", len(rest))
	}
}
```

- [x] **Step 2: Run codec tests and verify failure**

Run:

```bash
go test ./internal/codec -timeout 180s
```

Expected: FAIL because package does not exist.

- [x] **Step 3: Implement envelope and primitive codec**

Implement:

```go
type Magic string

type Envelope struct {
	Magic   Magic
	Version uint16
	Flags   uint16
	Payload []byte
}

func MarshalEnvelope(dst []byte, magic Magic, version uint16, flags uint16, payload []byte) []byte
func UnmarshalEnvelope(data []byte, want Magic, maxVersion uint16) (Envelope, error)
func AppendString(dst []byte, value string) []byte
func ReadString(src []byte) (string, []byte, error)
func AppendFieldValue(dst []byte, value model.FieldValue) []byte
func ReadFieldValue(src []byte) (model.FieldValue, []byte, error)
func AppendBoolBits(dst []byte, values []bool) []byte
func ReadBoolBits(src []byte, count int) ([]bool, []byte, error)
```

Encoding rules:

- Envelope layout: `magic[7] version u16 flags u16 payloadLen uvarint payload crc32c u32`.
- Integers use `binary.PutVarint` / `binary.PutUvarint` for variable values and little-endian for fixed arrays.
- String uses `uvarint length + raw bytes`.
- FieldValue uses `type byte + typed payload`.
- Errors must include context and never panic.

- [x] **Step 4: Run codec tests**

Run:

```bash
go test ./internal/codec -timeout 180s
```

Expected: PASS.

实现备注：已新增 `internal/codec`。Envelope 布局为 `magic[7] + version u16le + flags u16le + payloadLen uvarint + payload + crc32c u32le`，CRC 使用 Castagnoli 并覆盖 CRC 前所有字节。已实现 string、FieldValue、bool bitset 编解码；`go test ./internal/codec -timeout 180s` 通过。

## Task 3: Binary Catalog Persistence

**Files:**
- Modify: `internal/catalog/types.go`
- Modify: `internal/catalog/persist.go`
- Modify: `internal/catalog/catalog_test.go`
- Modify: `internal/catalog/internal_test.go`

- [x] **Step 1: Add binary catalog persistence tests**

Add tests asserting snapshot and WAL contain no JSON markers and reopen works:

```go
func TestCatalogPersistenceIsBinary(t *testing.T) {
	dir := t.TempDir()
	cat, err := catalog.Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	point := model.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Fields:      map[string]model.FieldValue{"usage": model.Float64Value(1)},
	}
	if _, err := cat.ResolvePoint(point); err != nil {
		t.Fatalf("ResolvePoint() error = %v", err)
	}
	if err := cat.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	for _, name := range []string{"catalog.wal", "snapshot.bin"} {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", name, err)
		}
		if bytes.Contains(data, []byte(`{"`)) {
			t.Fatalf("%s contains JSON marker", name)
		}
	}
	reopened, err := catalog.Open(dir)
	if err != nil {
		t.Fatalf("Open() after binary persistence error = %v", err)
	}
	if _, err := reopened.ResolvePoint(point); err != nil {
		t.Fatalf("ResolvePoint() after reopen error = %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("Close() reopened error = %v", err)
	}
}
```

- [x] **Step 2: Run catalog tests and verify failure**

Run:

```bash
go test ./internal/catalog -run TestCatalogPersistenceIsBinary -timeout 180s
```

Expected: FAIL because current persistence uses JSON files.

- [x] **Step 3: Implement binary catalog snapshot and WAL**

Implementation requirements:

- Replace `snapshot.json` with `snapshot.bin`.
- Keep reading `snapshot.json` unsupported with a clear error if present and no binary file exists.
- Catalog WAL record payload layout:
  - record type byte: series or field.
  - series: `seriesID uvarint + measurement string + tagCount uvarint + sorted tag key/value strings`.
  - field: `fieldID uvarint + measurement string + field name string + field type byte`.
- Catalog snapshot payload layout:
  - `nextSeriesID uvarint + nextFieldID uvarint + seriesCount + fieldsCount + records`.
- File envelope magic: `MTSCAT2`, version `2`.
- WAL line format must be binary length-prefixed records, not JSONL.

- [x] **Step 4: Run catalog package**

Run:

```bash
go test ./internal/catalog -timeout 180s
```

Expected: PASS.

实现备注：Catalog snapshot 已从 `snapshot.json` 切换为 `snapshot.bin`，文件 envelope magic 为 `MTSCAT2`、version `2`；仅存在旧 `snapshot.json` 时 `Open` 返回明确的 legacy JSON unsupported 错误。`catalog.wal` 文件名保留，内容改为 `uvarint frame length + MTSCAT2 envelope` 的二进制记录。`go test ./internal/catalog -timeout 180s` 通过。

## Task 4: WAL Binary Batch Payload

**Files:**
- Modify: `internal/wal/wal.go`
- Modify: `internal/wal/wal_test.go`
- Modify: `internal/wal/internal_test.go`

- [x] **Step 1: Add WAL binary payload tests**

Add a test that appends representative points and asserts WAL does not contain JSON markers:

```go
func TestWALPayloadIsBinary(t *testing.T) {
	dir := t.TempDir()
	log, err := wal.Open(dir, wal.Options{Sync: true})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	record := model.ResolvedPoint{
		Database: "db", RetentionPolicy: "rp", Measurement: "cpu",
		Tags: map[string]string{"host": "a"},
		SeriesID: 1, Timestamp: 10, WriteSeq: 2,
		Fields: []model.ResolvedField{
			{FieldID: 1, FieldName: "value", Type: model.FieldFloat64, Value: model.Float64Value(1.5)},
			{FieldID: 2, FieldName: "state", Type: model.FieldString, Value: model.StringValue("ok")},
		},
	}
	if err := log.Append([]model.ResolvedPoint{record}, true); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "000001.wal"))
	if err != nil {
		t.Fatalf("ReadFile(wal) error = %v", err)
	}
	if bytes.Contains(data, []byte(`"series_id"`)) || bytes.Contains(data, []byte(`{"`)) {
		t.Fatal("wal payload contains JSON marker")
	}
	log, err = wal.Open(dir, wal.Options{})
	if err != nil {
		t.Fatalf("Open(replay) error = %v", err)
	}
	got, err := log.Replay()
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if !reflect.DeepEqual(got, []model.ResolvedPoint{record}) {
		t.Fatalf("Replay() = %#v, want %#v", got, []model.ResolvedPoint{record})
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close(replay) error = %v", err)
	}
}
```

- [x] **Step 2: Run WAL test and verify failure**

Run:

```bash
go test ./internal/wal -run TestWALPayloadIsBinary -timeout 180s
```

Expected: FAIL because current WAL payload is JSON.

- [x] **Step 3: Implement WAL v2 payload codec**

Implementation requirements:

- Keep frame CRC and segment behavior.
- Change payload from `json.Marshal(records)` to binary batch:
  - `batchVersion byte`
  - `pointCount uvarint`
  - each point: database, retention policy, measurement, sorted tags, seriesID, timestamp, writeSeq, field count.
  - each field: fieldID, fieldName, field type, field value.
- Decode must validate buffer exhaustion and unsupported field type.
- Remove `encoding/json` from production WAL package.

- [x] **Step 4: Run WAL package**

Run:

```bash
go test ./internal/wal -timeout 180s
```

Expected: PASS.

实现备注：WAL frame 外壳保留现有 length/version/type/CRC32C，write batch payload 已切换为 batch version `2` 的二进制编码，包含 point count、database/rp/measurement、排序 tags、seriesID、timestamp、writeSeq 和 typed fields。`internal/wal` 已无 `encoding/json` 引用；`go test ./internal/wal -timeout 180s` 通过。滚段测试阈值从 80 调整为 40，以匹配更小的二进制 frame。

## Task 5: SSTable Binary Metadata, Index, And Blocks

**Files:**
- Create: `internal/sstable/encoding.go`
- Create: `internal/sstable/encoding_test.go`
- Modify: `internal/sstable/types.go`
- Modify: `internal/sstable/write.go`
- Modify: `internal/sstable/read.go`
- Modify: `internal/sstable/internal_test.go`
- Modify: `internal/sstable/sstable_test.go`

- [x] **Step 1: Add SSTable encoding tests**

Create tests for timestamp and all field value types:

```go
func TestBinaryBlocksRoundTripAllTypes(t *testing.T) {
	timestamps := []int64{10, 20, 35}
	timePayload := marshalTimeBlock(nil, timestamps)
	gotTimes, err := unmarshalTimeBlock(timePayload)
	if err != nil {
		t.Fatalf("unmarshalTimeBlock() error = %v", err)
	}
	if !slices.Equal(gotTimes, timestamps) {
		t.Fatalf("timestamps = %v, want %v", gotTimes, timestamps)
	}
	columns := []model.ColumnData{
		columnFor(model.FieldFloat64, []model.FieldValue{model.Float64Value(1), model.Float64Value(2)}),
		columnFor(model.FieldInt64, []model.FieldValue{model.Int64Value(1), model.Int64Value(2)}),
		columnFor(model.FieldBool, []model.FieldValue{model.BoolValue(true), model.BoolValue(false)}),
		columnFor(model.FieldString, []model.FieldValue{model.StringValue("a"), model.StringValue("bb")}),
	}
	for _, column := range columns {
		payload, err := marshalValueBlock(nil, column)
		if err != nil {
			t.Fatalf("marshalValueBlock(%v) error = %v", column.FieldType, err)
		}
		got, err := unmarshalValueBlock(payload)
		if err != nil {
			t.Fatalf("unmarshalValueBlock(%v) error = %v", column.FieldType, err)
		}
		if !reflect.DeepEqual(got.Samples, column.Samples) {
			t.Fatalf("samples = %#v, want %#v", got.Samples, column.Samples)
		}
	}
}
```

- [x] **Step 2: Run SSTable encoding tests and verify failure**

Run:

```bash
go test ./internal/sstable -run TestBinaryBlocksRoundTripAllTypes -timeout 180s
```

Expected: FAIL because binary block functions do not exist.

- [x] **Step 3: Implement SSTable v2 binary block codecs**

Required block encodings:

- Time block: `encoding byte + count uvarint + first int64 + delta varints`.
- Float block: `encoding byte + count uvarint + repeated timestamp delta + writeSeq uvarint + float64 little-endian`.
- Int block: same shape with int64 varint values.
- Bool block: timestamps/writeSeq arrays plus bitset values.
- String block: timestamps/writeSeq arrays plus `uvarint length + bytes`.
- Metadata/index/metaindex payloads use binary structs with magic/version envelope.

- [x] **Step 4: Migrate WritePart and OpenPart to binary files**

Implementation requirements:

- `metadata.json` becomes `metadata.bin`.
- `MANIFEST.json` migration happens in Task 6; Part metadata itself is binary here.
- `index.bin`, `metaindex.bin`, `timestamps.bin`, `values.bin`, `strings.bin` are all binary.
- Remove `encoding/json` from production `internal/sstable` except tests if needed.
- `OpenPart` returns unsupported format if old JSON metadata is found.

- [ ] **Step 5: Run SSTable package**

Run:

```bash
go test ./internal/sstable -timeout 180s
```

Expected: PASS.

实现备注：Part metadata 已切换为 `metadata.bin`，旧 `metadata.json` 仅返回 legacy JSON unsupported。`index.bin` 使用 `MTSIDX2` envelope，`metaindex.bin` 使用 `MTSMIX2` envelope，Part metadata 使用 `MTSPRT2` envelope，版本均为 `2`。`timestamps.bin` 使用 delta time block，`values.bin` 使用 typed value block，bool 值使用 bitset。`strings.bin` 仍保留为空文件占位。`go test ./internal/sstable -timeout 180s` 通过；`internal/sstable` 仅剩 `manifest.go` 仍使用 JSON，进入 Task 6 迁移。

## Task 6: Binary Manifest And Crash-Safe Commit

**Files:**
- Modify: `internal/sstable/manifest.go`
- Modify: `internal/sstable/sstable_test.go`
- Modify: `internal/storagefs/fs.go`
- Modify: `internal/storagefs/fs_test.go`

- [x] **Step 1: Add binary manifest tests**

Add tests:

```go
func TestManifestIsBinaryAndRejectsCorruption(t *testing.T) {
	dir := t.TempDir()
	manifest := sstable.Manifest{Parts: []sstable.PartMeta{{ID: "sst-000001", Level: 1, MinTime: 1, MaxTime: 2, Path: filepath.Join(dir, "sst-000001")}}}
	if err := sstable.WriteManifest(dir, manifest); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "MANIFEST.bin"))
	if err != nil {
		t.Fatalf("ReadFile(MANIFEST.bin) error = %v", err)
	}
	if bytes.Contains(data, []byte(`{"`)) {
		t.Fatal("manifest contains JSON marker")
	}
	loaded, err := sstable.LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if !reflect.DeepEqual(loaded, manifest) {
		t.Fatalf("manifest = %#v, want %#v", loaded, manifest)
	}
	data[len(data)-1] ^= 0xff
	if err := os.WriteFile(filepath.Join(dir, "MANIFEST.bin"), data, 0600); err != nil {
		t.Fatalf("WriteFile(corrupt) error = %v", err)
	}
	if _, err := sstable.LoadManifest(dir); err == nil {
		t.Fatal("LoadManifest(corrupt) error = nil, want error")
	}
}
```

- [x] **Step 2: Run manifest test and verify failure**

Run:

```bash
go test ./internal/sstable -run TestManifestIsBinaryAndRejectsCorruption -timeout 180s
```

Expected: FAIL because current manifest is JSON.

- [x] **Step 3: Implement `MANIFEST.bin` and durable rename**

Implementation requirements:

- `WriteManifest` writes `MANIFEST.bin.tmp-*`, fsyncs file, closes it, renames to `MANIFEST.bin`, then fsyncs directory.
- `LoadManifest` reads only `MANIFEST.bin`; missing file returns empty manifest.
- If legacy `MANIFEST.json` exists without `MANIFEST.bin`, return unsupported legacy manifest error.
- `storagefs.WriteFileAtomic` already provides most semantics; ensure it is used or extended for binary payload.

- [x] **Step 4: Run storagefs and sstable packages**

Run:

```bash
go test ./internal/storagefs ./internal/sstable -timeout 180s
```

Expected: PASS.

实现备注：Manifest 已从 `MANIFEST.json` 切换为 `MANIFEST.bin`，使用 `MTSMAN2` envelope、version `2`。缺失 manifest 返回空 manifest；仅存在旧 `MANIFEST.json` 时返回 legacy JSON unsupported；损坏 envelope/CRC 会阻止加载。`WriteManifest` 继续通过 `storagefs.WriteFileAtomic` 完成临时文件、fsync、rename、fsync 目录。`go test ./internal/storagefs ./internal/sstable -timeout 180s` 通过。

## Task 7: Query Pruning And Read Instrumentation

**Files:**
- Modify: `internal/sstable/types.go`
- Modify: `internal/sstable/read.go`
- Modify: `internal/sstable/sstable_test.go`
- Modify: `internal/engine/query.go`

- [x] **Step 1: Add query pruning test with read counters**

Add an internal test that writes multiple fields and asserts query only reads target value block. Use package-private test hooks:

```go
func TestPartQueryPrunesValueBlocksByField(t *testing.T) {
	dir := t.TempDir()
	columns := []model.ColumnData{
		columnWithField(1, 1, model.Float64Value(1)),
		columnWithField(1, 2, model.Int64Value(2)),
		columnWithField(1, 3, model.StringValue("skip")),
	}
	meta, err := WritePart(dir, 0, "sst-000001", columns)
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	stats := part.resetReadStatsForTest()
	got, err := part.Query(Query{SeriesIDs: map[uint64]struct{}{1: {}}, FieldIDs: map[uint32]struct{}{2: {}}, Start: 0, End: 10})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 || got[0].FieldID != 2 {
		t.Fatalf("Query() = %#v, want only field 2", got)
	}
	if stats.ValueBlocksRead != 1 {
		t.Fatalf("ValueBlocksRead = %d, want 1", stats.ValueBlocksRead)
	}
}
```

- [x] **Step 2: Run pruning test and verify failure or missing stats**

Run:

```bash
go test ./internal/sstable -run TestPartQueryPrunesValueBlocksByField -timeout 180s
```

Expected: FAIL until read stats and pruning are implemented.

- [x] **Step 3: Implement metaindex/index pruning**

Implementation requirements:

- Load metaindex rows at `OpenPart`.
- `Part.Query` first checks PartMeta time/series range.
- For matching metaindex rows, read only referenced index blocks.
- For matching index rows, check series/time/field before reading value block.
- Keep test-only stats under unexported struct; production behavior unchanged.

- [x] **Step 4: Run sstable and engine query tests**

Run:

```bash
go test ./internal/sstable ./internal/engine -run 'Query|Part' -timeout 180s
```

Expected: PASS.

实现备注：`Part` 增加未导出的 read stats hook，测试中可验证 `TimeBlocksRead` / `ValueBlocksRead`。`Part.Query` 先按 Part time range、seriesID 范围和 metaindex fieldIDs 做粗剪枝，再按 index row 和 columnRef 读取命中的 value block；定向测试确认查询单字段时只读取 1 个 value block。`go test ./internal/sstable ./internal/engine -run 'Query|Part' -timeout 180s` 通过。

## Task 8: Shard Lifecycle Locking And Flush Ordering

**Files:**
- Modify: `internal/engine/shard.go`
- Modify: `internal/engine/lifecycle.go`
- Modify: `internal/engine/engine_test.go`
- Modify: `internal/engine/error_test.go`

- [x] **Step 1: Add crash-ordering tests**

Add tests with injected failure points. Define package-private hooks:

```go
type shardTestHooks struct {
	afterPartWriteBeforeManifest func() error
	afterManifestBeforeWALTrunc  func() error
}
```

Test:

```go
func TestFlushFailureBeforeManifestDoesNotExposePart(t *testing.T) {
	shard := openTestShard(t)
	shard.testHooks.afterPartWriteBeforeManifest = func() error {
		return errors.New("stop before manifest")
	}
	if err := shard.Write(testResolvedPoint(1, 10, 1), true); err == nil {
		t.Fatal("Write() error = nil, want injected flush error")
	}
	reopened := reopenTestShard(t, shard.opts.Dir)
	got, err := reopened.Query(memtable.Query{Start: 0, End: 20})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("query count = %d, want WAL replayed data only", len(got))
	}
}
```

- [x] **Step 2: Run test and verify failure**

Run:

```bash
go test ./internal/engine -run TestFlushFailureBeforeManifestDoesNotExposePart -timeout 180s
```

Expected: FAIL until hooks and ordering are implemented.

- [x] **Step 3: Implement shard lifecycle locking and flush sequence**

Flush sequence:

1. Lock shard lifecycle mutex.
2. Snapshot memtable.
3. Write Part directory.
4. If injected failure before manifest, return error without manifest change.
5. Write binary manifest atomically.
6. Add part to in-memory part list.
7. Truncate WAL.

If step 7 fails, keep manifest and part; replay may duplicate but LWW must preserve correctness.

- [x] **Step 4: Run engine package**

Run:

```bash
go test ./internal/engine -timeout 180s
```

Expected: PASS.

实现备注：Shard 增加 lifecycle mutex，flush、compact、retention delete 互斥。Flush 顺序调整为：snapshot memtable、写 Part、测试 hook、打开 Part、原子写 manifest、更新内存 parts/manifest、测试 hook、最后截断 WAL。manifest 未提交失败时新 Part 不进入内存和 manifest，重启后仅通过 WAL replay 恢复；manifest 已提交但 WAL 未截断时允许 replay 重复数据，依赖 LWW 合并保持正确性。`go test ./internal/engine -timeout 180s` 通过。

## Task 9: Size-Tiered Compaction

**Files:**
- Modify: `internal/model/types.go`
- Modify: `types.go`
- Modify: `internal/engine/lifecycle.go`
- Modify: `internal/engine/shard.go`
- Modify: `internal/engine/engine_test.go`

- [x] **Step 1: Add compaction options tests**

Test automatic size-tiered compaction trigger:

```go
func TestSizeTieredCompactionTriggersByPartCount(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1,
		Compaction: model.CompactionOptions{
			Enabled:         true,
			Level0PartLimit: 2,
		},
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	for index := range 4 {
		point := model.Point{Measurement: "cpu", Timestamp: int64(index), Fields: map[string]model.FieldValue{"v": model.Float64Value(float64(index))}}
		if err := eng.Write(ctx, []model.Point{point}, model.WriteOptions{Sync: true}); err != nil {
			closeErr := eng.Close(ctx)
			t.Fatalf("Write() error = %v close = %v", err, closeErr)
		}
	}
	shard := onlyShardForTest(t, eng)
	if len(shard.manifest.Parts) > 2 {
		t.Fatalf("part count = %d, want compacted to <=2", len(shard.manifest.Parts))
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
```

- [x] **Step 2: Run compaction trigger test and verify failure**

Run:

```bash
go test ./internal/engine -run TestSizeTieredCompactionTriggersByPartCount -timeout 180s
```

Expected: FAIL because compaction options do not exist.

- [x] **Step 3: Implement compaction options and strategy**

Add:

```go
type CompactionOptions struct {
	Enabled           bool
	Level0PartLimit   int
	Level0SizeLimit   int64
	MaxOutputPartBytes int64
	BackgroundInterval time.Duration
}
```

Rules:

- Defaults: manual compaction remains available; automatic compaction disabled unless `Enabled=true`.
- If `Level0PartLimit <= 0`, default to `4`.
- Trigger after flush if enabled and threshold exceeded.
- Compact only selected level-0 parts into next level; do not always compact every part.
- Manifest update remains atomic and rollback-safe.

- [x] **Step 4: Run engine tests**

Run:

```bash
go test ./internal/engine -timeout 180s
```

Expected: PASS.

实现备注：新增 `model.CompactionOptions` 并通过根包 alias 暴露。默认自动 compaction 关闭；启用且 `Level0PartLimit <= 0` 时默认阈值为 `4`。Flush 后执行 size-tiered 检查，仅选择超过阈值或大小阈值的 level-0 Part 合并到 level 1，已有高层 Part 保留；手动 `Compact` 保持全量合并入口。`go test ./internal/engine -timeout 180s` 通过。

## Task 10: E2E Recovery, Integrity, Retention, And Pruning

**Files:**
- Create: `tests/e2e/wal_recovery/main.go`
- Create: `tests/e2e/flush_manifest_recovery/main.go`
- Create: `tests/e2e/compaction_integrity/main.go`
- Create: `tests/e2e/retention/main.go`
- Create: `tests/e2e/query_pruning/main.go`

- [x] **Step 1: Add e2e cases**

Each case must be a `main` package and follow this pattern:

```go
func main() {
	if err := run(); err != nil {
		log.Fatalf("<case> failed: %v", err)
	}
	log.Print("<case> passed")
}
```

Case requirements:

- `wal_recovery`: write 1000 unflushed points with `Sync=true`, close, reopen, verify 1000 rows.
- `flush_manifest_recovery`: flush multiple Parts, reopen, verify only manifest referenced Parts are visible.
- `compaction_integrity`: write repeated timestamps across Parts, compact, verify LWW value.
- `retention`: write two shards, apply cutoff, verify old shard removed and new shard remains.
- `query_pruning`: write 100 series x 4 fields, query one series/field, verify correct result and no JSON marker remains in files.

- [x] **Step 2: Run every e2e case**

Run:

```bash
for dir in tests/e2e/*; do
  [ -d "$dir" ] || continue
  (cd "$dir" && name=$(basename "$dir") && go build -o "$name" . && ./"$name"; status=$?; rm -f "$name"; exit "$status") || exit 1
done
```

Expected: PASS for every e2e case.

实现备注：已新增 `wal_recovery`、`flush_manifest_recovery`、`compaction_integrity`、`retention`、`query_pruning`。执行 `gofmt -w tests/e2e/*/main.go && for dir in tests/e2e/*; do ...; done` 通过，实际通过目录包括新增 5 个、`no_json_storage` 和 `simple_integrity`，总耗时约 1 秒，运行后已删除 e2e 二进制。

## Task 11: Pprof Workload Modes And Benchmarks

**Files:**
- Modify: `tests/pprof/storage_engine/main.go`
- Modify: `tests/pprof/storage_engine/main_test.go`
- Modify: `internal/bench/storage_bench_test.go`
- Create: `docs/benchmarks/storage-engine-phase2.md`

- [x] **Step 1: Add pprof workload mode tests**

Add table-driven tests for `-mode=write`, `-mode=query`, `-mode=compact`, `-mode=replay`:

```go
func TestRunModes(t *testing.T) {
	for _, mode := range []string{"write", "query", "compact", "replay"} {
		t.Run(mode, func(t *testing.T) {
			err := run([]string{"-mode", mode, "-points", "200", "-series", "10", "-query-repeat", "2"})
			if err != nil {
				t.Fatalf("run(%s) error = %v", mode, err)
			}
		})
	}
}
```

- [x] **Step 2: Implement pprof modes**

Mode behavior:

- `write`: write points only, no flush.
- `query`: write, flush, then query repeatedly.
- `compact`: write enough small flushes, compact.
- `replay`: write synced WAL, close, reopen to force replay.

- [x] **Step 3: Run pprof tests and smoke profile**

Run:

```bash
go test ./tests/pprof/storage_engine -timeout 180s
cd tests/pprof/storage_engine && go build -o storage_engine . && ./storage_engine -mode=query -points 10000 -series 100 -cpu-profile cpu.prof -mem-profile mem.prof; status=$?; test -s cpu.prof && test -s mem.prof; profile_status=$?; rm -f storage_engine cpu.prof mem.prof; if [ $status -ne 0 ]; then exit $status; fi; exit $profile_status
```

Expected: PASS and profile files generated then cleaned.

- [x] **Step 4: Record benchmark baseline**

Run:

```bash
go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s | tee /tmp/mts-storage-phase2-bench.txt
```

Write a short summary to `docs/benchmarks/storage-engine-phase2.md` including command, machine context from `go env GOOS GOARCH`, and benchmark rows.

实现备注：`tests/pprof/storage_engine` 新增 `-mode=write|query|compact|replay`，默认 `query`。`go test ./tests/pprof/storage_engine -timeout 180s` 通过；profile smoke 命令生成 CPU/heap profile 成功并清理。Benchmark 命令 `go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s` 通过，摘要写入 `docs/benchmarks/storage-engine-phase2.md`。

## Task 12: Quality Gate And Commit

**Files:**
- Modify: this plan with completion notes.
- Modify docs if benchmark/e2e command names changed.

- [x] **Step 1: Run full tests with coverage**

Run:

```bash
go test ./... -coverprofile=coverage.out -timeout 600s
go tool cover -func=coverage.out | tail -1
```

Expected: PASS and total coverage `>=90%`.

- [x] **Step 2: Run formatting and lint**

Run:

```bash
timeout 300s goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .
timeout 300s gofmt -w $(find . -name '*.go' -not -path './.git/*')
timeout 720s golangci-lint run --timeout 12m
```

Expected: all commands exit 0 and lint prints `0 issues.`

- [x] **Step 3: Run all e2e cases**

Run:

```bash
for dir in tests/e2e/*; do
  [ -d "$dir" ] || continue
  (cd "$dir" && name=$(basename "$dir") && go build -o "$name" . && ./"$name"; status=$?; rm -f "$name"; exit "$status") || exit 1
done
```

Expected: all e2e binaries pass and are removed.

- [x] **Step 4: Verify no non-code artifacts**

Run:

```bash
rm -f coverage.out
find . -maxdepth 6 -type f \( -name 'coverage.out' -o -name '*.prof' -o -name '*.test' -o -perm /111 \) -not -path './.git/*' -printf '%m %p\n'
```

Expected: no output.

- [x] **Step 5: Commit implementation**

Run:

```bash
git status --short
git add -- internal tests docs types.go
git diff --cached --check
git commit -m "feat(storage): 强化二进制存储引擎"
```

Expected: commit succeeds; staged files do not include binaries, profiles, or coverage files.

实现备注：`go test ./... -coverprofile=coverage.out -timeout 600s` 通过，`go tool cover -func=coverage.out | tail -1` 输出总覆盖率 `90.0%`。`goimports-reviser`、`gofmt`、`golangci-lint run --timeout 12m` 均通过，lint 输出 `0 issues.`。`tests/e2e` 全量 build/run 通过，包括 `compaction_integrity`、`flush_manifest_recovery`、`no_json_storage`、`query_pruning`、`retention`、`simple_integrity`、`wal_recovery`。已删除 `coverage.out`，产物扫描无输出。最终提交信息为 `feat(storage): 强化二进制存储引擎`。

## Self-Review

- Spec coverage: tasks cover binary-only persistence, WAL, Catalog, SSTable blocks, metadata/index/metaindex, manifest consistency, query pruning, compaction, retention locking, benchmarks, pprof, e2e and quality gates.
- 占位标记扫描：未发现未完成项或泛化占位描述。
- Type consistency: all new public option fields are under `model.CompactionOptions` and exposed through root `types.go` aliasing.
