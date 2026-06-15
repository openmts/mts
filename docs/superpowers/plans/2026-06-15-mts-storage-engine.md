# mts Storage Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first embedded `mts` storage engine vertical slice: multi-value time-series writes, WAL recovery, MemTable, directory-style columnar SSTable Parts, query, retention, and compaction.

**Architecture:** The public package `mts` exposes a small embedded API and delegates implementation to internal packages. Each time shard owns its WAL, MemTable, Parts, manifest, flush, compaction, and retention lifecycle. SSTable Parts use separate files for metadata, metaindex, index, timestamps, values, and strings, with CRC32C-protected blocks and simple versioned encodings.

**Tech Stack:** Go 1.26.2, standard library only for production and tests in the first slice, `go test`, `golangci-lint`, `goimports-reviser`.

---

## Constraints

- 任务预计总耗时：3-5 小时；硬超时：8 小时；每 30 秒到 5 分钟按当前动作反馈。
- 单包测试命令使用 `go test ./... -timeout 180s`；全量测试命令使用 `go test ./... -coverprofile=coverage.out -timeout 600s`。
- 目录权限必须为 `0700`，数据文件权限必须为 `0600`。
- 当前目录不是 Git 仓库，不能执行 commit 步骤；计划中的 commit 步骤记录为不可执行。

## File Structure

- Create: `types.go` - public type aliases, constructors, and `Open`.
- Create: `engine.go` - public `Engine` wrapper methods.
- Create: `internal/model/types.go` - shared domain types, field values, resolved points, query and result structs.
- Create: `internal/storagefs/fs.go` - secure mkdir, atomic file write, fsync helpers.
- Create: `internal/catalog/catalog.go` - persistent series/field catalog with WAL and snapshot.
- Create: `internal/wal/wal.go` - CRC-protected shard WAL writer/replayer.
- Create: `internal/memtable/memtable.go` - LWW in-memory table and snapshot/query helpers.
- Create: `internal/sstable/block.go` - CRC block framing.
- Create: `internal/sstable/part.go` - Part writer/reader and file metadata.
- Create: `internal/sstable/manifest.go` - shard manifest load/save.
- Create: `internal/engine/engine.go` - open/close/write/query/flush/compact/retention orchestration.
- Create: `internal/engine/shard.go` - shard-local WAL/MemTable/Part operations.
- Create tests next to each package and public integration tests in `engine_test.go`.

## Task 1: Public Model And API Surface

**Files:**
- Create: `internal/model/types.go`
- Create: `types.go`
- Create: `engine.go`
- Test: `types_test.go`

- [x] **Step 1: Write failing public API tests**

```go
func TestFieldValueConstructors(t *testing.T) {
    cases := []mts.FieldValue{
        mts.Float64Value(1.5),
        mts.Int64Value(-2),
        mts.StringValue("ok"),
        mts.BoolValue(true),
    }
    require.Equal(t, mts.FieldFloat64, cases[0].Type)
    require.Equal(t, 1.5, cases[0].Float64)
    require.Equal(t, mts.FieldInt64, cases[1].Type)
    require.Equal(t, int64(-2), cases[1].Int64)
    require.Equal(t, mts.FieldString, cases[2].Type)
    require.Equal(t, "ok", cases[2].String)
    require.Equal(t, mts.FieldBool, cases[3].Type)
    require.True(t, cases[3].Bool)
}
```

- [x] **Step 2: Run targeted test**

Run: `go test ./... -run TestFieldValueConstructors -timeout 180s`

Expected: FAIL because public types are not implemented.

- [x] **Step 3: Implement public and internal model types**

Define `FieldType`, `FieldValue`, constructors, `Point`, `Query`, `ColumnSeries`, `Row`, `Options`, `WALOptions`, `WriteOptions`, and `Engine` wrapper signatures.

- [x] **Step 4: Run targeted test**

Run: `go test ./... -run TestFieldValueConstructors -timeout 180s`

Expected: PASS.

