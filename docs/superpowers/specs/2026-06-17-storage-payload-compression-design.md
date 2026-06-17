# Storage Payload Compression Design

## 目标

在 SSTable 现有 typed encoding 之后增加 payload 级通用压缩，让 timestamp、write sequence、value 三段编码后的字节都能按配置使用 `snappy`、`lz4`、`zstd` 压缩。默认配置保持不做通用压缩，避免无意改变写入成本。

## EARS 需求

- 当 `CompressionOptions.Enabled=true` 且 `CompressionOptions.Algorithm` 为 `snappy`、`lz4` 或 `zstd` 时，系统应在 typed encoding 完成后对 timestamps、writeSeqs、values 三段 payload 分别执行通用压缩。
- 当 `CompressionOptions.Algorithm` 为空或 `none` 时，系统应保持现有 payload 格式语义，不做通用压缩。
- 当读取 compressed value page 时，系统应先根据 payload header 解压字节，再把解压后的 payload 交给 timestamp/writeSeq/value typed decoder。
- 当配置了未知压缩算法时，系统应返回明确错误，禁止静默回退。
- 当读取到未知压缩算法 ID、截断 header、截断 payload 或解压后长度不等于 header 中的原始长度时，系统应返回明确的数据损坏错误。
- 当显式配置 `snappy` 或 `zstd` 时，系统应使用 `github.com/klauspost/compress` 的 pure Go 实现。
- 当显式配置 `lz4` 时，系统应使用 `github.com/pierrec/lz4/v4` 的 pure Go LZ4 block codec，不引入 cgo。
- 如果 `lz4` block codec 判断单段 payload 不可压缩，系统应把该段 payload 算法标记为 `none` 并存储原文，避免为了显式算法扩大单段落盘。

## 方案对比

### 方案 A：payload wrapper 压缩

在每段 typed payload 外包一层 header：`typedCodec`、`payloadAlgorithm`、`rawSize`、`storedSize`、`storedPayload`。写路径只在 `appendCodecPayload` 附近处理，读路径只在 `readCodecPayload` 附近处理。

优点：边界清晰，对现有 typed decoder 和 streaming 查询路径侵入小；timestamp/writeSeq/value 三段都统一受益；错误检测集中。缺点：压缩 header 比旧格式多几个字节，旧 SSTable 不兼容。

### 方案 B：blockWriter 级压缩

对 values.bin 中每个 block 统一压缩，而不是对 typed payload 压缩。

优点：实现位置更低，所有 block 都可受益。缺点：会影响 index/value page index 等非数据块，查询单页时需要解压更多无关数据，且无法按 typed payload 精准比较压缩收益。

### 方案 C：字段类型 codec 内联压缩

在 float/int/string 的具体 encoder 内分别调用算法压缩。

优点：可以按类型做更细粒度策略。缺点：会重复实现 header 和错误处理，writeSeq/timestamp 也要各自改，耦合度更高。

选择方案 A。它符合当前 SSTable 已经存在的三段 payload 边界，改动集中，能保持读写语义和 streaming decoder 的调用方式。

## 设计

`CompressionOptions` 新增 `Algorithm string`。支持值为 `""`、`none`、`snappy`、`lz4`、`zstd`。空和 `none` 表示不启用通用压缩；未知值直接返回错误。

compressed value page 内每段 codec payload 改为：

```text
typed codec byte
payload compression algorithm byte
uncompressed size uvarint
stored size uvarint
stored payload bytes
```

算法 ID：

```text
0 none
1 snappy
2 lz4
3 zstd
```

写入时，typed encoder 先产生原始 payload，然后按 `Algorithm` 压缩 payload 并写入 wrapper。读取时，先解析 wrapper 和算法 ID，解压得到原始 typed payload，再走现有 decoder。`readCompressedSamples`、`readCodecSamples` 和直接 reader 不感知通用压缩算法。

`snappy` 与 `zstd` 使用 `github.com/klauspost/compress`。`zstd` 编解码器通过 `sync.Pool` 复用，避免热路径反复创建大型 encoder/decoder。`lz4` 使用 `github.com/pierrec/lz4/v4` 的 block codec，并通过 `sync.Pool` 复用 compressor。LZ4 block 压缩返回 0 时，该 payload 使用 `none` 存储原始字节。

## 测试策略

- 单元测试覆盖 `none/snappy/lz4/zstd` round trip。
- 单元测试覆盖未知配置、未知算法 ID、截断 payload、解压后长度不匹配。
- SSTable round trip 测试覆盖四种字段类型和 query 过滤。
- 尺寸测试使用重复字符串/规律数值验证开启 payload 压缩后 `values.bin` 小于未启用通用压缩。
- 全量验证执行 `go test ./...`、覆盖率、`goimports-reviser`、`golangci-lint` 和 `tests/e2e`。
