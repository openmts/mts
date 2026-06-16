# Storage Engine Next Phases Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Phase 2-11 后剩余的存储层缺口拆成可独立验收的后续阶段。

**Architecture:** 先降低查询常驻内存和锁占用，再增强 SSTable block 内剪枝和 typed compression，随后补齐 compaction、WAL/delete、metadata API 和性能治理。每个阶段保持单一主目标，避免同时改 API、磁盘格式和后台调度。

**Tech Stack:** Go 1.26.2、标准库、现有 `internal/engine`、`internal/sstable`、`internal/wal`、`internal/catalog`、`tests/e2e`、`tests/pprof`。

---

## Phase 12: Query Iterator And Lock Scope

**缺口：** 原始设计要求 iterator，当前公开 API 返回完整 slice；`queryColumnData` 查询期间持有 Engine 全局锁；`QueryRows` 使用 map materialize 全量结果。

**目标：** 增加内部流式 query 基础设施，缩短 Engine 锁持有时间，并保留现有 slice API 兼容。

**Files:**
- Modify: `types.go`
- Modify: `engine.go`
- Modify: `internal/engine/query.go`
- Modify: `internal/engine/engine.go`
- Test: `internal/engine/engine_test.go`
- Test: `tests/e2e/query_pruning/main.go`

- [x] **Task 12.1: 增加 query 快照测试**

  验收：长查询只在开始阶段复制 shard 指针和 catalog 元数据；查询执行期间新写入不会 panic，也不会破坏本次查询结果。

  Run:

  ```bash
  go test -count=1 ./internal/engine -run 'TestQuerySnapshot|TestQueryRows' -timeout 180s
  ```

  实现备注：新增 catalog snapshot 与 query shard snapshot，查询开始阶段复制元数据和 shard 指针；已通过 `go test -count=1 . ./internal/catalog ./internal/engine -timeout 180s`。

- [x] **Task 12.2: 缩短 Engine 锁范围**

  实现方向：
  - 在 `QueryColumns` 中先复制符合 database/rp/time 的 shard 指针。
  - catalog series/field 装饰需要使用快照或只读复制，避免每列重复加锁。
  - shard 查询仍由 shard 自身生命周期锁保护。

  验收：现有 `QueryColumns` / `QueryRows` 返回结果不变。

  实现备注：`queryShards` 缩短 Engine 全局锁范围，Shard 查询改用生命周期读锁保护 memtable/part 快照。

- [x] **Task 12.3: 增加内部 ColumnIterator**

  实现方向：
  - 先新增内部 iterator，不直接破坏公开 API。
  - 现有 `QueryColumns` 基于 iterator 收集 slice。
  - iterator 必须支持 `Close()`，后续能释放 Part 读取状态。

  验收：小查询和空查询行为与当前 slice API 一致。

  实现备注：新增 public `QueryColumnIterator`/`QueryRowIterator` 与内部 iterator；旧 slice API 基于 iterator 收集，保持兼容。

- [x] **Task 12.4: 优化 Row 聚合**

  实现方向：
  - 对已按 `(seriesID, fieldID, timestamp)` 排序的列，优先线性构建 rows。
  - 只有发现跨字段乱序时才退回 map 聚合。

  验收：`QueryRows` 在 100K wide10 query pprof 中 `columnsToRows` 不进入 alloc_objects top 20。

  实现备注：同 series 多字段 timestamp 对齐时走线性聚合快路径，不对齐才回退 map 聚合。

## Phase 13: SSTable Value Block Page Index

**缺口：** 当前可以按 Part/metaindex/index/field 剪枝，但命中字段后仍读取整块 value payload。

**目标：** 在 value block 内增加 page index，支持按时间范围只读部分 page。

**Files:**
- Modify: `internal/sstable/types.go`
- Modify: `internal/sstable/encoding.go`
- Modify: `internal/sstable/read.go`
- Modify: `internal/sstable/write.go`
- Test: `internal/sstable/encoding_test.go`
- Test: `internal/sstable/sstable_test.go`
- Test: `tests/e2e/query_pruning/main.go`

- [x] **Task 13.1: 设计 v4 value block**

  约束：
  - v3 仍可读取。
  - v4 page index 记录 page min/max timestamp、ordinal range、payload offset/size。
  - page 大小通过常量起步，默认目标 `32KB-128KB` uncompressed payload。

  实现备注：v4 value ref 指向 page index block，page index 记录 page min/max time 与独立 value page block ref；v2/v3 读取兼容保留。

