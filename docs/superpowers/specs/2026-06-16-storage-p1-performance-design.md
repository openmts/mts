# Storage P1 Performance Design

## 背景

当前 Phase 12-18 与上一轮 hotpath allocation 任务已经完成。新的 P1 性能检视指出三条仍值得一次性优化的路径：

1. Compaction 仍以 batch 为单位构造 `[]ColumnData`，未做到按 series 逐个释放。
2. SSTable value page 命中后仍分配 `timestamps`、`writeSeqs`、`samples` 多段切片。
3. WAL batch v2 每个 point 重复写入 database、retention policy、measurement、tags 与 field name，存在写放大。

Prometheus 本地 TSDB 与 VictoriaMetrics 都强调 block/part 级顺序数据、WAL 与持久层格式分离、按序列或时间窗口组织读写。本轮设计沿用这些原则：保持旧格式可读，新增更紧密的内部路径，先降低内存峰值与写放大，再用 pprof/benchmark 复测。

## EARS 需求

- 当执行 compaction 时，系统应按 seriesID 逐个拉取输入 part 的列、归并、应用 tombstone 并写入输出 part，从而避免 batch 级 `[]ColumnData` 常驻。
- 当 compaction 输入 part 数量较小且 index rows 总量不超过缓存阈值时，系统应允许复用已解码 index rows；否则应走 index streaming 路径，从而在 CPU 与 RSS 之间保持可控权衡。
- 当读取 SSTable value page 时，系统应将命中样本直接 append 到目标 `[]VersionedSample`，从而避免 `timestamps`、`writeSeqs`、`samples` 三段中间数组同时存在。
- 当读取旧 v2/v3/v5 value block 时，系统应保持向后兼容，且损坏 payload 仍返回明确错误。
- 当写入 WAL batch 时，系统应使用 v3 字典化格式：batch 级 identity table 与 field name table 去重，point 记录只写引用、时间、序列号和值。
- 当 replay WAL 时，系统应同时支持 v2 与 v3 batch，并返回与旧格式一致的 `model.ResolvedPoint`。
- 当遇到空 batch、字段类型冲突、非法引用或截断 payload 时，系统应返回错误，不得静默丢数据。

## 设计方案

### Compaction Per-Series Streaming

新增 `queryCompactionSeries`，每次只查询一个 seriesID。调用方仍先收集全局有序 seriesIDs，但输出循环从 batch 改为 per-series。每个 series 的临时列集合在 `output.addSeries` 返回后即可释放。这样保留现有 `mergeColumnData`、`applyTombstones` 和 `PartWriter` 行为，降低行为风险。

`SeriesBatchReader` 已缓存小规模 index rows。对 reader 路径增加 `QuerySeriesID(seriesID uint64)`，避免每个 series 构造 `[]uint64`。未缓存路径使用 `Part.QuerySeriesIDs(query, []uint64{seriesID})`，后续可继续优化为真正 single-series stream。

### SSTable Value Streaming Decoder

新增 `sampleAppender` 与 `readSamplesInto`。v3 plain page 解码时先读取 writeSeqs，再按字段类型直接 append 样本。aligned time refs 复用 row timestamps；indexed time refs 暂保留 `[]int64`，但去掉额外 values slice 和 buildValueBlock 的 samples 复制。v2 和 v5 保持兼容，压缩 v5 后续可单独扩展到完全 streaming。

bool 解码新增按 bit 读取的直接路径，避免 `[]bool -> []VersionedSample` 二次转换。

### WAL v3 Dictionary Batch

保留 WAL frame record type 不变，只升级 batch payload version：v2 继续可读，新增 v3 为默认写入格式。

v3 payload 结构：

```text
version=3
identity_count
identity: database, retention_policy, measurement, sorted_tags
field_name_count
field_name: string
point_count
point: identity_ref, series_id, timestamp, write_seq, field_count
field: field_id, field_name_ref, field_type, value
```

identity key 使用现有排序 tag 编码，保证相同 tags 在 batch 内去重。field name table 只按名称去重，不按 fieldID 去重，因为同名字段应在同 measurement 下保持 schema 一致；回放时仍恢复每个 field 的 FieldID 与 Type。

## 验证策略

- `internal/engine`：新增 compaction 单 series 输出等价测试，覆盖 tombstone、跨 part 重复 timestamp LWW、输出 part 查询正确。
- `internal/sstable`：新增 v3 value block streaming allocation 测试，证明不再分配 typed values slice；覆盖 bool 直接解码。
- `internal/wal`：新增 v3 WAL batch 小于 v2 的体积测试、v2/v3 兼容 replay 测试、坏引用错误测试。
- 定向验证：`go test -count=1 ./internal/wal ./internal/sstable ./internal/engine -timeout 180s`。
- 全量验证：`go test -count=1 ./... -coverprofile=coverage.out -timeout 600s`，覆盖率不低于 90%。
- 性能 smoke：使用 `tests/pprof/storage_engine` 的 100K wide10 write/query/compact 进行前后对比，重点关注 RSS、alloc_space、data_dir_bytes 与耗时。

## 风险与约束

- WAL v3 是持久化格式变更，必须保留 v2 解码路径。
- Compaction per-series 会增加 index 查询次数；小 part 使用 `SeriesBatchReader` 缓存减轻 CPU，后续可再做 single-series index seek。
- SSTable v5 compressed page 目前仍走旧解码组合路径，本轮先优化默认未压缩 v3 page 和 page index 命中后的主要路径，避免同时改压缩编解码。
- 本轮不引入大规模对象池，避免 RSS 常驻量反弹。
