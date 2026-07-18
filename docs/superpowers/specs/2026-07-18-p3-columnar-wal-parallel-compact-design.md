# P3-01 列式 WAL + 分区并行 Compact 设计（POC）

- **日期**：2026-07-18
- **阶段**：POC
- **兼容性**：**不考虑**旧 WAL/旧行为兼容；直接落地最终目标格式
- **非目标**：对象存储冷层、分布式、shard 内 series 级并行 merge

## 目标

1. WAL 写入/回放统一为 **列式帧**（series/time/seq/field 列布局）。
2. Compaction 在 **跨 shard（分区）** 维度有界并行，缩短多 shard wall-clock。
3. 用 `MaxConcurrentCompaction` + 既有 memory budget 控制 CPU/内存峰值。

## 资源影响（并行 Compact）

- 并发度 N 时，CPU/内存峰值约 **O(N × 单任务)**。
- POC 默认 `MaxConcurrentCompaction` 归一化为 `min(GOMAXPROCS, 4)`，至少为 1；可配置。
- 禁止无界 fan-out；同一 shard 仍单写者（lifecycleMu）。

## 列式 WAL 格式（最终）

- Segment magic 保持 `MTSWAL2`，**formatID = 2**（仅支持 2）。
- Record types：
  - `1` write columnar batch
  - `2` tombstone（保持）
- Payload：
  - identities 字典
  - field schema（name + type）
  - row_count
  - identity_ref[n], series_id[n]
  - timestamps delta-varint[n], write_seq delta-uvarint[n]
  - 每 field：dense values（按 schema type）

`Append` / `AppendTyped` 均编码为同一列式 payload；replay 解码为 `[]ResolvedPoint`。

## 并行 Compact

- `Engine.CompactWithResult` 快照 shard 列表后释放 `e.mu`，按全局 N 并行 `Shard.CompactWithResult`。
- 失败：等待 in-flight 结束后返回错误；已成功 shard 的 manifest 保持已提交结果。
- 指标：沿用 active/max_concurrent/skips。

## 验收

- 单测：列式 round-trip、截断、CRC、typed/points
- 多 shard 并行 compact 正确性
- make test / e2e / lint / race
- bench：WriteBatch 对比说明
