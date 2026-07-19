# 查询分配优化与读写 RSS 观测

- **日期**: 2026-07-19
- **范围**: 行/标签分配削减 + 页缓存策略 + 10M 分阶段 RSS
- **状态**: 已实现，10M 达标，全量门禁通过，已提交

## 1. 目标

1. 继续压缩查询路径分配（tags/fields clone、列装饰）
2. 调整 page decode cache，避免 compact 把 RSS 打爆
3. 用 10M write-query-compact 做冷/热查询与读写 RSS 观测

## 2. 实现

| ID | 改动 | 说明 |
|---|---|---|
| A1 | `decorateColumn` 共享 snapshot tags + 下标填充 samples | 10 字段不再重复 clone tags |
| A2 | `alignedRowsFromSeriesColumns` 预分配 + series tags 共享 | 2000 行减少 map clone |
| A3 | `cloneRow` 仅 clone Fields，tags 只读共享 | 保持 iterator 字段隔离语义 |
| A4 | page cache：`limit=256`，`maxSamples=512` | 查询窗结果可缓存；compact 大页不缓存 |
| A5 | storage_10m 分阶段 RSS：MaxRSS + VmRSS 当前值 | 写后/压实后/冷查后/热查后 |

## 3. 10M 对比（相对 `479e011`）

配置：`zstd + page4096 + omit-write-seq + typed + compact`，10M 点，2000 行整行。

| 指标 | 基线 `479e011` | 本轮 final | 变化 |
|---|---:|---:|---:|
| 冷查询 | 46.51 ms | **42.84 ms** | **-7.9%** |
| 热查询 | 41.26 ms | **41.79 ms** | +1.3%（噪声/持平） |
| 写吞吐 | 403,907 pts/s | 389,209 pts/s | -3.6%（噪声） |
| data_bytes | 6.55 MiB | 6.55 MiB | 持平 |
| process MaxRSS | ~1216 MiB（上轮量级） | **1216.2 MiB** | 持平 |

### 本轮分阶段内存（VmRSS 当前 / MaxRSS）

| 阶段 | 当前 RSS (VmRSS) | 进程 MaxRSS |
|---|---:|---:|
| 写后 | 290.5 MiB | 含写阶段峰值 |
| compact 后 | 471.5 MiB | **1216.2 MiB**（峰值出现在 compact） |
| 冷查询后 | 449.4 MiB | 1216.2 MiB（峰值未再抬升） |
| 热查询后 | 405.4 MiB | 1216.2 MiB |
| 结束 heap_alloc / heap_sys | 17.9 / 1191.0 MiB | GC=393 |

结论：

1. **读路径当前 RSS 约 0.4~0.5 GiB**，不是峰值来源。
2. **RSS 峰值由 compact 主导**（并行解压/重编码与 Go heap_sys 保留）。
3. 冷查询有稳定提升；热查询持平。

## 4. 缓存策略试错（未采用大缓存）

| 配置 | 冷查 | 热查 | MaxRSS | 结论 |
|---|---:|---:|---:|---|
| 无样本限制 + 大 limit | ~46 ms | **~27 ms** | **3.5~4.6 GiB** | 热快但 compact/缓存灌爆 |
| limit=256 + maxSamples=512 | **42.8 ms** | 41.8 ms | **1.22 GiB** | 达标，采用 |

## 5. 验证

- [x] `go test ./internal/sstable ./internal/engine ./internal/queryexec ./tests/scale/storage_10m`
- [x] `make test`
- [x] `make e2e`
- [x] `make lint`（0 issues）
- [x] 10M report：`/tmp/mts-10m-q3/report-v5.json`

## 6. 后续

1. 查询字段投影下推（单字段接近 VM）
2. compact 内存配额 / 流式复用，降低 MaxRSS
3. 可配置 query page cache（默认保守）