实现备注：已创建 `internal/model/types.go` 与根包 `types.go`，使用 type alias 暴露公共模型；`go test ./... -run TestFieldValueConstructors -timeout 180s` 通过。

## Task 2: Secure Filesystem Helpers

**Files:**
- Create: `internal/storagefs/fs.go`
- Test: `internal/storagefs/fs_test.go`

- [x] **Step 1: Test secure directory and atomic file permissions**

```go
func TestSecureDirsAndAtomicFile(t *testing.T) {
    root := t.TempDir()
    dir := filepath.Join(root, "data")
    require.NoError(t, storagefs.MkdirAll(dir))
    info, err := os.Stat(dir)
    require.NoError(t, err)
    require.Equal(t, fs.FileMode(0700), info.Mode().Perm())

    path := filepath.Join(dir, "meta.json")
    require.NoError(t, storagefs.WriteFileAtomic(path, []byte("ok")))
    info, err = os.Stat(path)
    require.NoError(t, err)
    require.Equal(t, fs.FileMode(0600), info.Mode().Perm())
}
```

- [x] **Step 2: Run targeted test**

Run: `go test ./internal/storagefs -timeout 180s`

Expected: FAIL because package is missing.

- [x] **Step 3: Implement helpers**

Implement `MkdirAll`, `WriteFileAtomic`, `SyncDir`, and `RemoveAll` wrappers using explicit error handling.

- [x] **Step 4: Run targeted test**

Run: `go test ./internal/storagefs -timeout 180s`

Expected: PASS.

实现备注：已创建 `internal/storagefs`，覆盖 `0700` 目录、`0600` 文件、原子写入和目录 fsync；`go test ./internal/storagefs -timeout 180s` 通过。

## Task 3: Persistent Catalog

**Files:**
- Create: `internal/catalog/catalog.go`
- Test: `internal/catalog/catalog_test.go`

- [x] **Step 1: Test series allocation, field type conflict, and reopen**

```go
func TestCatalogResolveReopenAndTypeConflict(t *testing.T) {
    dir := t.TempDir()
    cat, err := catalog.Open(dir)
    require.NoError(t, err)
    p := model.Point{
        Measurement: "cpu",
        Tags: map[string]string{"host": "a"},
        Fields: map[string]model.FieldValue{"usage": model.Float64Value(1)},
    }
    rp, err := cat.ResolvePoint(p)
    require.NoError(t, err)
    require.NoError(t, cat.Close())

    cat, err = catalog.Open(dir)
    require.NoError(t, err)
    rp2, err := cat.ResolvePoint(p)
    require.NoError(t, err)
    require.Equal(t, rp.SeriesID, rp2.SeriesID)

    p.Fields["usage"] = model.StringValue("bad")
    _, err = cat.ResolvePoint(p)
    require.Error(t, err)
    require.NoError(t, cat.Close())
}
```

- [x] **Step 2: Run targeted test**

Run: `go test ./internal/catalog -timeout 180s`

Expected: FAIL because catalog is missing.

- [x] **Step 3: Implement catalog WAL and snapshot**

Implement JSON snapshot and CRC32C-verified JSONL WAL records for series and field creation. `ResolvePoint` persists new catalog entries before returning resolved IDs.

- [x] **Step 4: Run targeted test**

Run: `go test ./internal/catalog -timeout 180s`

Expected: PASS.

实现备注：已创建 `internal/catalog`，使用 CRC32C JSONL WAL 与 JSON snapshot；覆盖重启恢复、字段类型冲突和按 tag 查询；`go test ./internal/catalog -timeout 180s` 通过。

## Task 4: Shard WAL

**Files:**
- Create: `internal/wal/wal.go`
- Test: `internal/wal/wal_test.go`

- [x] **Step 1: Test append, replay, sync, rollover, and tail truncation**

