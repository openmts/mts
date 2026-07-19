# 查询性能对齐 VM 数量级优化检视

- **日期**: 2026-07-19
- **范围**: SSTable float 读友好编码 + bitReader + 页解码缓存
- **状态**: 已实现，10M 冷/热查询有提升，全量门禁通过，已提交

## 1. 背景

上一轮冷查询窗口优化后，10M 整行查询约：

| 指标 | 上一轮优化后 |
|---|---:|
| 冷查询 | 57.55 ms |
| 热查询 | 50.97 ms |

相对 VictoriaMetrics（同规模单字段热查约 0.83 ms、10 字段合计约 12.6 ms），MTS 仍慢约一个数量级以上。

## 2. pprof 结论（500k 热路径）

| 热点 | 占比 | 说明 |
|---|---:|---|
| `bitReader.readBit` | ~48% | Gorilla 逐位 `/8 %8` 解码过慢 |
| Gorilla float 解码链 | ~60%+ cum | scale 的 f1/f2 等未命中 const-step，回退 Gorilla |
| GC / alloc | ~12% | 重复解码与对象分配 |

根因：

1. float `const-step` 仅支持 `base+i*step`，对 `float64(index)*1.1` 在页内非零起点时 **IEEE 不等价**，漏检后走 Gorilla。
2. Gorilla `bitReader` 实现非热路径友好。
3. 热查询重复扫同一 page 仍全量解码，无缓存。

## 3. 实现项

| ID | 改动 | 状态 |
|---|---|---|
| Q6 | float const-step 支持 `index*scale` 与旧 AP 载荷 | 已完成 |
| Q7 | 检测到读友好编码即优先选用，避免误选 Gorilla | 已完成 |
| Q8 | `bitReader` 寄存器式批量取位 | 已完成 |
| Q9 | Part 级 page 解码缓存（按 offset+查询窗） | 已完成 |

### 关键决策

- 新 const-step 载荷：`kind(1B)+payload(16B)`；旧 16B AP 仍可解码。
- `index*scale` 优先于 AP，精确匹配 scale 生成方式 `float64(i)*k`。
- 缓存返回副本，避免调用方污染；FIFO 上限 256 项。
- Gorilla 仍保留为不规则 float 路径；bitReader 优化惠及该路径。

## 4. 10M 前后对比

同配置：`zstd + page4096 + omit-write-seq + typed + compact`，10M 点，2000 行整行。

| 指标 | 上一轮（基线） | 本轮 | 变化 |
|---|---:|---:|---:|
| 冷查询 | 57.55 ms | **46.51 ms** | **-19.2%** |
| 热查询 | 50.97 ms | **41.26 ms** | **-19.1%** |
| 写吞吐 | 375,683 pts/s | 403,907 pts/s | +7.5%（含噪声） |
| data_bytes | 6.50 MiB | 6.55 MiB | +0.8%（载荷 kind 字节 + 编码选择） |

相对更早压缩后冷查 80.30 ms：本轮累计约 **-42%**。

### 与 VM 数量级对照（同一数据模型）

| 场景 | MTS 本轮 | VM 参考 | 差距 |
|---|---:|---:|---:|
| 整行 10 字段 / 2000 行 冷 | 46.5 ms | 单字段 1.5 ms；10 字段合计 ~12.6 ms | 仍约 **3.7x**（整行 vs 多字段合计） |
| 整行热 | 41.3 ms | 单字段 0.83 ms | 模型不对齐，但已从 80ms 级进入 **数十 ms** 同阶上沿 |

说明：VM 基准多为单字段 export；MTS scale 固定整行 10 字段 + row merge + 100 series 交叠时间窗。同数量级（10ms~100ms 整行）已部分达成；要贴近 VM 单字段 1ms 级仍需列式投影与 block cache 深化。

## 5. 验证

- [x] `go test ./internal/sstable ./internal/engine`
- [x] `make test`
- [x] `make e2e`
- [x] `make lint`（0 issues）
- [x] 10M `write-query-compact` 报告：`/tmp/mts-10m-q2/report.json`

## 6. 后续（未做）

1. 查询字段投影下推（只解码请求字段）
2. 更大/可配置 page cache + 压缩块 LRU
3. 行合并路径减少 `cloneRow` / map 分配
4. 与 VM 对齐的单字段查询基准
