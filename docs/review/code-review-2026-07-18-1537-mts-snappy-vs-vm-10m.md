# MTS snappy 压缩对齐后 vs VictoriaMetrics 10M 复测

- 日期：2026-07-18 15:37 
- 目标：基于上一轮检视报告，将 MTS 压缩从 `off` 拉齐为 `snappy` 后复测
- 规模：10,000,000 逻辑点 / 10 字段 / 100 series / batch=4096
- 数据模型：与 `tests/scale/storage_10m` 一致

## 测试矩阵

| 引擎 | 压缩 | 写路径 | 查询 |
|---|---|---|---|
| MTS（上轮） | off | WriteTypedBatch | 中间窗口 2000 整行 |
| MTS（本轮） | **snappy** | WriteTypedBatch | 中间窗口 2000 整行 |
| VictoriaMetrics（本轮） | 内置压缩 | HTTP Influx line | 单字段 export + 10 字段 export |

## 一、MTS 开启 snappy 前后对比

| 指标 | 上轮 compression=off | 本轮 compression=snappy | 变化 |
|---|---:|---:|---:|
| 写入耗时 | 20.863s | 25.024s | +19.9% |
| 写入吞吐 | 479316 pts/s | 399614 pts/s | -16.6% |
| 冷查询 | 16.40ms | 16.23ms | -1.0% |
| 热查询 | 17.91ms | 17.00ms | -5.1% |
| 数据体积 | 1062.1 MiB | 553.3 MiB | -47.9% |
| Compact | 5.405s | 6.363s | +17.7% |
| RSS peak | 253.8 MiB | 254.5 MiB | +0.3% |

## 二、本轮 MTS(snappy) vs VictoriaMetrics

| 指标 | MTS snappy | VictoriaMetrics | 比值（MTS/VM） | 结论 |
|---|---:|---:|---:|---|
| 写入耗时 | 25.024s | 29.457s | 0.85x | MTS更快 |
| 写入吞吐 | 399614 pts/s | 339479 pts/s | 1.18x | MTS更高 |
| 冷查询 | 16.23ms（整行10字段） | 5.62ms（单字段） | 2.89x | VM更快 |
| 热查询 | 17.00ms | 2.81ms | 6.05x | VM更快 |
| 多字段查询 | 16.23ms（整行） | 24.87ms（10字段export） | 0.65x | MTS更快 |
| 数据体积 | 553.3 MiB | 10.2 MiB | 54.1x | VM仍大幅领先 |

## 三、结论

1. **压缩生效**：MTS 打开 snappy 后，磁盘从约 **1062 MiB → 553 MiB**，约减半（约 **48%**），说明压缩路径有效。
2. **写性能代价**：写入吞吐从约 **479k → 400k pts/s**，下降约 **16.6%**；compact 略增。
3. **查询基本持平**：冷/热查询仍在 ~16-17ms，压缩未明显拖慢本轮窗口查询。
4. **相对 VM**：
   - 写入：MTS 仍略快（约 **1.18x** 吞吐）。
   - 磁盘：相对 VM 从约 **114x** 缩小到约 **54x**，差距仍在数量级。
   - 查询：单字段仍是 VM 更快；多字段口径下 MTS 整行与 VM 10 字段 export 更接近。

## 四、边界说明

- MTS 本地 API vs VM HTTP Influx，写入对比仍含协议成本。
- MTS 查询是整行 10 字段；VM 主指标是单字段 export。
- 单机一次性压测，非长时间稳态。

## 五、清理

- 容器与 `/tmp/mts-vm-compare-compress` 测试数据在报告落盘后清理。
