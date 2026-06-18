# Storage Engine Phase 5 SSTable V3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make SSTable storage denser and reads cheaper by encoding value block timestamps as references into the row time block.

**Architecture:** Keep the SSTable row time block and index layout. Add value block v3 for new writes, keep v2 read compatibility, and pass row timestamps to the value decoder during query.

**Tech Stack:** Go 1.26.2, standard library only, `go test`, `pprof`, `goimports-reviser`, `gofmt`, `golangci-lint`.

---

## Constraints And Budgets

- 预计耗时：4-8 小时；硬超时：16 小时。
- 单包测试超时：180s；全量测试超时：600s；benchmark 超时：900s；百万级 pprof smoke 超时：1200s；lint 超时：12m。
- 不改变 public API，不改变 WAL 格式。
- 新增文件权限保持 `0600`，新增目录权限保持 `0700`。
- 禁止在 for 循环中使用 defer。
- 每个行为变更先写失败测试，再实现。
- 每完成一个 task，更新本计划 checkbox 和实现备注。

## File Structure

- Modify: `internal/sstable/encoding.go` - add value block v3 encoder/decoder and direct sample decoding.
- Modify: `internal/sstable/read.go` - pass row timestamps into value block decoder.
- Modify: `internal/sstable/write.go` - pass row timestamps into value encoder and reduce block frame allocation.
- Modify: `internal/sstable/internal_test.go` - add v3 corruption and frame tests.
- Modify: `internal/sstable/encoding_test.go` - add aligned/indexed/v2 compatibility tests.
- Create: `docs/benchmarks/storage-engine-phase5.md` - record disk size and benchmark evidence.

## Task 1: SSTable V3 Encoding Tests

- [x] **Step 1: Add failing tests**

Add tests proving:
- aligned v3 roundtrip is shorter than v2 for dense columns.
- indexed v3 roundtrip handles sparse columns.
- v3 rejects missing row timestamps and out-of-range ordinal.
- v2 payload still decodes through compatibility path.

- [x] **Step 2: Run red test**

Run:

```bash
go test ./internal/sstable -run 'TestValueBlockV3|TestSSTableBinaryDecodersRejectTruncatedPrefixes' -timeout 180s
```

Expected: FAIL because v3 encoder/decoder does not exist.

实现备注：已新增 v3 aligned/indexed/v2 兼容和非法 time reference 测试。执行 `go test ./internal/sstable -run 'TestValueBlockV3|TestValueBlockV2CompatibilityThroughTimestampDecoder|TestSSTableBinaryDecodersRejectTruncatedPrefixes' -timeout 180s` 按预期编译失败，缺失 v3 函数和常量。

## Task 2: Implement Value Block V3

- [x] **Step 1: Add encoding constants and mode helpers**

Add `valueEncodingV3`, `timeRefModeAligned`, `timeRefModeIndexed`.

- [x] **Step 2: Encode time references**

Write aligned mode when `len(column.Samples) == len(rowTimestamps)` and timestamps match positionally. Otherwise encode strictly increasing ordinals found by walking sorted row timestamps and column samples.

- [x] **Step 3: Decode samples directly**

Decode v3 by resolving timestamp from row timestamps and reading write seq/value in one pass into `[]VersionedSample`.

- [x] **Step 4: Keep v2 compatibility**

Keep `unmarshalValueBlock(payload)` for v2 callers and add `unmarshalValueBlockWithTimestamps(payload, rowTimestamps, query)`.

- [x] **Step 5: Run sstable tests**

Run:

```bash
go test ./internal/sstable -timeout 180s
```

Expected: PASS.

实现备注：已新增 value block v3，支持 aligned/indexed time reference；`unmarshalValueBlockWithTimestamps` 保留 v2 兼容并按 query 过滤样本。执行 `go test ./internal/sstable -timeout 180s` 通过。

## Task 3: Wire Query And Writer

- [x] **Step 1: Writer uses v3**

