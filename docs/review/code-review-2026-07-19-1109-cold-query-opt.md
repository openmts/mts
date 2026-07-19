# 冷查询优化检视报告

- **日期**: 2026-07-19
- **范围**: SSTable 读路径窗口解码 + L1 冷层 page 策略
- **状态**: 已实现并通过全量门禁，10M 冷/热查询有提升，已提交

## 1. 背景

上一轮存储压缩对齐（const-step 时间戳、pack.bin、可配置 zstd）后，10M 数据体积已接近 VictoriaMetrics（约 6.50 MiB），但冷查询约 **80 ms**、热查询约 **68 ms**，明显慢于压缩前读路径。

根因判断：

1. 压缩 value page 查询时无条件物化整页 `timestamps`。
2. const-step 时间戳/数值在查询窗口外仍全页解码。
3. L1 冷层强制把 page size 抬到 16384，点查/小窗口放大 IO 与解码量。

## 2. 实现项

| ID | 改动 | 文件 | 状态 |
|---|---|---|---|
| Q1 | 压缩 page 懒加载行级 time block | `read_index.go`, `read.go` | 已完成 |
| Q2 | const-step 时间戳窗口物化 | `compression_time.go`, `query_window.go` | 已完成 |
| Q3 | 窗口化值解码快路径（const-step/delta/RLE） | `compression_values.go`, `compression_window.go` | 已完成 |
| Q4 | 有序时间戳二分窗口 | `encoding.go`, `query_window.go` | 已完成 |
| Q5 | L1 默认 page 16384→4096，尊重显式配置 | `paths.go`, `paths_test.go` | 已完成 |

### 关键决策

- Gorilla 编码依赖前缀状态，**不走窗口快路径**，仍走通用解码。
- 仅在 plain/page-index 真正需要行时间戳时调用 `ensureRowTimestamps`。
- scale 配置传 4096 时 L1 保持 4096，不再强行抬高到 16384。

## 3. 10M 前后对比

同配置：`zstd/default + page4096 + omit-write-seq + typed + compact`，10,000,000 points，查询窗口 2000 行整行。

| 指标 | 优化前（基线） | 优化后 | 变化 |
|---|---:|---:|---:|
| 冷查询 | 80.30 ms | 57.55 ms | **-28.3%** |
| 热查询 | 68.26 ms | 50.97 ms | **-25.3%** |
| 写吞吐 | 344,813 pts/s | 375,683 pts/s | +8.9%（含噪声） |
| data_bytes | 6.50 MiB | 6.50 MiB | 持平 |
| compact | ~16.1s | ~16.1s | 持平 |

- 基线：`docs/review/code-review-2026-07-19-0039-mts-rw-vs-history-and-vm.md`
- 优化后报告：`/tmp/mts-10m-queryopt-report.json`
  - `cold_query_latency_nanos=57546479`
  - `hot_query_latency_nanos=50968314`
  - `data_bytes=6815091`
  - `write_throughput≈375683`

**结论**：冷/热查询均有明显提升，数据体积无回退，符合提交条件。

## 4. 验证

- [x] `go test ./internal/sstable ./internal/engine`
- [x] `make test`（全包 ok，含 e2e/scale/fault）
- [x] `make e2e`（EXIT:0）
- [x] `make lint`（0 issues）

## 5. 残余差距与后续

相对 VM 仍慢的主要原因（本轮未做）：

1. MTS 整行 10 字段一次导出，VM 常按单字段/列扫描。
2. 无 block/page 缓存，冷热路径仍反复解码。
3. 多 part/row merge 与序列化开销仍在。

可选后续：

- page/block cache + 查询结果缓存
- 列式投影（只解码请求字段）
- 并行 part 扫描配额
- 更细粒度 index（min/max + sparse index）

## 6. 风险

- 窗口解码快路径覆盖编码组合有限，需依赖单测与 e2e 覆盖。
- L1 page 默认下调可能略增 part 内 page 数；本轮 10M 体积与 compact 时间持平，未见劣化。
