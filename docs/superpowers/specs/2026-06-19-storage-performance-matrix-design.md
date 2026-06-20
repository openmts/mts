# Storage Performance Matrix Design

用户问题本质：当前性能数据来自零散单场景执行，写入耗时、compaction、压缩、查询和持久化口径混在一起，无法系统比较 100K、1M、10M 三档规模在不同压缩算法和写入策略下的性能差异。

## 目标

构建一个可复用的存储层性能矩阵专项，覆盖三档数据规模、五种压缩算法、四种写入持久化策略，并输出机器可读 JSON 与 Markdown 汇总。矩阵工具必须能全量执行，也必须能按规模、压缩算法和写入策略过滤执行，避免 10M strict fsync 场景拖垮日常验证。

## 范围

- 复用 `tests/scale/storage_10m` 作为单场景执行器。
- 新增 `tests/scale/storage_matrix` 作为矩阵编排器。
- 增加 `storage_10m` 写入持久化策略参数。
- 增加 strict flush 所需的 SSTable 文件同步选项。
- 报告保留现有 limited query 语义：中间时间段、`LIMIT 2000`、逐行正确性校验。

## EARS 验收清单

- When 用户运行单场景性能用例时，系统应允许选择 `buffered`、`wal-sync`、`write-sync`、`strict-flush` 四种写入持久化策略。
- When 用户选择 `buffered` 时，系统应保持当前默认吞吐口径，写入后执行 flush，但不强制每批 WAL fsync 和 SSTable 数据文件 fsync。
- When 用户选择 `wal-sync` 时，系统应通过 Engine WAL 配置强制 WAL 每次 append 后 fsync。
- When 用户选择 `write-sync` 时，系统应通过 `WriteOptions{Sync:true}` 强制本次写入调用对应 WAL append 后 fsync。
- When 用户选择 `strict-flush` 时，系统应启用 WAL 同步、写入同步和 SSTable flush 文件同步，确认 SSTable 数据文件、索引文件、元数据文件和目录在 flush 路径上完成同步。
- When 单场景报告输出时，系统应包含 `durability`、`wal_sync`、`write_sync`、`flush_sync` 字段，避免误读写入耗时。
- When 用户运行矩阵用例时，系统应为每个规模、压缩算法和写入策略组合创建独立数据目录。
- When 矩阵单个 case 结束时，系统应记录写入耗时、compaction 耗时、cold/hot query 耗时、RSS peak、heap、total alloc、SSTable 数量变化、level 分布、data bytes 和 amplification。
- When 查询执行时，系统应固定中间时间范围并 `LIMIT 2000`，同时校验返回数据准确性。
- When 任意矩阵 case 失败或超时时，系统应记录失败 case、错误原因、已完成结果，并返回非零退出码。
- When 用户只需要小规模 smoke 时，系统应允许通过 flags 过滤规模、压缩算法和写入策略。
- When 文档生成时，系统应明确区分 buffered 吞吐口径和强持久化口径。

## 矩阵维度

规模：

- `100k` = 100,000 points
- `1m` = 1,000,000 points
- `10m` = 10,000,000 points

压缩：

- `off`
- `none`
- `snappy`
- `lz4`
- `zstd`

写入策略：

- `buffered`
- `wal-sync`
- `write-sync`
- `strict-flush`

默认 workload：

- `mode=compact`
- `ingest-path=typed`
- `batch-size=4096`
- `memtable-max-samples=8192`
- `query-limit=2000`

## 架构

`tests/scale/storage_10m` 继续负责单场景执行，不做矩阵循环。它新增 durability 参数，并把该参数映射到 `mts.Options.WAL`、`mts.WriteOptions` 和 SSTable flush sync 选项。

`tests/scale/storage_matrix` 负责编排矩阵。它先构建或使用指定的 `storage_10m` runner，再按 case 调用 runner，解析 JSON 输出，生成汇总 JSON 与 Markdown。矩阵 runner 不直接写入引擎数据，避免和单场景逻辑重复。

## 数据目录

矩阵根目录由 `-data-root` 指定。每个 case 使用：

```text
<data-root>/<size>/<compression>/<durability>
```

未显式指定 `-data-root` 时，矩阵使用临时目录并在退出时清理。显式指定时保留数据，方便事后检查磁盘结构。

## 输出

每个 case 保留单场景 runner 的 JSON 字段。矩阵汇总增加：

- `size`
- `points`
- `compression_algorithm`
- `durability`
- `data_dir`
- `started_at`
- `finished_at`
- `duration_nanos`
- `error`
- `report`

Markdown 汇总包含：

- case 标识
- write duration
- compaction duration
- cold/hot query latency
- RSS peak
- data bytes
- SSTable before/after
- level distribution
- status/error

## 验证策略

- 单元测试覆盖 durability 参数解析、写入选项映射、strict flush 选项映射。
- SSTable 测试覆盖 sync 失败会在 strict flush 场景返回错误。
- 矩阵测试覆盖 size/compression/durability 解析、case 生成、命令参数生成、JSON 聚合和 Markdown 输出。
- smoke 执行使用小规模过滤矩阵，不默认跑满 10M 全矩阵。
- 手动性能执行按 100K、1M、10M 分批运行，并将结果写入 `docs/benchmarks`。
