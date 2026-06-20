# Storage Performance Matrix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建覆盖 100K、1M、10M、压缩算法和写入持久化策略的存储层性能矩阵用例。

**Architecture:** `tests/scale/storage_10m` 作为单场景执行器，新增 durability 参数和 strict flush 同步能力；`tests/scale/storage_matrix` 作为矩阵编排器，调用单场景 runner 并聚合 JSON/Markdown 报告。SSTable flush sync 通过显式选项下推到 writer，不改变默认 buffered 性能口径。

**Tech Stack:** Go、`testing`、`os/exec`、现有 `mts` public API、`internal/sstable`、`internal/storagefs`、`goimports-reviser`、`golangci-lint`。

---

### Task 1: 扩展单场景 durability 参数

**Files:**
- Modify: `tests/scale/storage_10m/main.go`
- Modify: `tests/scale/storage_10m/main_test.go`

- [x] **Step 1: 写失败测试**

新增测试：

```go
func TestParseConfigAcceptsDurabilityModes(t *testing.T) {
	for _, mode := range []string{"buffered", "wal-sync", "write-sync", "strict-flush"} {
		t.Run(mode, func(t *testing.T) {
			cfg, err := parseConfig([]string{"-points", "10", "-durability", mode})
			if err != nil {
				t.Fatalf("parseConfig() error = %v", err)
			}
			if cfg.durability != mode {
				t.Fatalf("durability = %q, want %q", cfg.durability, mode)
			}
		})
	}
}
```

Run:

```bash
timeout 180s go test ./tests/scale/storage_10m -run TestParseConfigAcceptsDurabilityModes -count=1
```

Expected: FAIL，提示 `durability` 字段或 flag 不存在。

- [x] **Step 2: 实现最小解析和报告字段**

在 `config` 增加 `durability string`，在 `report` 增加 `Durability string`、`WALSync bool`、`WriteSync bool`、`FlushSync bool`。新增 `validDurability` 和 `durabilityOptions`。

- [x] **Step 3: 运行测试确认通过**

Run:

```bash
timeout 180s go test ./tests/scale/storage_10m -run 'TestParseConfigAcceptsDurabilityModes|TestReportIncludesAmplificationAndLevelDistribution' -count=1
```

Expected: PASS。

**实现备注：** 已在 `storage_10m` 配置和报告中加入 durability 及 `wal_sync`、`write_sync`、`flush_sync` 字段，解析测试覆盖四种持久化模式。

### Task 2: 实现 strict flush SSTable 同步

**Files:**
- Modify: `types.go`
- Modify: `internal/model/types.go`
- Modify: `internal/engine/paths.go`
- Modify: `internal/engine/shard.go`
- Modify: `internal/sstable/write.go`
- Modify: `internal/sstable/stream_write.go`
- Modify: `internal/sstable/internal_test.go`

- [x] **Step 1: 写失败测试**

新增 SSTable strict sync 故障测试：打开 fault controller，让 `OpSync` 失败，调用 `sstable.WritePartWithOptions(..., sstable.WriteOptions{Sync:true})`，期望返回错误。

Run:

```bash
timeout 180s go test ./internal/sstable -run TestWritePartWithSyncReturnsSyncError -count=1
```

Expected: FAIL，提示 `Sync` 字段不存在或未返回 sync 错误。

- [x] **Step 2: 实现 sync 下推**

在 `sstable.WriteOptions` 增加 `Sync bool`。`partFiles.close`、`writeIndexBlocks`、`writeBinaryBlock`、`ensureStringsFile` 在 Sync 模式下先 `storagefs.Sync(file)` 再 close。`PartWriter.Close` 在成功写完 metadata 和 strings 后执行 `storagefs.SyncDir(w.path)`。

- [x] **Step 3: 暴露 Engine flush sync 配置**

在 public `Options` 和 internal `model.Options` 增加 `FlushSync bool`。`toModelOptions` 转换该字段。`Shard.flushWriteOptions` 设置 `sstable.WriteOptions{Sync:s.opts.FlushSync}`。

- [x] **Step 4: 运行 SSTable 和 Engine 定向测试**

Run:

```bash
timeout 240s go test ./internal/sstable ./internal/engine -run 'TestWritePartWithSyncReturnsSyncError|Test.*Flush.*' -count=1
```

Expected: PASS。

**实现备注：** 已新增 `FlushSync` 配置并下推到 SSTable `WriteOptions.Sync`，strict flush 会同步数据、索引、元数据、字符串文件和 part 目录；SSTable sync fault 测试覆盖错误返回。

### Task 3: 把 durability 映射到单场景执行

**Files:**
- Modify: `tests/scale/storage_10m/main.go`
- Modify: `tests/scale/storage_10m/main_test.go`

- [x] **Step 1: 写失败测试**

新增测试覆盖：

```go
func TestDurabilityOptionsMapToEngineAndWriteOptions(t *testing.T) {
	tests := []struct {
		name      string
		wantWAL   bool
		wantWrite bool
		wantFlush bool
	}{
		{name: "buffered"},
		{name: "wal-sync", wantWAL: true},
		{name: "write-sync", wantWrite: true},
		{name: "strict-flush", wantWAL: true, wantWrite: true, wantFlush: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := durabilityOptions(tt.name)
			if got.walSync != tt.wantWAL || got.writeSync != tt.wantWrite || got.flushSync != tt.wantFlush {
				t.Fatalf("durabilityOptions(%q) = %#v", tt.name, got)
			}
		})
	}
}
```

