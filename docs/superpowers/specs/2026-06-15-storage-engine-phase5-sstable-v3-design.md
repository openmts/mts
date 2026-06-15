# mts Storage Engine Phase 5 SSTable V3 Design

## 背景

Phase 4 将 MemTable 写入结构改为 append-only column buffer，wide10 10K 写入从 Phase 3 的约 `63ms/op`、`87.8MB/op`、`181.2k allocs/op` 降到约 `46.7ms/op`、`66.4MB/op`、`167.6k allocs/op`。当前主要瓶颈已经转移到 SSTable flush 和 Catalog resolve。

当前 SSTable 已使用二进制格式，但 value block 仍然为每个字段列重复保存 sample timestamp。与此同时，每个 series row 已经有一个共享 time block，记录该 series 下所有字段列的 timestamp 并集。因此，对于 wide10 workload，10 个字段会重复保存 10 份相同或高度重叠的 timestamp delta。这会放大落盘体积、flush 编码量和读取解码量。

本阶段目标是升级 SSTable value block 编码，让 value block 引用 row time block 的位置，避免重复落盘 timestamp，并在读取时减少中间切片分配。

## 方案对比

### 方案 A：只复用编码 buffer，不改格式

优点是风险最低，能减少 flush 中的临时分配。缺点是磁盘结构不变，value block 仍重复存 timestamp，无法满足“落盘结构足够紧密”的目标。

### 方案 B：SSTable value block v3 使用 time index 引用

每个 series row 继续写一个 time block。value block v3 不再保存 timestamp delta，而保存 sample 在 row time block 中的 ordinal。若列样本与 row timestamps 完全对齐，则只保存一个 aligned mode，不保存 ordinal 数组。读取时通过 row timestamps 还原样本 timestamp。

优点是直接减少 wide series 的磁盘重复数据，并让读取能基于 row time block 做范围过滤。缺点是 read path 需要将 row timestamps 传给 value decoder，格式兼容逻辑略复杂。

### 方案 C：引入独立 string dictionary 和列级压缩算法

优点是长期压缩率最好。缺点是会改变更多文件结构，影响 compaction、读路径和损坏检测。当前阶段应先完成 time index 引用，再单独设计 dictionary 和 Gorilla/XOR/delta-of-delta 压缩。

推荐采用方案 B，并保留 v2 解码兼容能力。新写入默认使用 v3。

## 范围

### 本阶段包含

- 新增 SSTable value block v3 编码。
- value block v3 支持两种 timestamp 引用模式：
  - `aligned`: 列样本数量等于 row time block 数量，且每个 sample timestamp 与 row timestamp 同位置一致，不写 ordinal。
  - `indexed`: 写入按升序排列的 row time ordinal delta。
- query row 时读取 time block 后传入 value block decoder。
- 解码 value block 时直接构造 `[]VersionedSample`，不再先构造 `[]timestamp`、`[]writeSeq`、`[]FieldValue` 三个中间切片。
- `writeBlock` 避免构造完整 frame 副本，改为写 header、payload、crc，减少编码时大块临时分配。
- 保留旧 v2 value block 解码，已有 Part 仍可读取。
- 增加磁盘大小 benchmark/smoke 文档，记录 wide10 写入后的 Part 体积变化。

### 本阶段不包含

- 不改变 public API。
- 不改变 WAL 格式。
- 不引入外部压缩库。
- 不实现 string dictionary。
- 不实现 float Gorilla/XOR 或 timestamp delta-of-delta。
- 不强制兼容 JSON/Gob/CSV，生产持久化继续只使用二进制。

## EARS 需求

- When SSTable writes a value block for a column, the system shall encode it using value block v3 by default.
- When a column sample timestamp exactly aligns with the row time block at the same ordinal, the system shall use aligned mode and omit per-sample timestamp references.
- When a column has a sparse subset of row timestamps, the system shall encode row time ordinals in increasing order.
- When reading a v3 value block, the system shall reconstruct sample timestamps from the row time block.
- When reading a v2 value block, the system shall keep decoding the embedded timestamp payload for compatibility.
- When query time range filters out samples, the system shall avoid appending filtered samples to the result.
- When a v3 value block references an ordinal outside the row time block, the system shall return a corruption error.
- When writing a block frame, the system shall not allocate a full copy of header + payload + checksum.
- If any encoder sees an unsupported field type, the system shall return an explicit error.
- If any decoder sees truncated or malformed binary payload, the system shall return an explicit error.

## 格式

value block v3:

```text
byte   encoding = 3
uvarint field_id
byte   field_type
uvarint sample_count
byte   time_ref_mode
       mode 0 aligned: no ordinal payload
       mode 1 indexed: uvarint first ordinal, then uvarint deltas
uvarint write_seq repeated sample_count
typed values repeated sample_count
```

`indexed` mode 的 ordinal 必须严格递增，且每个 ordinal 必须小于 row timestamp 数量。`aligned` mode 要求 `sample_count == len(rowTimestamps)`。

## 测试策略

- v3 aligned 编码比 v2 更短，并能正确 roundtrip。
- v3 indexed 编码能处理稀疏字段列，并能正确 roundtrip。
- v3 解码拒绝越界 ordinal、非递增 ordinal、缺失 row timestamps。
- v2 value block 仍可解码。
- `Part.Query` 读取新写入的 v3 part 时只读取命中的 value block。
- `writeBlock` 写出的 frame 仍能通过 CRC 校验读取。
- 全量测试、lint、e2e 继续通过，总覆盖率保持 `>=90%`。

## 性能验收

- wide10 10K 写入后的 SSTable value payload 比 Phase 4 更小。
- `BenchmarkEngineWriteWideBatch/points=10000` 不出现明显回退；若 CPU 有小幅波动，必须以磁盘体积和 alloc_space 改善作为依据。
- 1M wide10 pprof smoke 继续在稳定内存下完成。

## 风险与权衡

- v3 decoder 依赖 row time block，因此 read path 需要保留并传递 row timestamps。这个依赖符合当前 SSTable row 结构。
- aligned mode 对 wide10 最有收益；稀疏字段列使用 indexed mode 仍比完整 timestamp 更紧凑。
- query 时仍需读取整块 value payload。后续可以在 v4 中增加 block 内 min/max 或 page index。
