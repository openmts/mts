# 10M 降采样查询性能报告

## 测试配置

- 命令：`go run ./tests/scale/downsample_policy -points 10000000 -series 100 -query-limit 2000 -batch-size 1024 -checkpoint-interval 1 -run-timeout 20m -out docs/benchmarks/downsample_10m_2026-06-21.json`
- 原始点数：10,000,000
- series 数：100
- 降采样 policy：1 个，`1m` 窗口，`avg/min/max/count/last`
- 查询：目标 retention policy `rp_1m`，`LIMIT 2000`，并校验查询结果准确性
- 原始 JSON：`docs/benchmarks/downsample_10m_2026-06-21.json`

## 结果

| 指标 | 结果 |
| --- | ---: |
| 总耗时 | 21.283s |
| 写入耗时 | 14.951s |
| 写入吞吐 | 668,851 points/s |
| 降采样耗时 | 6.162s |
| 降采样输出点 | 166,700 |
| 降采样吞吐 | 27,055 points/s |
| 查询耗时 | 140.876ms |
| 查询行数 | 2,000 |
| 查询吞吐 | 14,197 rows/s |
| 全局 RSS peak | 120.50MiB |
| 写入阶段 RSS peak | 40.11MiB |
| 降采样阶段 RSS peak | 58.54MiB |
| 查询阶段 RSS peak | 114.20MiB |
| HeapAlloc | 101.10MiB |
| HeapInuse | 104.93MiB |
| GC 次数 | 4,482 |
| GC 总暂停 | 162.90ms |
| 磁盘占用 | 193.21MiB |
| SSTable 数 | 1,194 |

## 分析

- 查询走的是降采样目标 RP，不是原始 10M 数据；查询窗口覆盖完整目标范围，使用 `LIMIT 2000`。
- 查询阶段 RSS peak 是查询期间进程当前 RSS 的采样峰值，包含写入和降采样后 Go runtime 尚未归还 OS 的内存，不等同于查询本身新增分配。
- 查询耗时 140.876ms，在 166,700 个降采样目标点中返回 2,000 行，结果校验通过。
- 全局 RSS peak 120.50MiB，低于 512MiB 目标，但查询阶段采样值高于降采样阶段，说明 runtime 在后期持有了更多 heap span。若要进一步区分查询新增内存，需要增加查询前后 heap delta 或 pprof alloc profile。
- SSTable 数 1,194 偏高，说明当前 scale 配置下写入阶段仍产生较多小 SSTable；这会影响长期查询和 compaction 成本，后续性能对比应加入写后 compaction 或调大 MemTable 配置的对照组。
