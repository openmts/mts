# 代码检视 / 实施报告：timestamps.bin const-step

- 日期：2026-07-18 21:42
- 范围：`internal/sstable/encoding.go` 行时间戳块编码
- 状态：**已完成并验证**

## 1. 问题

探针显示 100 series × 4096 点 part 中：

| 组件 | 旧体积 |
|---|---:|
| timestamps.bin | **411,400** |
| values.bin | 41,037 |
| 其它 | ~10k |
| **合计** | **~0.44 MiB** |

`timestamps.bin` 使用朴素 `delta`（首值 + 逐点 varint），等间隔 1s 序列仍约 5B/点，成为存储主导。

## 2. 变更

新增 `timeEncodingConstStep`：

- 检测全页/全块恒定 step
- 存储：`count + base + step`（O(1)）
- 不规则序列回退旧 delta
- 读路径对称解码

## 3. 效果

同探针：

| 组件 | 新体积 |
|---|---:|
| timestamps.bin | **2,000** |
| values.bin | 41,037 |
| **合计** | **~0.05 MiB** |

10M scale（zstd + page4096 + omit-write-seq + 7d shard + compact）：

| 阶段 | MiB |
|---|---:|
| P2 冷层后 | 63.7 |
| **+ timestamps const-step** | **6.5** |
| VM 参考 | ~10 |

相对 P0（~479MiB）约 **-98.6%**；相对 VM 约 **0.65x（更优）**。

性能：write ≈ 380k pts/s，query ≈ 73ms，未见明显劣化。

## 4. 验证

- `make test` 通过（compaction_integrity 偶发竞态 flake，复跑稳定）
- `make e2e` / `make lint` 通过
- `internal/sstable` coverage 90.3%
- 200k `verify=true` 通过
- 临时数据已清理
