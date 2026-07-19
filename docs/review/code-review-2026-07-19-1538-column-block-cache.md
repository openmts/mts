# 列式导出加速 + 查询侧 block payload 缓存

日期：2026-07-19 15:38  
基线 commit：`1d35cc9`（投影下推 + compact 内存 + QueryPageCache）  
状态：**已合入（性能达标）**

## 目标

1. 列式结果导出：降低 `model.ColumnSeries -> mts.ColumnSeries` 转换分配，并引导宽表/导出走 `QueryColumnIterator`。
2. 查询侧 block/mmap 缓存：本轮先做用户态 **block payload 缓存**（ReadAt+CRC 后的原始字节），热路径少做重复读盘与 CRC。

## 实现要点

| 项 | 说明 | 状态 |
|---|---|---|
| `blockPayloadCache` | 默认 limit=512 / maxBytes=64MiB / 单块>1MiB 不缓存 | 已完成 |
| 仅 OpenPart 启用 | OpenPartTrusted（compact）关闭，避免全表扫灌缓存 | 已完成 |
| 仅 timestamps/values | index 不进缓存，避免破坏性校验/原地改写被掩盖 | 已完成 |
| 打开深校验后 clear | 避免 validate 预热污染查询缓存 | 已完成 |
| 配置 | `mts.QueryBlockCacheOptions{Limit,MaxBytes}` 贯通 model/convert/engine | 已完成 |
| 列式转换 | factor==1 时 timestamps 切片拷贝；tags 只读共享；FieldValue 字段拷贝 | 已完成 |
| harness | `storage_10m` 增加 column/projected-column 冷热延迟与 block-cache flags | 已完成 |

## 10M 前后对比

配置：`zstd + page4096 + omit-write-seq + typed + compact + projected-query + column-query`  
points=10,000,000；query limit=2000 行窗口。

| 指标 | 基线 `1d35cc9` | 本轮 | 变化 |
|---|---:|---:|---:|
| 整行冷查 | 47.459 ms | 42.303 ms | **-10.9%** |
| 整行热查 | 44.904 ms | 39.573 ms | **-11.9%** |
| 投影 f0 冷查 | 6.139 ms | 5.157 ms | **-16.0%** |
| 投影 f0 热查 | 5.204 ms | 4.490 ms | **-13.7%** |
| 列式冷查（新增） | n/a | 35.368 ms | — |
| 列式热查（新增） | n/a | 35.205 ms | — |
| 投影列式冷查 | n/a | 3.663 ms | — |
| 投影列式热查 | n/a | 2.598 ms | — |
| MaxRSS | 152.69 MiB | 156.75 MiB | +2.7% |
| 写吞吐 | 395093 pts/s | 384989 pts/s | -2.6% |
| data_bytes | 6.55 MiB | 6.55 MiB | 0% |

报告：

- 基线：`/tmp/mts-10m-q4/report-v2.json`
- 本轮：`/tmp/mts-10m-q5/report-opt.json`

## 验收结论

- 查询路径有明确提升（整行约 -11%，投影约 -14%~-16%）。
- 投影列式热查约 **2.6 ms**，更接近应用侧应走的路径。
- RSS 轻微上升（约 +4 MiB）可接受，未出现数量级回退。
- **判定：合入主干。**

## 门禁

- [x] `go test . ./internal/sstable ./internal/engine ./tests/scale/storage_10m`
- [x] `make test`（二次全量通过；首次 pprof 偶发目录竞态，与本改动无关）
- [x] `make e2e`
- [x] `make lint`（0 issues）
- [x] goimports-reviser 变更文件

## 风险与后续

1. block cache 是 per-Part 用户态拷贝缓存，非真 mmap；后续可评估只读 mmap + 进程级共享。
2. 列式公开类型与 model 跨包，Values 仍需逐元素 FieldValue 拷贝。
3. 整行 10 字段仍受 row merge 主导；应用侧应优先 Fields 投影 + `QueryColumnIterator`。
