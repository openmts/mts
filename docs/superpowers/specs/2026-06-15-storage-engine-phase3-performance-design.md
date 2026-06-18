# mts Storage Engine Phase 3 Performance Design

## 背景

Phase 2 已经把存储层核心持久化路径切换为二进制格式，并通过 WAL、SSTable、Manifest、Catalog、query pruning、compaction、retention 的单元测试与 e2e 验证。当前功能基线可用于上层开发，但写入性能仍存在明显瓶颈：`Engine.Write` 内部逐点处理，`Shard.Write` 每个 point 单独写 WAL frame，Catalog 每点重复构造 series key 和 field 排序，MemTable flush 时会 clone 全量数据后再写 SSTable。

本阶段目标是优化千万级写入的稳定性。目标 workload 是每个 point 包含 10 个字段：5 个 `float64`、3 个 `int64`、1 个 `string`、1 个 `bool`。优化重点不是引入复杂异步架构，而是在现有一致性模型内降低每点固定开销、降低 flush 峰值内存、提供可复跑的 benchmark 和 pprof 证据。

## 范围

### 本阶段包含

- `Engine.Write` 内部从逐点写入改为按 shard 批量写入。
- `Shard` 增加 batch write 路径，一次 WAL append 覆盖同 shard 的多个 resolved point。
- `MemTable` 增加 batch apply，并将 `SnapshotAndReset` 改为 swap 当前 map，避免 flush 前 clone 全量数据。
- Catalog 增加批量 resolve 入口或热路径辅助，减少 batch 写入时重复锁切换。
- SSTable Part 打开时只常驻 metadata/metaindex，index rows 改为查询命中后懒加载，降低大量 Part 的常驻内存。
- Benchmark 增加 10 字段宽点写入场景，报告 `ns/op`、`B/op`、`allocs/op`。
- `tests/pprof/storage_engine` 增加可配置字段布局，支持 10 字段写入 workload。
- 更新性能基线文档，记录优化前后对比。

### 本阶段不包含

- 不重写 MemTable 为 append-only column buffer。
- 不引入异步 write queue、后台 writer 或跨 goroutine backpressure。
- 不改变公开数据模型，只继续支持 `float64`、`int64`、`string`、`bool`。
- 不引入外部压缩库。
- 不实现最终版 Gorilla/XOR/delta-of-delta 压缩。
- 不改变现有 WAL frame 和 SSTable v2 文件格式的兼容语义。

## EARS 需求

- When `Engine.Write` receives multiple points, the system shall normalize and resolve the points, group them by shard, and write each shard group through one batch path.
- When a shard receives a batch, the system shall append one WAL batch payload for the shard group instead of one WAL frame per point.
- When `WriteOptions.Sync=true`, the system shall fsync the WAL before returning from the batch write.
- When `WriteOptions.Sync=false`, the system shall keep honoring `WALOptions.Sync`, `BatchRecords`, and `BatchBytes`.
- When applying a batch to MemTable, the system shall update all points under one MemTable lock acquisition.
- When MemTable reaches `MemTableMaxSamples`, the system shall flush after applying the durable batch.
- When `SnapshotAndReset` is called, the system shall transfer ownership of the current table data to the snapshot and install a new empty table without cloning all samples.
- When querying MemTable while no flush is active, the system shall still return a snapshot that cannot be mutated by later writes.
- When query and flush read snapshot data, the system shall preserve Last Write Wins semantics by `writeSeq`.
- When writing the 10-field workload, the pprof tool shall generate points with 5 float fields, 3 int fields, 1 string field, and 1 bool field.
- When benchmarks run, the system shall include existing 4-field workload and a 10-field workload so regressions are visible.
- When a Part is opened after flush or restart, the system shall not keep full index rows resident unless a query needs them.
- When a query matches a Part by metadata/metaindex, the system shall load and decode the index block on demand and return corruption errors at query time.
- If any batch point fails validation or catalog resolution, the system shall return an error before writing that point to WAL or MemTable.
- If WAL append succeeds but MemTable apply fails, the system shall return an error and rely on WAL replay for durability, matching existing behavior.
- If flush fails after MemTable swap, the system shall not silently lose the swapped snapshot.

## 推荐架构

### Engine Batch Pipeline

`Engine.Write` 保留公开 API，但内部改为两阶段：

1. 在 Catalog 中批量 resolve points，并分配连续 `writeSeq`。
2. 在 Engine 锁内按 shard 聚合 `[]model.ResolvedPoint`，调用 `Shard.WriteBatch`。

批量 resolve 必须保证错误发生时尚未写入 WAL。为保持实现简单，Catalog 可以先提供 `ResolvePoints(points []model.Point) ([]model.ResolvedPoint, error)`，在一个 Catalog 锁内完成 validate、series resolve、field resolve 和字段排序。Engine 负责填充 `writeSeq`，避免 Catalog 依赖 Engine 状态。

### Shard Batch Write

