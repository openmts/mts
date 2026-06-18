# Storage Engine pprof 执行报告 2026-06-18

## 结论

本轮对 `tests/pprof/storage_engine` 的 100K wide10 写入和查询用例做了同机同参数 baseline/after 对比。确认 WAL batch identity 查重存在可优化空间，并已将 WAL identity lookup 从“每个新 identity 构造二进制字符串 key”优化为“优先按 SeriesID 建立 ref 索引，连续 SeriesID 使用 dense slice，稀疏 SeriesID 回退 map，SeriesID 冲突或缺失时才回退精确 identity key”。

该优化保持 WAL 字典引用语义不变，并显著降低写入路径的 WAL 临时分配：write pprof 中 `wal.batchIdentities` alloc_space 从 `21.68MB` 降到 `9.88MB`，`wal.identityKeyWithScratch` 不再进入 write alloc_objects top。

## 测试口径

- 时间：2026-06-18 15:56-16:02 CST
- 平台：Fedora 44，linux/amd64
- workload：`points=100000`、`series=1000`、`field-layout=wide10`
- 字段：5 个 float64、3 个 int64、1 个 string、1 个 bool
- 写入批量：`write-batch-size=1024`
- MemTable：`memtable-max-samples=1200000`
- SSTable：`flush-on-exit`，最终 `sstable_count=1`
- 压缩：`compression_algorithm=off`
- 说明：query 用例包含预写入、flush 和 20 次 query，因此 query pprof 的主要热点仍包含写入准备成本。

## 执行命令

```bash
rm -rf /tmp/mts-pprof-analysis
mkdir -p /tmp/mts-pprof-analysis
go build -o /tmp/mts-pprof-analysis/storage_engine ./tests/pprof/storage_engine
```

baseline 使用 `git archive HEAD` 导出未改动源码后构建：

```bash
git archive HEAD | tar -x -C /tmp/mts-baseline-src
cd /tmp/mts-baseline-src
go build -o /tmp/mts-pprof-analysis/storage_engine_baseline ./tests/pprof/storage_engine
```

write/query 参数：

```bash
/tmp/mts-pprof-analysis/storage_engine -mode=write \
  -field-layout=wide10 -points=100000 -series=1000 \
  -memtable-max-samples=1200000 -write-batch-size=1024 \
  -flush-on-exit -query-repeat=20 \
  -data-dir=/tmp/mts-pprof-analysis/after-write-data \
  -cpu-profile=/tmp/mts-pprof-analysis/after-write.cpu.prof \
  -mem-profile=/tmp/mts-pprof-analysis/after-write.mem.prof
```

## 指标对比

| 场景 | 指标 | Baseline | After | 变化 |
| --- | --- | ---: | ---: | ---: |
| write | workload_duration_ms | 684 | 676 | -1.17% |
| write | heap_total_alloc_bytes | 827108520 | 810669408 | -1.99% |
| write | mallocs | 2260958 | 2158787 | -4.52% |
| write | rss_peak_bytes | 149102592 | 128946176 | -13.52% |
| write | data_dir_bytes | 8745503 | 8745500 | 基本持平 |
| query | workload_duration_ms | 848 | 840 | -0.94% |
| query | heap_total_alloc_bytes | 841855640 | 833220816 | -1.03% |
| query | mallocs | 2322381 | 2220686 | -4.38% |
| query | rss_peak_bytes | 168554496 | 168448000 | -0.06% |
| query | query_rows_streamed | 2000 | 2000 | 持平 |
| query | query_stats_samples | 20000 | 20000 | 持平 |
| query | query_stats_value_pages | 200 | 200 | 持平 |
| query | query_stats_parts | 20 | 20 | 持平 |

## write pprof 热点

### Baseline alloc_space top

- `main.wide10WorkloadPoint`: `137.11MB`
- `mts.toModelPoint`: `121.63MB`
- `memtable.growInt64s`: `96.55MB`
- `catalog.ResolvePoints`: `87.16MB`
- `memtable.materializeSortedColumnSamples`: `62.39MB`
- `memtable.growUint64s`: `62.04MB`
- `memtable.reserveBatchLocked`: `46.74MB flat / 247.37MB cum`
- `wal.batchIdentities`: `21.68MB flat / 23.68MB cum`
- `wal.encodeBatch`: `18.62MB flat / 51.89MB cum`

### After alloc_space top

- `main.wide10WorkloadPoint`: `135.12MB`
- `mts.toModelPoint`: `124.62MB`
- `catalog.ResolvePoints`: `84.01MB`
- `memtable.growInt64s`: `72.04MB`
- `memtable.materializeSortedColumnSamples`: `59.87MB`
- `memtable.growUint64s`: `52.53MB`
- `memtable.reserveBatchLocked`: `33.79MB flat / 198.40MB cum`
- `wal.batchIdentities`: `9.88MB flat / 13.43MB cum`
- `wal.encodeBatch`: `18.59MB flat / 43.06MB cum`