- [x] **Task 13.2: 写失败测试**

  验收：
  - 查询小时间范围只读取命中 page。
  - page CRC 或长度损坏返回错误。
  - v3 part 兼容读取。

  实现备注：新增 page index round-trip、畸形 payload 拒绝、命中页读取统计测试。

- [x] **Task 13.3: 实现 v4 写入和读取**

  实现方向：
  - time block 保持 row-level。
  - value page 保存 writeSeq 和 typed values。
  - page index 放在 value block 头部或尾部，读取时先解析 index 再按 query range 读取 page。

  实现备注：`WritePart` 默认写 v4 page index，读路径先解析 index，只读取时间范围命中的 page，并按命中 page 估算结果容量。

- [x] **Task 13.4: pprof 验证**

  Run:

  ```bash
  go build -o /tmp/mts-pprof-run/storage_engine_phase13 ./tests/pprof/storage_engine
  /tmp/mts-pprof-run/storage_engine_phase13 -mode=query -field-layout=wide10 -points=100000 -series=1000 -query-repeat=20 \
    -cpu-profile=/tmp/mts-pprof-run/query_phase13_cpu.prof \
    -mem-profile=/tmp/mts-pprof-run/query_phase13_heap.prof
  ```

  验收：窄时间范围查询的 `sstable.readFloatSamples` / `readWriteSeqs` 分配随命中 page 数增长，而不是随整块样本数增长。

  实现备注：100K wide10 query pprof 使用 `-prebuild-points` 复跑，heap profile 中 `main.wide10WorkloadPoint` 未进入 `alloc_space` / `alloc_objects` 前 20；`readValuePages`、`readFloatSamples`、`readWriteSeqs` 只在命中 page 后出现分配，验证 page 级剪枝生效。

## Phase 14: Typed Compression Policy

**缺口：** 原始设计的 delta-of-delta、Gorilla/XOR、int bitpacking、string dictionary 仍未实现；`Options` 没有 compression 配置。

**目标：** 增加可配置压缩策略，并先在 SSTable v4 page 级别启用。

**Files:**
- Modify: `internal/model/types.go`
- Modify: `types.go`
- Modify: `internal/sstable/encoding.go`
- Create: `internal/sstable/compression.go`
- Test: `internal/sstable/compression_test.go`
- Test: `internal/sstable/encoding_test.go`

- [x] **Task 14.1: 增加 CompressionOptions**

  建议字段：
  - `Enabled bool`
  - `Timestamp string`
  - `Float string`
  - `Int string`
  - `String string`
  - `MinPageValues int`

  默认保持当前编码，避免无配置时改变行为。

  实现备注：新增 `model.CompressionOptions` 并通过根包 alias 暴露；Shard 将配置传递给 SSTable `WritePartWithOptions`，默认 `Enabled=false` 时仍写未压缩 v3 page payload。

- [x] **Task 14.2: 实现 timestamp delta-of-delta**

  验收：等间隔 timestamp 的 payload 小于当前 delta 编码，乱序 timestamp 拒绝或回退。

  实现备注：v5 compressed page timestamp 支持 delta-of-delta + zero-run，规则间隔序列小于 plain delta。

- [x] **Task 14.3: 实现 float XOR 编码**

  验收：平滑 float 序列 payload 小于 plain float64；随机 float 可回退 plain，避免 CPU 换不到空间。

  实现备注：float64 支持 XOR 编码，只有候选 payload 小于 plain float64 时启用，否则自动回退 plain。

- [x] **Task 14.4: 实现 int delta/zigzag 编码**

  验收：递增 int64 序列 payload 小于 plain varint/value encoding。

  实现备注：int64 支持 delta + zigzag 编码，适合大基数递增序列；随机或收益不足时回退 plain。

- [x] **Task 14.5: 实现 string page-local dictionary**

  验收：重复字符串序列 payload 小于 length-prefixed plain string；高基数字符串可回退 plain。

  实现备注：string 支持 page-local dictionary + ordinal 编码，重复字符串序列小于 length-prefixed plain；已通过 `go test -count=1 ./internal/sstable ./internal/engine -timeout 180s`。

## Phase 15: Compaction Scheduler And Output Splitting

