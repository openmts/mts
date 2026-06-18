# mts Storage Engine Phase 4 MemTable Performance Design

## 背景

Phase 3 已经完成批量 resolve、同 shard 批量 WAL、MemTable swap flush 和 SSTable lazy index。100 万级 `wide10` 写入 smoke 可以完成，写入结束后的 in-use heap 已经稳定在较低水平，但 `alloc_space` profile 仍显示累计分配主要集中在 MemTable 多层 map、flush/query 重组和 SSTable 写入前的列分组。

当前 MemTable 使用 `map[seriesID]map[fieldID]map[timestamp]sampleEntry`。这个结构读写语义直接，但对 10 字段宽点写入不够紧凑：每个唯一 sample 都需要 timestamp map entry，写入时立即做 LWW merge，flush 时又要把 map 重新组装成列式 slice。Phase 4 的核心目标是把写入热路径改成更贴近列式存储的 append buffer，减少每个 sample 的 map 分配和 GC 压力。

## 开源实现参考

- Prometheus TSDB Head block 的写入路径先把样本追加到活跃 chunk，并通过 WAL 保证恢复；chunk 满或时间范围达到阈值后再进入后续持久化流程。它的启发是：热写路径应尽量顺序追加，压实和持久化放在边界阶段处理。
- InfluxDB TSM/Cache 模型同样把 WAL 与内存 cache 配合使用，再在 snapshot/compaction 阶段生成不可变文件。它的启发是：内存结构服务写入和 flush，不必在每次写入时完成最终存储形态。
- VictoriaMetrics 以不可变 part 和后台 merge 为核心，强调紧凑存储与低常驻内存。它的启发是：写入缓冲应避免过早构建过多细粒度索引，把排序、去重和合并放到批处理阶段更适合高吞吐。

## 方案对比

### 方案 A：保留三层 map，只增加池化

优点是改动很小，风险低，当前测试基本可复用。缺点是无法解决根因：timestamp map entry 仍然按样本增长，flush 仍要从 map 重建列式 slice，`alloc_space` 下降有限。这个方案适合短期微调，不适合作为下一阶段核心优化。

### 方案 B：MemTable 改为 append-only column buffer

按 `(seriesID, fieldID)` 建立列 buffer，写入时只追加 `VersionedSample`。LWW 去重从写入阶段移动到 query/flush 阶段。优点是热写路径少分配、cache locality 更好、flush 输入天然接近 SSTable 的列式格式。缺点是同 timestamp 覆写会让 `sampleCount` 按追加样本增长，可能比旧实现更早触发 flush；查询/flush 需要做一次列内压实。

### 方案 C：引入异步 writer 和后台 flush 队列

优点是能进一步提高前台写入吞吐，并为 backpressure 做准备。缺点是会引入并发队列、错误传播、关闭语义和 WAL/MemTable 可见性复杂度。当前 profile 的主要瓶颈还在单线程热路径分配，先改数据结构收益更直接。

推荐采用方案 B。它不改变公开 API，不改变 WAL/SSTable 二进制格式，改动边界集中在 MemTable 和 flush 输入路径。

## 范围

### 本阶段包含

- 将 MemTable 内部数据结构从三层 map 改为按列追加的 buffer。
- 保持 `Apply`、`ApplyBatch`、`Snapshot`、`SnapshotAndReset`、`Restore`、`Query` 的外部语义不变。
- `SnapshotAndReset` 继续采用 swap，flush 失败时 `Restore` 仍能恢复可见数据。
- query 和 flush 输出继续保持按 `SeriesID`、`FieldID` 排序，列内按 timestamp 升序。
- 同一 `(seriesID, fieldID, timestamp)` 多次写入时，查询与 flush 只返回 `WriteSeq` 最大的样本。
- 为 flush 增加列式输出入口，避免通过通用查询路径做不必要过滤。
- benchmark/pprof 记录 `wide10` 对比结果。

### 本阶段不包含

- 不改变 WAL frame 和 SSTable 文件格式。
- 不引入异步写队列、后台 writer 或跨 goroutine backpressure。
- 不改公开 `Engine.Write` / `Shard.WriteBatch` API。
- 不做最终压缩算法改造。
- 不把 Catalog/Engine 上层查询模型纳入本阶段。

## EARS 需求