### CPU top 观察

write CPU 仍以哈希、GC scan、MemTable reserve/apply 和 syscall 为主：

- `aeshashbody`: `12.84%`
- `runtime.scanObject`: `15.60% cum`
- `memtable.columnBuffer.approxMemoryBytes`: `5.50%`
- `syscall6`: `5.50%`
- `catalog.resolveFieldsFromSchema`: `2.75%`
- `memtable.columnBuffer.reserve`: `7.34% cum`
- `wal.appendPoint`: `1.83%`

## query pprof 热点

query 用例仍包含预写入和 flush。After alloc_space top：

- `main.wide10WorkloadPoint`: `147.62MB`
- `mts.toModelPoint`: `114.62MB`
- `memtable.growInt64s`: `83.05MB`
- `catalog.ResolvePoints`: `82.70MB`
- `memtable.materializeSortedColumnSamples`: `64.40MB`
- `memtable.growUint64s`: `51.03MB`
- `memtable.reserveBatchLocked`: `31.65MB flat / 208.26MB cum`
- `wal.batchIdentities`: `13.96MB flat / 14.46MB cum`
- `queryexec.alignedRowsFromSeriesColumns`: `5.50MB`

query CPU top 仍由写入准备、MemTable reserve/materialize、哈希和 GC 组成；纯查询阶段的 `queryexec.alignedRowsFromSeriesColumns` 只在 alloc_space 中出现 `5.50MB`，不是本轮最主要瓶颈。

## 已实施优化

- `internal/wal.batchIdentities` 增加 SeriesID fast path。
- 连续 SeriesID 使用 dense slice 保存 `ref+1`，避免每个 batch 分配大 map bucket。
- 稀疏 SeriesID 自动回退 `map[uint64]int`，避免大跨度 SeriesID 造成超大 slice。
- SeriesID 缺失或同 SeriesID identity 不一致时，回退原有二进制 identity key，保证 WAL 字典引用正确性。
- 新增测试覆盖 SeriesID fast path、冲突 fallback、dense/sparse index 选择。

## 剩余优化空间

1. `main.wide10WorkloadPoint` 属于 pprof 造数成本，不是引擎核心，但会污染 profile。后续做纯引擎分析时应使用预生成 points 或单独的 read-only query profile。
2. `mts.toModelPoint` 和 `cloneStringMap` 仍是 public API 到内部模型转换的主要分配源。可评估批量写入 API 的 typed fast path，但需要谨慎处理外部输入所有权，不能破坏 API 安全边界。
3. `catalog.ResolvePoints` 和 `catalog.seriesKeyWithScratch` 仍有哈希与临时 key 成本。单 tag 已有 fast path，多 tag 场景仍可继续用 pprof 单独评估。
4. `memtable.reserveBatchLocked`、`growInt64s`、`growUint64s` 仍是主要引擎内部分配路径。当前峰值受 flush 物化和列扩容影响，后续可继续评估更精确的 batch 预估和 flush-time buffer 复用。

## 验证

已执行：

```bash
goimports-reviser -project-name github.com/openmts/mts -rm-unused -set-alias -format ./...
go test ./internal/wal ./internal/engine ./tests/pprof/storage_engine -count=1
go test ./... -count=1
go test ./... -coverprofile=/tmp/mts-coverage.out -covermode=atomic -count=1
golangci-lint run ./...
```

结果：

- `go test ./internal/wal ./internal/engine ./tests/pprof/storage_engine -count=1` 通过。
- `go test ./... -count=1` 通过。
- 覆盖率统计命令通过；全仓 total statements 为 `86.3%`，低于项目目标 `90%`，本轮修改包 `internal/wal` 为 `93.1%`。`/tmp/mts-coverage.out` 已删除。
- `golangci-lint run ./...` 通过，输出 `0 issues.`。
- `tests/e2e` 下 12 个实际用例均已单独 build/run 通过：`compaction_integrity`、`flush_manifest_recovery`、`format_governance`、`no_json_storage`、`query_aggregate_window`、`query_pruning`、`read_amplification`、`retention`、`service_ops`、`simple_integrity`、`streaming_query`、`wal_recovery`。
- 已清理 `/tmp/mts-pprof-analysis`、`/tmp/mts-baseline-src` 和 e2e 构建二进制，未发现 profile 或测试二进制残留。

本报告为单次 pprof smoke 对比，用于确认热路径和优化方向；不声明统计显著性。若要作为发布性能基线，应补充多轮 benchmark/benchstat 或固定 CPU/GOMAXPROCS 的重复采样。