```go
func TestWALAppendReplayAndTruncateTail(t *testing.T) {
    dir := t.TempDir()
    log, err := wal.Open(dir, wal.Options{SegmentBytes: 128, Sync: true})
    require.NoError(t, err)
    record := model.ResolvedPoint{SeriesID: 1, Timestamp: 10, WriteSeq: 7}
    require.NoError(t, log.Append([]model.ResolvedPoint{record}, true))
    require.NoError(t, log.Close())

    log, err = wal.Open(dir, wal.Options{SegmentBytes: 128})
    require.NoError(t, err)
    got, err := log.Replay()
    require.NoError(t, err)
    require.Equal(t, []model.ResolvedPoint{record}, got)
    require.NoError(t, log.Close())
}
```

- [x] **Step 2: Run targeted test**

Run: `go test ./internal/wal -timeout 180s`

Expected: FAIL because WAL is missing.

- [x] **Step 3: Implement WAL record framing**

Implement `[length][version][type][payload][crc32c]`, sorted segment replay, segment rollover, fsync policy, `TruncateAll`, and tail truncation for incomplete final records.

- [x] **Step 4: Run targeted test**

Run: `go test ./internal/wal -timeout 180s`

Expected: PASS.

实现备注：已创建 `internal/wal`，实现二进制 WAL frame、CRC32C、segment 滚动、尾部半条截断和中间损坏检测；`go test ./internal/wal -timeout 180s` 通过。

## Task 5: MemTable

**Files:**
- Create: `internal/memtable/memtable.go`
- Test: `internal/memtable/memtable_test.go`

- [x] **Step 1: Test LWW, unordered writes, snapshot, and query**

```go
func TestMemTableLWWAndSnapshot(t *testing.T) {
    mt := memtable.New()
    older := model.ResolvedPoint{SeriesID: 1, Timestamp: 10, WriteSeq: 1, Fields: []model.ResolvedField{{FieldID: 2, Type: model.FieldFloat64, Value: model.Float64Value(1)}}}
    newer := model.ResolvedPoint{SeriesID: 1, Timestamp: 10, WriteSeq: 2, Fields: []model.ResolvedField{{FieldID: 2, Type: model.FieldFloat64, Value: model.Float64Value(3)}}}
    require.NoError(t, mt.Apply(older))
    require.NoError(t, mt.Apply(newer))
    snap := mt.SnapshotAndReset()
    cols := snap.Query(memtable.Query{SeriesIDs: map[uint64]struct{}{1: {}}, FieldIDs: map[uint32]struct{}{2: {}}, Start: 0, End: 20})
    require.Len(t, cols, 1)
    require.Equal(t, 3.0, cols[0].Samples[0].Value.Float64)
}
```

- [x] **Step 2: Run targeted test**

Run: `go test ./internal/memtable -timeout 180s`

Expected: FAIL because MemTable is missing.

- [x] **Step 3: Implement MemTable**

Implement nested maps with sorted query output and memory sample counting.

- [x] **Step 4: Run targeted test**

Run: `go test ./internal/memtable -timeout 180s`

Expected: PASS.

实现备注：已创建 `internal/memtable`，实现 LWW、乱序查询排序、snapshot/reset 和列式输出；`go test ./internal/memtable -timeout 180s` 通过。

## Task 6: SSTable Part And Manifest

**Files:**
- Create: `internal/sstable/block.go`
- Create: `internal/sstable/part.go`
- Create: `internal/sstable/manifest.go`
- Test: `internal/sstable/sstable_test.go`

- [x] **Step 1: Test Part write/read, field pruning, CRC failure, manifest save/load**

```go
func TestPartWriteReadAndManifest(t *testing.T) {
    dir := t.TempDir()
    cols := []model.ColumnData{{SeriesID: 1, FieldID: 2, FieldType: model.FieldFloat64, Samples: []model.VersionedSample{{Timestamp: 10, WriteSeq: 1, Value: model.Float64Value(5)}}}}
    meta, err := sstable.WritePart(dir, 0, "sst-000001", cols)
    require.NoError(t, err)
    part, err := sstable.OpenPart(filepath.Join(dir, meta.ID))
    require.NoError(t, err)
    got, err := part.Query(sstable.Query{SeriesIDs: map[uint64]struct{}{1: {}}, FieldIDs: map[uint32]struct{}{2: {}}, Start: 0, End: 20})
    require.NoError(t, err)
    require.Equal(t, cols[0].Samples[0].Value.Float64, got[0].Samples[0].Value.Float64)
}
```

