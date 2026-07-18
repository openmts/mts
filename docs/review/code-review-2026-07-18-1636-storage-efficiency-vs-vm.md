# MTS 存储效率 vs VictoriaMetrics：差距根因与优化方案

- 日期：2026-07-18
- 目标：以 VM 数据存储效率为参照，解释压缩差距，并给出 MTS 可落地优化方案
- 基线（同模型 10M 逻辑点 × 10 字段 = 100M samples）

| 引擎 | 数据体积 | B/sample | 相对 VM |
|---|---:|---:|---:|
| VictoriaMetrics | ~10.2 MiB | ~0.11 | 1.0x |
| MTS snappy | ~553 MiB | ~5.80 | ~54x |
| MTS off | ~1062 MiB | ~11.1 | ~104x |

---

## 1. 为什么 VM 能压得这么小？

### 1.1 数据模型与布局

| 维度 | VictoriaMetrics | MTS 现状 |
|---|---|---|
| 组织单位 | 按 **TSID/series** 排序的大 block | 按 **series+field column + page(256 samples)** |
| 时间跨度合并 | 后台 merge 成更大 part，压缩窗口大 | 本轮 scale 产生 **116 shard × 多 part**，窗口碎 |
| 字段形态 | 每个 field 独立 metric series（高同质列） | multi-field 共 measurement，但 value page 仍按 field 切 |
| 通用压缩 | 在专用编码后再压（常 zstd 级效果） | snappy 通用压缩 + 较弱 typed 编码组合 |

VM 的核心不是“同一个 snappy 压得更狠”，而是：

1. **先做时序专用编码**（timestamp / float / int）把冗余降到接近信息论下界  
2. **再对大块同质字节做强通用压缩**（大 window + zstd 类算法）  
3. **减少固定元数据与每 sample 附加字段**  
4. **后台 merge 保证 block 足够大**，避免小页碎压缩

### 1.2 时序专用编码（VM 强项）

典型 Prometheus/VM 路线：

| 数据类型 | VM/TSDB 常见编码 | 对 scale 数据的效果 |
|---|---|---|
| timestamp | delta-of-delta + 位打包 | 1s 等间隔 → 几乎全 0，极小 |
| float64 | Gorilla XOR + 前导/尾随零位打包 | 平滑序列很强；单调计数器中等 |
| int | delta + varint/bitpack | `i0=index` 几乎每点 delta=1，极小 |
| 重复字符串 | 字典/不存（label 侧） | `s0="ok"` 几乎零成本 |
| bool | bitset | 1 bit/点 |

### 1.3 MTS 当前编码实际在做什么

MTS 已有专用编码骨架（默认）：

- timestamp: `delta-of-delta`
- float: `xor`（**仅存 `uvarint(xor)`，无 Gorilla 位打包**）
- int: `delta + zigzag uvarint`
- string: `dictionary`
- 再可选 payload 通用压缩：`snappy|lz4|zstd`

关键弱点：

| 问题 | 现象 | 对 10M scale 的影响 |
|---|---|---|
| page 仅 256 samples | `valueBlockPageSamples=256` | 压缩字典冷启动频繁，帧头/CRC 放大 |
| XOR 未位打包 | 单调 float 的 xor 常接近满 64-bit | f0..f4 几乎压不动，再靠 snappy 有限 |
| 每 sample 带 writeSeq | value page 内额外一列 | 每点固定附加成本 |
| block frame 开销 | `len+payload+crc32c` 每 block | 小页时占比高 |
| part/shard 固定组件 | metadata/index/metaindex/series_index/strings… | 116 shard 固定税显著 |
| 默认通用压缩偏弱 | 对比用 snappy | 同编码下 zstd 通常再降一截 |
| flush 粒度碎 | memtable→多 L0 part | merge 前体积远大于稳态 |

### 1.4 量级拆解（估算）

对 100M samples，若做到“接近 VM”：

| 成本项 | 理想量级 | MTS 现状倾向 |
|---|---:|---|
| 共享 timestamp（每 series） | 很低（DoD） | 有 DoD，但 page 碎 + 重复存于 value page 时间引用 |
| float 5 列 | 低~中 | XOR 无 bitpack + snappy → 仍大 |
| int 3 列 | 极低 | delta 有帮助，但仍受 page/frame 拖累 |
| string/bool | 极低 | dict/plain 尚可 |
| writeSeq | 0（若可不存或可删） | 每 sample 常驻 |
| 索引/元数据 | 很低 | 多文件多 part 偏高 |

结论：**差距 50x+ 不是单点 bug，而是编码深度 × 页粒度 × 布局碎片 × 附加列 × 压缩算法** 的乘积。