Change `writeSeries` to pass row timestamps to `writeValueBlock`, and `writeValueBlock` to call v3 encoder.

- [x] **Step 2: Reader passes row timestamps**

Change `queryRow` to retain `timeBlock.Timestamps` and pass them to `readValueColumn`.

- [x] **Step 3: Run engine and e2e smoke**

Run:

```bash
go test ./internal/engine ./internal/sstable -timeout 180s
```

Expected: PASS.

实现备注：`writeSeries` 已将 row timestamps 传给 `writeValueBlock`，新写入默认使用 v3；`queryRow` 已读取 time block 并传给 value decoder。执行 `go test ./internal/engine ./internal/sstable -timeout 180s` 通过。

## Task 4: Reduce Block Frame Allocation

- [x] **Step 1: Change `writeBlock`**

Write 4-byte length header, payload, and 4-byte CRC directly to file without allocating `4+payload+4` frame.

- [x] **Step 2: Run block tests**

Run:

```bash
go test ./internal/sstable -run 'TestBlockReadValidationErrors|TestPartWriteReadFieldPruneAndManifest' -timeout 180s
```

Expected: PASS.

实现备注：`writeBlock` 已改为分段写入 header、payload、crc，避免完整 frame 临时分配，并处理零字节写入异常。执行 `go test ./internal/sstable -run 'TestBlockReadValidationErrors|TestPartWriteReadFieldPruneAndManifest|TestValueBlockV3' -timeout 180s` 通过。

## Task 5: Benchmark, Profile, Disk Size

- [x] **Step 1: Add or run disk size smoke**

Run a wide10 write+flush workload and record total data directory size.

- [x] **Step 2: Run benchmark**

```bash
go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s
```

- [x] **Step 3: Run pprof smoke**

```bash
timeout 1200s go run ./tests/pprof/storage_engine -mode=write -field-layout=wide10 -points 1000000 -series 10000 -mem-profile /tmp/mts-wide10-phase5-mem.prof
go tool pprof -inuse_space -top /tmp/mts-wide10-phase5-mem.prof
go tool pprof -alloc_space -top /tmp/mts-wide10-phase5-mem.prof
```

- [x] **Step 4: Record results**

Create `docs/benchmarks/storage-engine-phase5.md`.

实现备注：已记录 100K wide10 数据目录大小约 `29.4MB`。最终 benchmark wide10 10K 约 `44.4ms/op`、`64.7MB/op`、`166.0k allocs/op`。1M wide10 pprof smoke `real 20.15s`，in-use heap 约 `9.3MB`，alloc_space 约 `9.16GB`。query smoke `real 8.60s`。结果已写入 `docs/benchmarks/storage-engine-phase5.md`。

## Task 6: Final Verification

- [x] **Step 1: Format**

```bash
goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .
gofmt -w internal/sstable/*.go
```

- [x] **Step 2: Full tests and coverage**

```bash
go test ./... -coverprofile=coverage.out -timeout 600s
go tool cover -func=coverage.out | tail -1
```

Expected: total coverage `>=90%`.

- [x] **Step 3: Lint**

```bash
golangci-lint run --timeout 12m
```

- [x] **Step 4: E2E**

For each directory under `tests/e2e`, run `go build`, execute the binary, then remove it.

- [x] **Step 5: Clean artifacts**

Remove `coverage.out`, `/tmp/mts-wide10-phase5-mem.prof`, and any e2e binaries.

实现备注：已执行 `goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .` 和 `gofmt`。已执行 `go test ./... -coverprofile=coverage.out -timeout 600s`，总覆盖率 `90.0%`。已执行 `golangci-lint run --timeout 12m`，结果 `0 issues`。已逐个 build/run `tests/e2e` 现有目录：`compaction_integrity`、`flush_manifest_recovery`、`no_json_storage`、`query_pruning`、`retention`、`simple_integrity`、`wal_recovery`，全部通过。已清理 `coverage.out`、pprof/coverage 临时文件、`/tmp/mts-phase5-size` 和 e2e 临时二进制。
