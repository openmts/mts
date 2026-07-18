# 代码检视 / 实施报告：P1 编码深度（Gorilla / 步长 / RLE）

- 日期：2026-07-18 18:55
- 范围：`internal/sstable` 时序专用编码
- 状态：**已完成并验证**

## 1. 目标

对齐 VictoriaMetrics 级时序编码深度，降低 10M scale 稳态体积。

## 2. 变更摘要

| 项 | 实现 | 文件 |
|---|---|---|
| Gorilla float | 替换 `compressionXOR` 载荷为位打包（MSB-first） | `compression_gorilla.go`, `compression_bits.go` |
| 固定步长时间戳 | 新 codec `compressionConstStep`：`base(int64 LE)+step(varint)` | `compression_time.go` |
| Int delta-RLE | 新 codec `compressionRLE`，与 delta 自动择短 | `compression_int_rle.go`, `compression_values.go` |
| writeSeq delta-RLE | 递增 seq 走 RLE，显著降低 page 税 | `compression_writeseq.go` |

说明：POC 不兼容旧 uvarint-xor float 载荷。

## 3. 编码层探针（page=4096）

| 列 | plain | 压缩后 | codec |
|---|---:|---:|---|
| float 常数 | 32768 | 520 | XOR/Gorilla |
| float f0=index | 32768 | 5761 | XOR/Gorilla |
| float f1=index*1.1 | 32768 | 27390 | XOR/Gorilla |
| int i0=index | 8128 | 4 | RLE |
| ts 1s 步进 | 20483 | 13 | ConstStep |
| writeSeq 递增 | 8065 | 4 | RLE |

## 4. 10M 体积对比

条件：`points=10M, typed, zstd, value-page-samples=4096, buffered, write-query-compact`。

| 阶段 | data_bytes | MiB |
|---|---:|---:|
| 旧 snappy+page256（历史） | ~580M | ~553 |
| P0 zstd+page4096 | ~502M | ~479 |
| P1 typed 编码（无 writeSeq RLE） | 499133929 | 476.0 |
| **P1 + writeSeq RLE** | **120603121** | **115.0** |
| VM 参考 | ~10M | ~9.3~10 |

相对 P0：约 **-76%**。相对 VM：约 **11~12x**（仍有 string/bool/布局/固定税空间）。

本轮性能：
- write_throughput ≈ 356k pts/s
- duration ≈ 35s
- query_latency ≈ 43ms
- 未见明显劣化（相对 P0 同场景 ~39s / ~257k 吞吐）

## 5. 验证

- `internal/sstable` coverage: **90.1%**
- `go test ./internal/sstable` 通过
- `make test` 通过
- `make e2e` 通过
- `make lint` 0 issues
- 10M scale 复测完成，临时数据已清理

## 6. 后续

- P1-4：bool bitset / string 字典强化
- P2：冷层更大 page 重压、shard/part 固定税、省略 writeSeq 配置项
- 目标下一阶段：10M ≤40~80 MiB