Run:

```bash
timeout 180s go test ./tests/scale/storage_10m -run TestDurabilityOptionsMapToEngineAndWriteOptions -count=1
```

Expected: FAIL，提示映射函数不存在。

- [x] **Step 2: 实现映射**

`openScaleEngine` 使用 `WAL.Sync` 和 `FlushSync`。`writeScaleBatches` 根据 durability 设置 `mts.WriteOptions{Sync:true}`。

- [x] **Step 3: 运行定向测试**

Run:

```bash
timeout 180s go test ./tests/scale/storage_10m -run 'TestParseConfigAcceptsDurabilityModes|TestDurabilityOptionsMapToEngineAndWriteOptions|TestRunWorkloadModes' -count=1
```

Expected: PASS。

**实现备注：** `buffered`、`wal-sync`、`write-sync`、`strict-flush` 已分别映射到 WAL sync、write sync 和 flush sync，单场景写入路径按 durability 设置写入选项。

### Task 4: 新增 storage_matrix 编排器

**Files:**
- Create: `tests/scale/storage_matrix/main.go`
- Create: `tests/scale/storage_matrix/main_test.go`
- Create: `tests/scale/storage_matrix/README.md`

- [x] **Step 1: 写失败测试**

测试覆盖 size 解析、列表解析、case 生成和命令参数生成。

Run:

```bash
timeout 180s go test ./tests/scale/storage_matrix -run 'TestParseSizes|TestBuildCases|TestRunnerArgs' -count=1
```

Expected: FAIL，包不存在。

- [x] **Step 2: 实现矩阵配置和 case 生成**

实现 flags：`-sizes`、`-compressions`、`-durabilities`、`-data-root`、`-runner`、`-out`、`-markdown`、`-case-timeout`、`-batch-size`、`-memtable-max-samples`、`-query-limit`。

- [x] **Step 3: 实现 runner 调用和报告聚合**

当 `-runner` 为空时，构建临时 `storage_10m` runner；否则使用指定 runner。每个 case 调用 runner，解析 JSON，记录成功或失败。

- [x] **Step 4: 实现 Markdown 汇总**

输出表格，包含 case、write、compaction、query、RSS、data bytes、SSTable before/after 和 status。

- [x] **Step 5: 运行矩阵包测试**

Run:

```bash
timeout 180s go test ./tests/scale/storage_matrix -count=1
```

Expected: PASS。

**实现备注：** 已新增 `tests/scale/storage_matrix` 编排器，支持规模、压缩、持久化、数据目录、runner、输出文件、case timeout、batch、MemTable 和 query limit 参数，并生成 JSON/Markdown 汇总。自动构建 runner 时会向上定位仓库根目录，避免从子目录运行时找错 `storage_10m` 包。

### Task 5: smoke 执行和文档更新

**Files:**
- Modify: `docs/benchmarks/storage-engine-10m-pprof-2026-06-18.md`
- Modify: `docs/superpowers/plans/2026-06-19-storage-performance-matrix.md`

- [x] **Step 1: 运行格式化**

Run:

```bash
timeout 300s goimports-reviser -rm-unused -set-alias -format ./...
```

Expected: exit 0。

- [x] **Step 2: 运行定向测试**

Run:

```bash
timeout 240s go test ./tests/scale/storage_10m ./tests/scale/storage_matrix ./internal/sstable ./internal/engine -count=1
```

Expected: PASS。

- [x] **Step 3: 运行矩阵 smoke**

Run:

```bash
timeout 600s go run ./tests/scale/storage_matrix \
  -sizes 100k \
  -compressions off,snappy \
  -durabilities buffered,write-sync \
  -case-timeout 300s \
  -markdown /tmp/mts-storage-matrix-smoke.md \
  -out /tmp/mts-storage-matrix-smoke.json
```

Expected: exit 0，JSON 和 Markdown 均生成。

- [x] **Step 4: 更新 benchmark 文档**

把 smoke 结果和全量矩阵执行命令写入 `docs/benchmarks/storage-engine-10m-pprof-2026-06-18.md`。

**实现备注：** benchmark 文档已写入矩阵 runner 用法、100K 完整矩阵结果和 1M/10M 全量矩阵的执行命令；本轮最终验证会重新运行格式化、全量测试和 lint。

### Task 6: 最终验证

**Files:**
- All modified files

- [x] **Step 1: 全量 Go 测试**

Run:

```bash
timeout 720s go test ./... -count=1 -timeout 10m
```

Expected: PASS。

- [x] **Step 2: lint**

Run:

```bash
timeout 720s golangci-lint run ./...
```

Expected: `0 issues.`

- [x] **Step 3: 清理临时产物**

Run:

```bash
rm -f /tmp/mts-storage-matrix-smoke.json /tmp/mts-storage-matrix-smoke.md
```

Expected: no temp benchmark artifacts remain in repo.

**实现备注：** 已重新执行 `go test ./... -count=1 -timeout 10m`、`golangci-lint run ./...`、`go test ./tests/scale/storage_matrix -cover -count=1` 和 100K smoke 矩阵；矩阵包覆盖率为 `90.3%`。临时 JSON/Markdown 输出和矩阵临时目录已清理。
