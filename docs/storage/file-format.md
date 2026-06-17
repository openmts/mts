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

每个 part 是一个目录，必需组件：

```text
metadata.bin
metaindex.bin
index.bin
series_index.bin
timestamps.bin
values.bin
strings.bin
```

`metadata.bin` magic 为 `MTSPRT2`，payload 包含 `PartMeta`、`index_ref`、`metaindex_ref`、`series_index_ref`、`created_unix` 和 component 列表。缺失 component 列表的旧 metadata 默认使用上述必需组件。

`index.bin`、`metaindex.bin`、`series_index.bin` 分别使用 `MTSIDX2`、`MTSMIX2`、`MTSSIX2` envelope。`timestamps.bin` 和 `values.bin` 使用 block frame：

```text
payload_len uint32be | payload bytes | crc32c uint32be
```

`OpenPart` 必须校验 component 存在、metadata block ref 不越界、index/series index/time/value page block CRC 正确；任一组件不一致时拒绝加载。
