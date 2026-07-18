# Changelog

## Unreleased

### Added

- SSTable part 合并为 `metadata.bin` + `pack.bin`，逻辑组件 section 保留 block ref 语义。
- `Compression.ZstdLevel` 可配置 zstd 强度（fastest|default|better|best），写路径按级别分 pool。

- 行时间戳块 `timestamps.bin` 支持 const-step 编码，显著降低等间隔序列存储。
- 冷层 L1+ 默认更大 value page（16384）与更强 zstd（SpeedDefault）；scale 默认 shard 7d。
- SSTable float 自动选择 const-step / 整数值 delta-RLE / Gorilla 最短编码。
- SSTable 字符串字典强化（const/ordinal RLE）与 `Compression.OmitWriteSeq` 可省略写序号。
- SSTable 时序编码 P1：Gorilla float 位打包、固定步长时间戳、int/writeSeq delta-RLE。

- SSTable `ValuePageSamples` 可配置（默认 1024）；未显式 `Algorithm` 时 L0=snappy、L1+=zstd 分层压缩。
- 公开写接口文档明确优先使用 `WriteTypedBatch` 提升性能。
- `PointsToTypedBatch` 与 `Engine.WritePointsAsTypedBatch`：同构 `[]Point` 转列式写入。
- wide 布局 bench：`Write` / `WritePointsAsTypedBatch` / `WriteTypedBatch` 三路对比。

- 列式 WAL 编码路径减少中间列矩阵分配；`Compaction.MaxConcurrent` 与 `MaxConcurrentCompaction` 配额可配置同步。


- POC：列式 WAL（segment formatID=2）与跨 shard 有界并行 compact。
- `MaxConcurrentCompaction` 默认归一化为 `min(GOMAXPROCS, 4)`。


- 查询默认保护 `QueryProtection` 与 MemTable 乱序降载 flush。
- 查询代价校准：`MatchedParts` / `EstimatedPartRows` / part 行数比例估计。
- `MaxConcurrentCompaction` 全局 compaction 并发配额与维护指标。
- SSTable block CRC round-trip fuzz 测试。
- tombstone 待回收 gauge：`mts_tombstones_pending`。
- 运维文档：`docs/ops/nightly-gates.md`、`delete-reclaim.md`、`query-protection.md`。
- 兼容矩阵扩展见 `docs/compatibility.md`。

### Added (historical)


- 增加根包 README、package godoc 和可执行示例。
- 增加 `DefaultOptions(path)`、`Options.Validate()`、`ErrInvalidOptions`。
- 增加公共错误类别 `ErrNotFound`、`ErrUnsupported`。
- 增加 QueryBuilder `TimeRangeTime(start, end)` 和聚合常量。
- 增加 MIT License、贡献说明和 AI 友好项目摘要。

### Changed

- 拆分根包公开 DTO 和 internal 转换文件，降低 API 维护成本。
- 明确 HTTP 查询服务仍属于 internal 实现，不作为当前外部稳定 API。