**缺口：** 当前 compaction 是同步 size-tiered 起步实现；`BackgroundInterval` 和 `MaxOutputPartBytes` 没有完整产品化；输出不会按目标大小拆分。

**目标：** 增加后台 compaction 调度、输出 part 拆分和 orphan part 维护。

**Files:**
- Modify: `internal/engine/engine.go`
- Modify: `internal/engine/lifecycle.go`
- Modify: `internal/engine/shard.go`
- Test: `internal/engine/engine_test.go`
- Test: `tests/e2e/compaction_integrity/main.go`

- [x] **Task 15.1: 后台 compaction lifecycle**

  验收：
  - `Compaction.Enabled=true` 且 `BackgroundInterval>0` 时启动后台循环。
  - `Engine.Close` 能停止循环并等待退出。
  - 重复 `Close` 不 panic。

  实现备注：`Compaction.Enabled=true` 且 `BackgroundInterval>0` 时启动后台 ticker；`Engine.Close` 先停止后台 goroutine 并等待退出，重复 Close 保持安全。

- [x] **Task 15.2: 输出 part 拆分**

  验收：
  - `MaxOutputPartBytes` 生效。
  - 单次 compaction 可输出多个 level+1 part。
  - 查询结果与单 part 输出一致。

  实现备注：compaction 按 `MaxOutputPartBytes` 估算列大小并分组输出多个 level+1 part，manifest 原子切换后再关闭并移除旧候选 part。

- [x] **Task 15.3: orphan part 清理**

  验收：
  - manifest 未引用的 part 不暴露。
  - 清理失败不影响引擎打开，但会返回可观测错误或记录到维护结果。

  实现备注：打开 shard 时清理 manifest 未引用的 `sst-*` 目录；失败记录到 shard maintenance error，并通过 `Engine.MaintenanceErrors` 暴露。已通过 `go test -count=1 ./internal/engine -timeout 180s`。

## Phase 16: WAL Durability, Checkpoint, Tombstone

**缺口：** WAL 没有 interval 型批量 fsync，没有 segment-level checkpoint；delete/tombstone 仍是原始设计预留。

**目标：** 补齐 WAL 耐久策略和删除记录，为 retention 之外的删除能力打基础。

**Files:**
- Modify: `internal/model/types.go`
- Modify: `internal/wal/wal.go`
- Modify: `internal/wal/encoding.go`
- Modify: `internal/engine/engine.go`
- Test: `internal/wal/wal_test.go`
- Test: `tests/e2e/wal_recovery/main.go`

- [x] **Task 16.1: BatchFsync interval**

  增加 `WALOptions.BatchInterval time.Duration`。

  验收：
  - 未到 records/bytes 阈值时，超过 interval 会 fsync。
  - `WriteOptions.Sync=true` 仍强制同步。

  实现备注：`WALOptions.BatchInterval` 启动后台 ticker，对 pending WAL 执行有锁 fsync；强制同步与 records/bytes 阈值仍沿用原同步路径。

- [x] **Task 16.2: segment checkpoint**

  验收：
  - flush 后只删除已 checkpoint 的旧 segment。
  - 当前活跃 segment 不被误删。
  - replay 从剩余 segment 恢复完整数据。

  实现备注：新增 `Log.Checkpoint`，flush 后滚动到新活跃 segment，再删除已 checkpoint 的旧 segment；保留 `TruncateAll` 兼容测试和显式清空场景。

- [x] **Task 16.3: delete tombstone record**

  先实现内部 delete range，不暴露公开 API。

  验收：
  - tombstone WAL replay 后仍生效。
  - query 过滤被删除时间范围。
  - compaction 后 tombstone 可折叠进输出 part 或保留 tombstone sidecar。

  实现备注：新增内部 `model.Tombstone`、WAL tombstone record、`Shard.DeleteRange`；查询过滤 tombstone，WAL replay 后仍生效，compaction 会折叠 tombstone 并 checkpoint。已通过 `go test -count=1 ./internal/wal ./internal/engine -timeout 180s`。

## Phase 17: Metadata Management API

**缺口：** 当前 database、measurement、retention policy 主要通过写入隐式创建，缺少显式 metadata 管理。

**目标：** 在进入服务层前补齐 metadata 管理 API。

**Files:**
- Modify: `types.go`
- Modify: `engine.go`
- Modify: `internal/catalog/catalog.go`
- Modify: `internal/catalog/persist.go`
- Test: `internal/catalog/catalog_test.go`
- Test: `tests/e2e/simple_integrity/main.go`