---

## 2. MTS 可优化方向（按收益排序）

### P0 — 立即可见的存储收益（目标：先打到 5~15x 内相对 VM，而不是 54x）

#### P0-1 默认启用更强 payload 压缩 + 分级策略
**EARS**：当 `Compression.Enabled=true` 时，系统应支持按 level 选择算法：L0=`snappy/lz4`（写快），L1+=`zstd`（存小）。  
**验收**：同 10M scale，L1 稳态 `data_bytes` 相对 snappy 再降 ≥30%，写吞吐回落 <25%。

#### P0-2 增大 value page 与压缩阈值
**EARS**：当写入 SSTable 时，系统应支持可配置 `valueBlockPageSamples`（建议默认 1024/4096，冷数据更大）。  
**验收**：page=256→4096 后，同数据 `values.bin` 下降 ≥20%，点查延迟不劣化 >15%。

#### P0-3 压缩阈值策略
**EARS**：当 page samples < `MinPageValues` 或探测压缩率 < 阈值时，系统应回退 none，避免“越压越大”。  
**验收**：随机小页不膨胀；重复串/等间隔时间页压缩率可观测。

#### P0-4 写路径默认打开 typed+payload 压缩
**EARS**：scale/生产默认 profile 应 `Enabled=true, Algorithm=zstd(or snappy), MinPageValues` 合理，而不是 off。  
**验收**：`make storage-10m` 默认报告带压缩，体积相对 off ≥40% 下降（已用 snappy 验证约 48%）。

---

### P1 — 编码深度对齐 VM（目标：再砍 2~5x）

#### P1-1 Float Gorilla 真位打包
**EARS**：当 `Compression.Float=xor|gorilla` 时，系统应存储 leading/trailing zeros + 有效 bit，而不是仅 `uvarint(xor bits)`。  
**验收**：单调/平滑 float 列压缩率显著优于当前 xor；f0..f4 体积降 ≥40%。

#### P1-2 Timestamp bitpack / 常数步长快路径
**EARS**：当 page 内检测到固定 step（如全 1s）时，系统应存 `base + step + count` 或 DoD 位打包，而不是逐点 varint。  
**验收**：等间隔 timestamp 接近 O(1) page header 成本。

#### P1-3 Int 位宽压缩 / RLE
**EARS**：当 int delta 落在小范围时，系统应使用 bit-packing 或 RLE（delta=1 长 run）。  
**验收**：`i0=index` 场景接近 1 bit 级增量成本。

#### P1-4 Bool bitset、String 全局/页字典强化
**EARS**：bool 必须 bitset；string 高重复应 dict + 可复用 page 字典。  
**验收**：`s0="ok"`、`b0` 接近理论下界。

#### P1-5 writeSeq 可选/延迟物化
**EARS**：当未启用版本冲突/精确写序查询时，系统应允许 SSTable 省略 per-sample writeSeq，或仅存 run-length。  
**验收**：关闭 writeSeq 后体积再降 ≥8%~15%（视页结构）。


### P1 实施状态（2026-07-18）

| 项 | 状态 | 备注 |
|---|---|---|
| P1-1 Gorilla float | **已完成** | `compressionXOR` 载荷改为 leading/trailing 位打包；POC 不兼容旧 uvarint-xor |
| P1-2 固定 step 时间戳 | **已完成** | 新 codec `compressionConstStep`：`base + step` |
| P1-3 Int RLE | **已完成** | 新 codec `compressionRLE`，与 delta 自动择短 |
| P1-4 Bool/String | **已完成（部分）** | bool 本就是 bitset；string 增 const/ordinal RLE |
| P1-5 writeSeq | **已完成** | delta-RLE + `OmitWriteSeq` 可省略（解码为 0） |

**10M 复测（zstd + value-page-samples=4096 + typed + compact）**：
- P0 后：约 **479 MiB**
- 仅 P1 typed 编码初版：约 **476 MiB**（writeSeq 税掩盖）
- P1 + writeSeq RLE：约 **115 MiB**（`data_bytes=120603121`）
- P1-4/P1-5（+string const/RLE + omit-write-seq）：约 **113.6 MiB**（`data_bytes=119109606`）
- **float const-step / int-as-float RLE**：约 **75.8 MiB**（`data_bytes=79425428`）
- **P2 冷层 page + zstd 强化 + 7d shard**：约 **63.7 MiB**（`data_bytes=66804649`，shard 116→17）
- **行时间戳 const-step（timestamps.bin）**：约 **6.5 MiB**（`data_bytes=6813204`）
- 相对 P0：约 **-98.6%**；相对 VM ~10 MiB 约 **0.65x（更优）**
- 说明：此前 timestamps.bin 朴素 delta 占 part 约 90%；const-step 后体积逼近/优于 VM 同模型






