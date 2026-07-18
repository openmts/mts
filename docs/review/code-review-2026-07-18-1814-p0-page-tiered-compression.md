# P0 存储效率实现：ValuePageSamples + 分层压缩

- 日期：2026-07-18 18:14 
- 状态：已实现并通过 make test / make e2e / make lint

## 实现内容

1. **`CompressionOptions.ValuePageSamples`**
   - public / model / convert 贯通
   - 默认 **1024**（原硬编码 256）
   - clamp：`[64, 65536]`
2. **分层压缩算法（未显式 Algorithm 时）**
   - L0 / flush：`snappy`
   - L1+ compact 输出：`zstd`
   - 全局显式 `Algorithm` 时全层级沿用
3. **scale 工具**
   - 新增 `-value-page-samples`
4. **文档**
   - README 增加存储压缩示例；CHANGELOG 记录

## 10M 复测（points=10M, batch=4096, write-query-compact）

| 配置 | 数据体积 | 写入吞吐 | 查询 | Compact | vs 旧 snappy(553MiB) | vs VM(~10MiB) |
|---|---:|---:|---:|---:|---:|---:|
| 旧 snappy + page256（上轮） | 553.3 MiB | ~400k pts/s | ~16ms | ~6.4s | 1.00x | ~54x |
| **snappy + page4096（本轮）** | **508.5 MiB** | 382708 | 43.24ms | 13.749s | 0.92x | 49.9x |
| **zstd + page4096（本轮）** | **479.0 MiB** | 282497 | 38.19ms | 7.695s | 0.87x | 47.0x |

## 结论

- P0 已落地：**页可配置 + 默认放大 + 分层算法**。
- 体积相对旧 snappy 约 **-8.1%（snappy/4096）**、**-13.4%（zstd/4096）**。
- 相对 VM 仍约 **47~50x**，符合阶段 A 预期上限（编码深度/writeSeq/布局税尚未动）。
- 下一阶段应推进 **P1 Gorilla bitpack / 固定 step 时间戳 / int RLE**。

## 验证

- `make test` 通过
- `make e2e` 通过
- `make lint` 0 issues
