# 代码检视 / 实施报告：P2 冷层 page 与布局固定税

- 日期：2026-07-18 21:00
- 范围：`internal/engine` 分层压缩、`internal/sstable` zstd、scale shard 默认
- 状态：**已完成并验证**

## 1. 变更

| 项 | 实现 |
|---|---|
| zstd 强度 | `SpeedFastest` → `SpeedDefault` |
| L1+ value page | 默认抬高到 **16384**（保留更大显式配置） |
| scale shard | 默认 `24h` → **7d**，降低 10M 跨度固定税 |
| 层级继承 | 显式层级压缩时 L1+ 同样抬高 page |

## 2. 10M 体积

条件：`zstd + value-page-samples=4096 + omit-write-seq + typed + compact`（L1 compact 会抬到 16384）。

| 阶段 | data_bytes | MiB | shards |
|---|---:|---:|---:|
| float const-step 后 | 79,425,428 | 75.8 | 116 |
| **本轮 P2** | **66,804,649** | **63.7** | **17** |
| VM 参考 | ~10M | ~10 | - |

相对上轮约 **-16%**；相对 P0（~479MiB）约 **-87%**；相对 VM 约 **6.4x**。

性能：
- write_throughput ≈ 377k pts/s
- duration ≈ 46s（compact 更重，约 19s）
- query_latency ≈ 111ms（大页/少 shard 读路径变化）
- 200k `verify=true` 通过

## 3. 验证

- `make test` 通过
- `make e2e` 通过
- `make lint` 0 issues
- 临时数据已清理

## 4. 后续

- P2-2 part 多文件组件合并
- 更高 zstd level（如 BetterCompression）可配置
- 继续降低 catalog/index 固定税