---

### P2 — 布局与合并（目标：稳态体积与 VM 同量级逼近）

#### P2-1 大 block 合并与“冷层重压”
**状态**：部分完成（L1+ page=16384 + zstd SpeedDefault）
**EARS**：当 part 进入 L2+ 时，系统应重编码为更大 page + zstd 高压缩级。  
**验收**：flush 瞬时体积可偏大，但 compact 后稳态体积显著下降（双阶段体积曲线）。

#### P2-2 降低固定税：组件合并 / 嵌入式小索引
**EARS**：系统应减少每 part 多文件固定开销（小 part 可打包单文件或省略空组件）。  
**验收**：高 shard 数场景下元数据占比下降可测。

#### P2-3 Shard 策略与时间对齐
**状态**：scale 默认 7d（10M 场景 shard 116→17）
**EARS**：对等间隔高密度写入，系统应避免无意义的过细 shard 切片导致 100+ 微小 part。  
**验收**：同 10M 数据 part/shard 数下降，固定开销下降。

#### P2-4 统计驱动编码选择
**EARS**：每个 value page 应选择最短编码（plain/xor/gorilla/delta/rle/dict）并写入 codec id。  
**验收**：混合负载下“不劣于任一单一策略”。

---

## 3. 推荐实施路线（以 VM 存储效率为目标）

### 阶段 A（1~2 周，低风险）
1. 默认 profile：`snappy` 或 `zstd` + 合理 `MinPageValues`  
2. `valueBlockPageSamples` 可配置，默认升到 1024/4096  
3. 增加 **storage efficiency bench**：固定 10M scale，输出 `B/sample`、分文件体积  
4. L0 snappy / L1+ zstd 分层压缩  

**阶段 A 成功标准**：10M scale 稳态体积从 ~553MiB → **≤150~250MiB**（约再降 2~3.5x）。

### 阶段 B（2~4 周，中风险，编码）
1. Gorilla float bitpack  
2. 固定 step timestamp 快路径  
3. int RLE/bitpack  
4. writeSeq 可关/RLE  

**阶段 B 成功标准**：同负载 → **≤40~80MiB**（相对 VM 约 4~8x 内）。

### 阶段 C（4+ 周，布局）
1. 冷层大页重压  
2. part 组件精简  
3. merge 策略面向“压缩窗口最大化”  

**阶段 C 成功标准**：逼近 **≤15~30MiB**（相对 VM 约 1.5~3x；完全持平需接受模型/功能取舍）。

---

## 4. 不可回避的边界（避免错误预期）

| 边界 | 说明 |
|---|---|
| 功能税 | MTS multi-field 行模型、writeSeq、校验 CRC、本地库 API 语义会保留一定开销 |
| 写放大 vs 体积 | zstd/大页/重编码会换 CPU 与 compact 时间 |
| 对比口径 | VM 10MiB 含其自有编码与 merge 稳态；MTS 若只比 flush 后未充分 merge 会吃亏 |
| 字符串 | VM metric 值不保留文本；MTS 真字符串字段本身就更“重” |
| 正确性优先 | 任何编码优化必须 round-trip + 10M 内容一致性门禁 |

---

## 5. 建议的验收门禁（每完成一阶段）

1. **正确性**：`Points` 内容一致性（已有 MTS↔期望/交叉）  
2. **体积**：`data_bytes`、分组件 `timestamps.bin/values.bin/index*`  
3. **性能**：写吞吐、compact 时间、窗口查询 P50/P95  
4. **回归**：相对上一阶段体积下降阈值；查询劣化阈值（建议 <10%~15%）  

固定复现命令：

```bash
# MTS 体积基线
go run ./tests/scale/storage_10m \
  -points 10000000 -batch-size 4096 \
  -mode write-query-compact -ingest-path typed \
  -compression-algorithm zstd \
  -durability buffered -verify=false
```

---

## 6. 结论与下一步

**VM 强，是因为“专用时序编码 + 大块 + 强压缩 + 低固定税”叠乘；MTS 弱，主要弱在 page 太碎、float XOR 不够深、writeSeq/帧头税、merge/算法未拉满。**

最划算的顺序：

1. **先大页 + zstd 分层**（快、收益稳）  
2. **再 Gorilla/步长快路径/int RLE**（对齐 VM 编码深度）  
3. **最后布局与冷层重压**（冲刺同量级）

若进入实现阶段，建议从 **P0-2（page size）+ P0-1（zstd 分层）** 开刀，并用 10M scale 的 `B/sample` 作为唯一主指标盯紧。