新增 `Shard.WriteBatch(points []model.ResolvedPoint, syncWrite bool) error`：

- 空 batch 直接返回。
- 先调用 `wal.Append(points, syncWrite)`，保持 WAL-before-MemTable。
- 再调用 `mem.ApplyBatch(points)`。
- 如果 sample count 达到阈值，调用现有 `Flush`。

`Shard.Write` 保留为单点兼容包装，内部调用 `WriteBatch`。

### MemTable Swap Snapshot

当前 `SnapshotAndReset` 会 clone 全量 map，flush 高峰期会同时持有原 map 和 clone map。本阶段改为：

- 在锁内保存 `snapshotData := m.data`。
- 将 `m.data` 替换成新的空 map。
- snapshot 持有旧 map 所有权。

`Snapshot` 仍需要 clone，因为它用于查询当前 memtable，不应持有会被后续写入修改的 map。flush 路径只使用 `SnapshotAndReset`，因此可以安全 swap。

如果后续 `WritePart` 或 manifest 提交失败，需要把 snapshot 数据恢复到 memtable，或至少保持 WAL 不截断并允许重启恢复。本阶段为了运行期不丢可见数据，应在 `Shard.flushLocked` 失败时把 snapshot merge 回当前 memtable，并保持 WAL 不截断。

### 10 字段 Workload

pprof 工具增加 `-field-layout`：

- `default`：保留当前 4 字段布局。
- `wide10`：生成 5 个 float、3 个 int、1 个 string、1 个 bool。

benchmark 增加 `BenchmarkEngineWriteWideBatch`，使用固定 series 数和 10 字段 workload。benchmark 仍使用临时目录，避免留下构建或数据产物。

### SSTable Lazy Index

`OpenPart` 不再解码完整 `index.bin`。Part 常驻 `metadata.bin` 与 `metaindex.bin`，用于 time、series、field 粗剪枝。只有 query 通过这些剪枝后，才读取 `index.bin` 并解码 index rows。这样写入大量小 Part 时，常驻内存不再随所有 Part 的完整 index rows 线性增长。坏 index block 的错误从 open 阶段移动到 query 阶段。

## 测试策略

### TDD 用例

- `internal/engine`: 批量写入同 shard 时 WAL 文件中只出现一个完整 write batch frame。
- `internal/engine`: 批量写入跨 shard 时每个 shard 写一个 WAL batch，查询结果正确。
- `internal/memtable`: `ApplyBatch` 与逐点 `Apply` 结果一致。
- `internal/memtable`: `SnapshotAndReset` 后旧 snapshot 可查询，新 memtable 为空，后续写入不修改旧 snapshot。
- `internal/engine`: flush 写 Part 失败时，未 flush 数据仍可查询，WAL 未被截断。
- `tests/pprof/storage_engine`: `-field-layout=wide10` 生成 10 字段并能完成 write/query smoke。
- `internal/bench`: 10 字段 benchmark 编译并可运行。
- `internal/sstable`: 坏 index block 在 `OpenPart` 阶段不报错，在 query 命中时返回错误。

### 验证命令

- `go test ./... -coverprofile=coverage.out -timeout 600s`
- `go tool cover -func=coverage.out | tail -1`
- `go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s`
- `go run ./tests/pprof/storage_engine -mode=write -field-layout=wide10 -points 100000 -series 1000 -mem-profile /tmp/mts-wide10-mem.prof`
- `goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .`
- `gofmt -w` on touched Go files
- `golangci-lint run --timeout 12m`
- 按项目规则运行 `tests/e2e` 下每个目录的 build/run，并清理二进制产物。

## 性能验收

- 10K wide10 写入 benchmark 的 `allocs/op` 应低于逐点 WAL 版本的基线。
- pprof wide10 写入 workload 能在内存稳定的情况下完成 100K smoke。
- 对千万级目标，本阶段提供流式入口和低分配基础，不在单次 CI 中强制写 10M；需要人工或专用环境运行 `tests/pprof/storage_engine -points 10000000 -field-layout=wide10`。
- 所有功能性测试与 e2e 通过，总覆盖率保持 `>=90%`。

## 风险与权衡

- 批量 resolve 会扩大 Catalog 锁持有时间，但减少每点锁切换和排序分配；对写入批处理更有利。
- `SnapshotAndReset` swap 能显著降低 flush 峰值内存，但失败恢复必须显式处理，避免运行期可见数据丢失。
- 本阶段不做 append-only MemTable，因此 map 层级分配仍存在；这是 Phase 4 的优化空间。
- 不引入异步 writer，吞吐上限低于后台队列模型，但一致性、错误传播和测试复杂度更可控。

## 退出条件

完成本阶段后，存储层应具备低分配批量写入路径、10 字段宽点性能压测入口和更新后的 benchmark 基线。若测试或 benchmark 显示批量路径没有降低分配，应停止继续扩展并回到 profile 分析。