- [x] **Task 17.1: database/rp API**

  建议 API：
  - `CreateDatabase(ctx, name string) error`
  - `DropDatabase(ctx, name string) error`
  - `CreateRetentionPolicy(ctx, database string, policy RetentionPolicy) error`
  - `ListRetentionPolicies(ctx, database string) ([]RetentionPolicy, error)`

  实现备注：根包与 internal engine 暴露 database/RP API；Catalog 使用独立 `metadata.bin` 二进制文件持久化显式 database 与 retention policy，写入路径也会隐式创建。

- [x] **Task 17.2: schema inspection API**

  建议 API：
  - `ListMeasurements(ctx, database string) ([]string, error)`
  - `ListFields(ctx, database, measurement string) ([]FieldSchema, error)`
  - `ListSeries(ctx, database, measurement string, tags map[string]string) ([]Series, error)`

  实现备注：新增 `ListMeasurements`、`ListFields`、`ListSeries`，基于现有 series/field 索引返回稳定排序结果。

- [x] **Task 17.3: metadata persistence**

  验收：重启后 database/rp/schema 仍可查询；删除 database 后旧 shard 不再暴露。

  实现备注：Catalog metadata 重启后可查询；`DropDatabase` 会关闭并移除对应 shard 数据目录，旧 shard 不再参与查询。已通过 `go test -count=1 ./internal/catalog ./internal/engine -timeout 180s`。

## Phase 18: Benchmark And Pprof Governance

**缺口：** Phase 6-11 多为 benchmark 文档，缺少统一统计比较；pprof workload 会混入造数成本。

**目标：** 让性能优化可复跑、可比较、可防回退。

**Files:**
- Modify: `tests/pprof/storage_engine/main.go`
- Modify: `tests/pprof/storage_engine/main_test.go`
- Modify: `internal/bench/storage_bench_test.go`
- Create: `docs/benchmarks/storage-engine-benchmark-guide.md`

- [x] **Task 18.1: pprof 预生成 points 模式**

  增加 `-prebuild-points`，将造数排除在 CPU profile 主阶段之外。

  验收：profile 中 `main.wide10WorkloadPoint` 不进入 alloc_space top 20。

  实现备注：`tests/pprof/storage_engine` 新增 `-prebuild-points`，在 CPU profile 启动前构建 workload points，写入/查询/compact 路径复用预生成 slice；预生成阶段临时关闭 `runtime.MemProfileRate`，避免 heap profile 把造数分配计入主阶段热点。

- [x] **Task 18.2: benchstat 工作流**

  文档化：
  - `go test -bench ... -count=10`
  - `benchstat old.txt new.txt`
  - 没有统计显著性时不得声称性能提升。

  实现备注：新增 `docs/benchmarks/storage-engine-benchmark-guide.md`，记录 pprof、benchmark、benchstat 的复跑方法和显著性要求。

- [x] **Task 18.3: performance gate**

  先做本地脚本，不直接接 CI：
  - 运行 10K default/wide10 benchmark。
  - 输出 sec/op、B/op、allocs/op。
  - 对比上一份基线文件。

  实现备注：新增 `scripts/storage_benchmark_gate.sh`，运行 10K default/wide10 benchmark，存在基线时自动执行 `benchstat`。已通过 `go test -count=1 ./tests/pprof/storage_engine ./internal/bench -timeout 180s`。

## 统一完成门禁

每个 phase 完成时必须执行：

```bash
goimports-reviser -project-name codeberg.org/mts/mts -recursive -format -rm-unused .
go test -count=1 ./... -coverprofile=coverage.out -timeout 600s
go tool cover -func=coverage.out | tail -1
golangci-lint run --timeout 12m
```

覆盖率验收：总覆盖率 `>=90.0%`。

涉及功能行为的 phase 还必须逐个执行：

```bash
for dir in tests/e2e/*; do
  [ -d "$dir" ] || continue
  bin=$(basename "$dir")
  (cd "$dir" && go build -o "$bin" . && "./$bin")
  rm -f "$dir/$bin"
done
```

完成后清理：

```bash
rm -f coverage.out
find tests/e2e -maxdepth 2 -type f -perm /111 -print
find . -maxdepth 4 \( -name "*.prof" -o -name "coverage.out" \) -print
```
