# 10M 降采样查询懒加载优化报告

## 结论

本轮将 SSTable `SeriesIDs` 查询路径从 eager 批量读取改为 lazy 流式读取。10M 源数据、100 series、目标降采样 166,700 行、`LIMIT 2000` 查询场景下，查询耗时从 140.876ms 降至 18.666ms，耗时下降 86.75%，速度提升 7.55 倍。

## 根因

- 查询计划会将 measurement 下匹配的 `SeriesIDs` 传入 SSTable，用于保证 measurement/tag 过滤正确。
- 原 `ScanColumns` 在 `SeriesIDs` 非空时调用 `querySeriesIndexRows`，创建 stream 阶段就把当前 part 中所有匹配 series 的 index row 和 value column 全部读出。
- `LIMIT 2000` 只能在上层 row stream 关闭时生效，无法阻止 SSTable 已经发生的超额预读。

## 优化

- 保留 `SeriesIDs` 过滤语义，避免误读其它 measurement。
- `ScanColumns` 的 `SeriesIDs` 路径改为按排序后的 seriesID 懒加载。
- 创建 stream 时不读取 index row；每次 `Next()` 只读取下一个匹配 series 的数据，使上层 `LIMIT` 关闭可以截断后续 series 读取。

## 性能对比

| 指标 | 优化前 | 优化后 | 变化 |
| --- | ---: | ---: | ---: |
| 查询耗时 | 140.876ms | 18.666ms | -86.75% |
| 查询吞吐 | 14,197 rows/s | 107,149 rows/s | +654.72% |
| 查询阶段 RSS peak | 114.20MiB | 56.91MiB | -50.17% |
| 总 RSS peak | 120.50MiB | 60.73MiB | -49.60% |
| 写入耗时 | 14.951s | 14.899s | 基本持平 |
| 降采样耗时 | 6.162s | 6.322s | 基本持平 |
| 磁盘空间 | 193.21MiB | 193.21MiB | 基本持平 |
| SSTable 数量 | 1,194 | 1,194 | 不变 |

## 本次命令

```bash
timeout 1200s go run ./tests/scale/downsample_policy -points 10000000 -series 100 -query-limit 2000 -batch-size 1024 -checkpoint-interval 1 -run-timeout 20m -out docs/benchmarks/downsample_10m_2026-06-21_query_lazy.json
```

## 优化后原始数据

```json
{
  "points": 10000000,
  "series": 100,
  "query_limit": 2000,
  "points_written": 166700,
  "query_rows": 2000,
  "duration_nanos": 21272118853,
  "write_duration_nanos": 14898861916,
  "downsample_duration_nanos": 6322376354,
  "query_duration_nanos": 18665620,
  "query_throughput_rows_per_second": 107148.86513279495,
  "rss_peak_bytes": 63684608,
  "write_rss_peak_bytes": 41897984,
  "downsample_rss_peak_bytes": 58855424,
  "query_rss_peak_bytes": 59670528,
  "disk_bytes": 202599699,
  "sstable_count": 1194,
  "verified": true
}
```