- [x] **Step 2: Run targeted test**

Run: `go test ./internal/sstable -timeout 180s`

Expected: FAIL because SSTable is missing.

- [x] **Step 3: Implement Part and manifest**

Implement directory Part files, CRC block wrapper, JSON metadata/index/metaindex payloads, timestamp/value blocks, `OpenPart`, `Query`, `WriteManifest`, and `LoadManifest`.

- [x] **Step 4: Run targeted test**

Run: `go test ./internal/sstable -timeout 180s`

Expected: PASS.

实现备注：已创建 `internal/sstable`，实现 CRC block、目录式 Part、metadata/index/metaindex/timestamps/values/strings 文件、字段裁剪、manifest 保存加载；`go test ./internal/sstable -timeout 180s` 通过。

## Task 7: Engine, Shard, Flush, Recovery, Query

**Files:**
- Create: `internal/engine/engine.go`
- Create: `internal/engine/shard.go`
- Modify: `types.go`
- Modify: `engine.go`
- Test: `engine_test.go`

- [x] **Step 1: Test write/query/flush/reopen**

```go
func TestEngineWriteFlushReopenQueryRows(t *testing.T) {
    dir := t.TempDir()
    eng, err := mts.Open(context.Background(), mts.Options{Path: dir, ShardDuration: time.Hour, MemTableMaxSamples: 2})
    require.NoError(t, err)
    point := mts.Point{Measurement: "cpu", Tags: map[string]string{"host": "a"}, Timestamp: time.Unix(0, 10).UnixNano(), Fields: map[string]mts.FieldValue{"usage": mts.Float64Value(1.5), "state": mts.StringValue("ok")}}
    require.NoError(t, eng.Write(context.Background(), []mts.Point{point}, mts.WriteOptions{Sync: true}))
    rows, err := collectRows(eng.QueryRows(context.Background(), mts.Query{Measurement: "cpu", Tags: map[string]string{"host": "a"}, StartTime: 0, EndTime: time.Hour.Nanoseconds()}))
    require.NoError(t, err)
    require.Len(t, rows, 1)
    require.NoError(t, eng.Close(context.Background()))
}
```

- [x] **Step 2: Run targeted test**

Run: `go test ./... -run TestEngineWriteFlushReopenQueryRows -timeout 180s`

Expected: FAIL because engine orchestration is missing.

- [x] **Step 3: Implement engine and shard orchestration**

Implement `Open`, `Close`, `Write`, `Flush`, `QueryColumns`, `QueryRows`, shard discovery, WAL replay, manifest loading, MemTable plus Part query merge, and LWW result assembly.

- [x] **Step 4: Run targeted test**

Run: `go test ./... -run TestEngineWriteFlushReopenQueryRows -timeout 180s`

Expected: PASS.

实现备注：已创建 `internal/engine` 与根包 wrapper，实现 Open/Close/Write/Flush/QueryColumns/QueryRows、shard discovery、WAL replay、manifest Part 加载、MemTable+Part LWW 合并；`go test ./... -run 'TestEngine(WriteFlushReopenQueryRows|ReplaysUnflushedWAL)' -timeout 180s` 通过。

## Task 8: Compaction And Retention

**Files:**
- Modify: `internal/engine/engine.go`
- Modify: `internal/engine/shard.go`
- Test: `engine_test.go`

- [x] **Step 1: Test compaction LWW and retention shard drop**

