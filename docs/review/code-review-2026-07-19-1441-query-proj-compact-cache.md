# 字段投影 / compact 内存 / 可配置 page cache

- **日期**: 2026-07-19
- **基线**: `a7c8652`（整行冷 42.84ms / 热 41.79ms / MaxRSS 1216MiB）
- **状态**: 已实现并通过全量门禁，10M 达标合入

## 1. 实现项

| ID | 项 | 改动 | 效果 |
|---|---|---|---|
| 1 | 字段投影下推 | 行查询保留 `FieldIDs` 存储裁剪；谓词字段仍参与扫描，最终行层投影 | 单字段投影冷/热 **6.1 / 5.2 ms** |
| 2 | compact 内存 | 默认并行上限 2；`OpenPartTrusted` 关 page cache；大 series 按 65536 样本窗口切分写入 | MaxRSS **1216 → 153 MiB（-87%）** |
| 3 | 可配置 page cache | `QueryPageCache{Limit,MaxSamples}`；`Limit=-1` 关闭；默认 256/512 | 查询可配，compact 路径强制不缓存 |

## 2. 10M 对比

配置：`zstd + page4096 + omit-write-seq + typed + compact + projected-query`。

| 指标 | 基线 `a7c8652` | 本轮 | 变化 |
|---|---:|---:|---:|
| 整行冷查询 | 42.84 ms | 47.46 ms | +10.8% |
| 整行热查询 | 41.79 ms | 44.90 ms | +7.5% |
| **投影冷（f0）** | n/a | **6.14 ms** | 约 **7.7x 快于整行冷** |
| **投影热（f0）** | n/a | **5.20 ms** | 约 **8.6x 快于整行热** |
| 写吞吐 | 389k pts/s | 395k pts/s | +1.5% |
| data_bytes | 6.55 MiB | 6.55 MiB | 持平 |
| **MaxRSS** | **1216 MiB** | **153 MiB** | **-87%** |
| write VmRSS | 290 MiB | 110 MiB | -62% |
| compact VmRSS | 472 MiB | 56 MiB | -88% |
| 冷/热查询 VmRSS | 449 / 405 MiB | 45 / 45 MiB | -90% |
| compact 时长 | ~16s（历史）/基线未细比 | 27.3s | 可接受（并行上限 2） |

### 合入判定

- 整行查询有 **约 +8%~11%** 轻微回退（可接受阈值 ≤15%）。
- **RSS 大幅下降**、**投影查询达到 VM 同数量级（单字段 ~5–6ms）**，综合收益明确 → **合入**。

### 整行略慢原因

1. compact 默认并发从更高值收敛到 ≤2，输出布局/调度略有变化，整行 10 字段仍解码全量字段。
2. page cache 对 compact 路径关闭后，查询冷路径更“干净”，但热路径缓存收益有限（窗口命中面大、整行字段多）。
3. 投影路径才真正减少解码量；整行场景仍是 10 字段 * 100 series merge。

## 3. API / 配置

```go
mts.Options{
  QueryPageCache: mts.QueryPageCacheOptions{
    Limit:      256, // 0=默认, -1=关闭
    MaxSamples: 512, // 0=默认
  },
  MaxConcurrentCompaction: 2, // 0=默认最多2
}
```

`storage_10m` 新增：

- `-page-cache-limit` / `-page-cache-max-samples`
- `-max-concurrent-compaction`
- `-projected-query`（默认 true，输出 `projected_*_query_latency_nanos`）

## 4. 验证

- [x] `go test ./internal/sstable ./internal/engine ./internal/queryexec ./tests/scale/storage_10m`
- [x] `make test`
- [x] `make e2e`
- [x] `make lint`
- [x] 10M：`/tmp/mts-10m-q4/report-v2.json`
