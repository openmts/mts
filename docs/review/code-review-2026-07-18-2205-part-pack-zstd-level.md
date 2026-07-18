# 检视与实现：part 组件合并 + 可配置 zstd 级别

- 日期：2026-07-18
- 状态：已实现（POC，不兼容旧多文件布局）

## 目标

1. 将 part 多文件布局合并为更少文件，降低 inode/固定税。
2. 贯通 `Compression.ZstdLevel`，支持 fastest/default/better/best。

## 方案

### pack 布局

```text
metadata.bin
pack.bin
```

`pack.bin`：

```text
MTSPAK1 | section_count | (name,size)* | payloads
```

逻辑组件名仍为 metaindex/index/series_index/timestamps/values/strings；block ref 相对 section 起始。

### zstd 级别

- 选项：`options.go` / `internal/model` / `convert_options.go`
- 编码：`payload_compression.go` 按 level 分 encoder pool
- 写路径：`compression.go` 透传 `opts.ZstdLevel`
- scale：`-zstd-level`

## 验证

- `go test ./internal/sstable ./internal/storagecheck ./internal/engine . ./tests/scale/storage_10m`
- 全量 `make test` / `make e2e` / `make lint`（实现后执行）
- 10M：zstd + page 4096 + omit-write-seq + typed + compact，对比体积

## 风险

- 打开 part 后若外部改写 pack 再查询，已打开句柄读旧视图；测试改为 reopen 验证损坏。
- metrics 将 `pack.bin` 计入 data bytes（POC 简化）。


## 10M 复测（zstd default + page 4096 + omit-write-seq + typed + compact）

| 指标 | 基线 (const-step) | 本轮 (pack + ZstdLevel) |
|---|---:|---:|
| data_bytes | ~6.5 MiB | **6.50 MiB** (6,815,023 B) |
| write_tp | ~380k pts/s | **367k pts/s** |
| query latency | ~73 ms | **75.6 ms** |
| sstable after compact | 17 | 17 |
| 每 part 文件数 | 7 | **2** (`metadata.bin`+`pack.bin`) |

结论：

- 体积与 const-step 后基线持平（pack 主要降 inode/文件数固定税，不额外压 values 载荷）。
- 写/查吞吐未见明显劣化（波动 <5%）。
- 临时数据已清理。

## 处理状态

- [x] 可配置 zstd 级别贯通编码路径
- [x] part 组件合并 pack.bin
- [x] 单测 / e2e / lint
- [x] 10M 复测

- 中间逻辑文件清理使用 `os.Remove`（不走 fault inject），避免 compact cleanup 配额被误消耗。