- When MemTable applies a point, the system shall append each field sample to the column buffer identified by `(seriesID, fieldID)`.
- When MemTable applies a batch, the system shall append all field samples under one MemTable lock acquisition.
- When multiple samples share the same `(seriesID, fieldID, timestamp)`, the system shall return only the sample with the greatest `WriteSeq` from MemTable query.
- When a stale sample with lower `WriteSeq` is restored after a newer current sample, the system shall keep the newer sample visible.
- When `SnapshotAndReset` is called, the system shall transfer the current append buffers to the snapshot and install a new empty buffer map.
- When new writes arrive after `SnapshotAndReset`, the system shall not mutate the old snapshot.
- When `Snapshot` is called, the system shall return a defensive copy that is safe from later MemTable writes.
- When `Snapshot.Columns` is used for flush, the system shall emit LWW-compacted columns sorted by series, field and timestamp.
- When a query has series, field or time filters, the system shall apply filters before returning samples while preserving LWW semantics.
- If query time range is invalid, the system shall return an empty initialized slice.
- If snapshot is nil, the system shall return zero sample count and empty query results.
- If flush receives columns from a snapshot, the system shall not require any JSON/Gob/CSV intermediate representation.

## 设计

MemTable 新内部结构：

```go
type columnKey struct {
    seriesID uint64
    fieldID  uint32
}

type columnBuffer struct {
    seriesID  uint64
    fieldID   uint32
    fieldType model.FieldType
    samples   []model.VersionedSample
}

type tableData map[columnKey]*columnBuffer
```

写入路径只负责 `append`。`sampleCount` 统计追加样本数，而不是 LWW 后唯一样本数。这样实现简单且能控制内存阈值：同 timestamp 大量覆写会更早 flush，避免 stale sample 无限留在 MemTable。

快照路径分两类：

- `SnapshotAndReset`：swap map，不 clone，flush 独占旧 map。
- `Snapshot`：clone 每个列 buffer 的 sample slice，供在线查询使用。

输出路径统一通过 `Snapshot.Columns(query)` 实现。它按列过滤 series/field，再对命中的列做 LWW 压实和 timestamp 排序。`Snapshot.Query` 调用 `Columns` 保持兼容；`Shard.flushLocked` 调用无过滤的 `Columns`，语义更明确。

LWW 压实策略：

- 小列或存在重复 timestamp 时使用临时 `map[int64]VersionedSample`。
- 对每个 timestamp 保留 `WriteSeq` 最大的 sample。
- 输出 slice 按 timestamp 升序排序。

Restore 策略：

- 将 snapshot 中的列样本追加回当前 MemTable 对应列。
- `sampleCount` 按追加样本数增加。
- 如果当前 MemTable 已有更新写入，查询/flush 阶段 LWW 会保留更新样本。

## 测试策略

- `internal/memtable`: batch append 与逐点 apply 查询结果一致。
- `internal/memtable`: 同 timestamp 多 `WriteSeq` 写入后查询只返回最新值。
- `internal/memtable`: `SnapshotAndReset` 后旧 snapshot 稳定，新写入不污染旧 snapshot。
- `internal/memtable`: `Restore` 合并旧 snapshot 和当前数据时保留更大 `WriteSeq`。
- `internal/memtable`: `Snapshot.Columns` 输出已按 series/field/timestamp 排序并完成 LWW。
- `internal/engine`: flush 后 SSTable 查询能看到 LWW 后的最新值。
- `internal/bench`: `wide10` benchmark 记录 Phase 4 后结果。

## 验证命令

- `go test ./internal/memtable -timeout 180s`
- `go test ./internal/engine -run 'TestShard|TestEngine' -timeout 180s`
- `go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s`
- `timeout 1200s go run ./tests/pprof/storage_engine -mode=write -field-layout=wide10 -points 1000000 -series 10000 -mem-profile /tmp/mts-wide10-phase4-mem.prof`
- `go test ./... -coverprofile=coverage.out -timeout 600s`
- `go tool cover -func=coverage.out | tail -1`
- `goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .`
- `gofmt -w` on touched Go files
- `golangci-lint run --timeout 12m`
- `tests/e2e/*` 每个目录执行 `go build && ./binary`，并清理二进制产物。

## 性能验收

- `BenchmarkEngineWriteWideBatch/points=10000` 的 `allocs/op` 应低于 Phase 3 的约 `181.2k allocs/op`。
- `wide10` 100K 或 1M pprof smoke 的 in-use heap 保持稳定，不出现随写入总量线性增长的常驻内存。
- 总测试覆盖率保持 `>=90%`。

## 风险与权衡

- 查询路径会在 MemTable 中做 LWW 压实，短时间内大量重复 timestamp 查询可能比旧 map 慢；写入型 workload 下这是可接受交换。
- `sampleCount` 从唯一样本数变为追加样本数，会更早触发 flush；这有利于限制 stale sample 常驻内存。
- Restore 采用追加恢复会临时保留重复样本，语义由统一 LWW 输出保证；如果未来 flush 失败频繁，可再增加 Restore 阶段压实。
- SSTable 写入仍会做列分组和 timestamp 收集，Phase 4 先减少 MemTable 热路径分配，SSTable 写入优化作为后续独立阶段。
