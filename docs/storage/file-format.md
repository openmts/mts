# mts 存储文件格式

## 通用 Envelope

`codec.Envelope` 用于 Manifest、SSTable metadata、index、metaindex、series index 等逻辑块：

```text
magic[7] | flags uint16le | payload_len uvarint | payload bytes | crc32c uint32le
```

CRC 使用 Castagnoli，多字节整数按小端或 payload 内部约定编码。读取端必须校验 magic、payload 长度和 CRC。

## WAL Segment

每个 `*.wal` segment 以固定 header 开头：

```text
"MTSWAL2"[7] | format_id uint16le | header_len uint16le | header_crc32c uint32le
```

当前 `format_id=1`，`header_len=15`。header 后是连续 record frame：

```text
body_len uint32be | record_type byte | payload bytes | record_crc32c uint32be
```

`record_type=1` 表示写入 batch，`record_type=2` 表示 tombstone batch。replay 时最后一个 segment 的尾部 partial record 可截断；header 缺失、未知 format、record CRC 错误必须报错。

## Manifest

Manifest 文件名为 `MANIFEST.bin`，magic 为 `MTSMAN2`。新格式设置 envelope flag `1`，payload：

```text
sequence uvarint | part_count uvarint | part_meta...
```

旧 payload 无 sequence flag 时按 `sequence=0` 处理。`part_meta` 包含 part id、level、time range、series range、row count、series count、block count、max write seq 和 part path。

## SSTable Part

每个 part 是一个目录。最终布局（POC，不兼容旧多文件）：

```text
metadata.bin
pack.bin
```

`metadata.bin` magic 为 `MTSPRT2`，payload 包含 `PartMeta`、`index_ref`、`metaindex_ref`、`series_index_ref`、`created_unix` 和 **逻辑** component 列表（仍按 `metaindex/index/series_index/timestamps/values/strings` 命名，便于 block ref 语义保持不变）。

`pack.bin` magic 为 `MTSPAK1`：

```text
"MTSPAK1"[7]
section_count uvarint
for each section:
  name_len uvarint | name bytes | size uvarint
section payloads (按 section 顺序紧密排列)
```

逻辑组件内容与历史独立文件一致：`index/metaindex/series_index` 使用 `MTSIDX2`/`MTSMIX2`/`MTSSIX2` envelope；`timestamps/values` 使用 block frame：

```text
payload_len uint32be | payload bytes | crc32c uint32be
```

block ref 的 offset/size 是相对逻辑 section 起始的偏移，而不是整个 `pack.bin` 的绝对偏移。

payload 压缩算法支持 `none|snappy|lz4|zstd`；当算法为 `zstd` 时，`Compression.ZstdLevel` 可选 `fastest|default|better|best`（空=default）。

`OpenPart` 必须校验 pack section 完整、metadata block ref 不越界、index/series index/time/value page block CRC 正确；任一组件不一致时拒绝加载。