```go
func TestEngineCompactionAndRetention(t *testing.T) {
    dir := t.TempDir()
    eng, err := mts.Open(context.Background(), mts.Options{Path: dir, ShardDuration: time.Hour, Retention: time.Hour, MemTableMaxSamples: 1})
    require.NoError(t, err)
    first := mts.Point{Measurement: "cpu", Timestamp: 10, Fields: map[string]mts.FieldValue{"v": mts.Float64Value(1)}}
    second := mts.Point{Measurement: "cpu", Timestamp: 10, Fields: map[string]mts.FieldValue{"v": mts.Float64Value(2)}}
    require.NoError(t, eng.Write(context.Background(), []mts.Point{first, second}, mts.WriteOptions{Sync: true}))
    require.NoError(t, eng.Compact(context.Background()))
    rows, err := collectRows(eng.QueryRows(context.Background(), mts.Query{Measurement: "cpu", StartTime: 0, EndTime: 20}))
    require.NoError(t, err)
    require.Equal(t, 2.0, rows[0].Fields["v"].Float64)
    require.NoError(t, eng.ApplyRetention(context.Background(), time.Unix(0, int64(2*time.Hour))))
    require.NoError(t, eng.Close(context.Background()))
}
```

- [x] **Step 2: Run targeted test**

Run: `go test ./... -run TestEngineCompactionAndRetention -timeout 180s`

Expected: FAIL until compaction/retention are implemented.

- [x] **Step 3: Implement compaction and retention**

Implement per-shard compaction by reading active Parts plus MemTable snapshot into merged columns, writing a new higher-level Part, atomically replacing manifest entries, and removing obsolete Part directories. Implement retention by dropping expired shard directories after manifest state is closed.

- [x] **Step 4: Run targeted test**

Run: `go test ./... -run TestEngineCompactionAndRetention -timeout 180s`

Expected: PASS.

实现备注：已实现 `Compact` 和 `ApplyRetention`，compaction 合并多 Part 并按 `writeSeq` 保留最新值，retention 按过期 shard 删除目录；`go test ./... -run TestEngineCompactionAndRetention -timeout 180s` 通过。

## Task 9: Quality Gate

**Files:**
- Modify tests as needed without changing production behavior.

- [x] **Step 1: Run all unit tests with coverage**

Run: `go test ./... -coverprofile=coverage.out -timeout 600s`

Expected: PASS and line coverage `>=90%`.

- [x] **Step 2: Run formatting and linting**

Run: `goimports-reviser ./...`

Expected: PASS or tool unavailable; if unavailable, record it and use `gofmt`.

Run: `golangci-lint run --timeout 12m`

Expected: PASS or tool unavailable; if unavailable, record it.

- [x] **Step 3: Run e2e tests if present**

Run each `tests/e2e/*` by `go build` and executing the produced binary with a bounded timeout.

Expected: All present e2e tests pass. If `tests/e2e` does not exist, record that e2e is not applicable yet.

- [x] **Step 4: Clean temporary artifacts**

Remove `coverage.out` and any test binaries generated by e2e commands.

Expected: No temporary build artifacts remain.

实现备注：已补充 `storagefs` 与 `wal` 错误路径/边界行为测试；`go test ./... -coverprofile=coverage.out -timeout 600s` 通过，总行覆盖率 `90.5%`，其中 `wal` 包覆盖率 `90.8%`；`goimports-reviser -project-name codeberg.org/mts/mts -recursive -format -rm-unused .`、`gofmt` 与 `golangci-lint run --timeout 12m` 均通过。当前仓库不存在 `tests/e2e` 目录，e2e 不适用；已删除 `coverage.out`，未发现 e2e 测试二进制产物。

## Self-Review

- Spec coverage: tasks cover public API, filesystem permissions, catalog, WAL, MemTable, SSTable Part, manifest, recovery, query, compaction, retention, and verification.
- Placeholder scan: no `TBD`, `TODO`, or unspecified implementation steps remain.
- Type consistency: public aliases point to `internal/model`; internal packages exchange `ResolvedPoint`, `ColumnData`, and `VersionedSample`.
