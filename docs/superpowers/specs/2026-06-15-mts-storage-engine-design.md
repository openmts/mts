# mts 存储引擎设计规格

日期：2026-06-15
状态：已通过头脑风暴确认，等待用户审阅

## 背景

`mts` 是使用 Go 实现的 micro time series database。第一版优先建设高质量嵌入式存储引擎库，而不是完整数据库服务。存储引擎是后续 HTTP/gRPC 服务、查询语言、集群能力和运维工具的基础。

项目当前是空 Go module，模块路径为 `github.com/openmts/mts`。第一版设计应保持实现边界清晰、测试充分、崩溃恢复可靠，并为后续性能优化预留格式与接口空间。

## 目标

- 实现一个嵌入式 Go 时序存储引擎库。
- 支持多值时序点模型：`measurement + tags + timestamp + fields`。
- 支持字段类型：`float64`、`int64`、`string`、`bool`。
- 支持有限乱序写入。
- 重复点采用 Last Write Wins，以 `writeSeq` 判定最新值。
- 使用 LSM Tree 存储模型，包含 WAL、MemTable、SSTable Part、flush、compaction。
- 使用时间 shard 作为数据生命周期和 retention 边界。
- 支持跨 shard 查询。
- 底层查询返回列式结果，上层提供行式适配器。
- SSTable 使用目录式列式 Part，文件格式预留编码演进空间。

## 非目标

第一版不实现以下能力：

- 分布式集群。
- HTTP/gRPC 服务。
- 完整查询语言。
- remote write/read 协议。
- 高级压缩的完整优化实现。
- 任意 series 或时间范围删除。
- downsampling。
- 权限认证。

## EARS 需求

- 当客户端写入一个多值时序点时，系统应根据 `measurement + sorted tags` 获取或创建 `seriesID`，并根据字段名获取或创建 `fieldID`。
- 当客户端写入字段值时，系统应只接受 `float64`、`int64`、`string`、`bool` 四种字段类型。
- 当同一 `measurement + fieldName` 已存在字段类型时，系统应拒绝不同类型的新写入。
- 当同一 `(seriesID, fieldID, timestamp)` 被重复写入时，系统应返回最大 `writeSeq` 对应的值。
- 当写入时间戳落在允许乱序窗口内时，系统应接收该写入并在 MemTable 与 compaction 中保持有序查询结果。
- 当写入时间戳超过允许乱序窗口或不属于任何可写 shard 时，系统应返回明确错误。
- 当普通写入成功返回时，系统应至少已将 WAL record 交给配置的 durability 策略处理。
- 当强同步写入成功返回时，系统应确认对应 WAL record 已完成 fsync。
- 当 MemTable 达到阈值时，系统应创建只读 snapshot，并将 snapshot flush 为 L0 SSTable Part。
- 当系统重启时，系统应加载已提交 manifest、恢复 catalog、清理未提交临时 Part，并 replay WAL 重建未 flush 数据。
- 当查询指定时间范围时，系统应按时间范围选择相关 shard，并跳过无关 shard。
- 当查询指定 fields 时，系统应只读取相关字段的 ColumnChunk。
- 当 retention 判断 shard 已过期时，系统应以 shard 为单位停止写入、从 manifest 标记删除并异步清理目录。
- 当 compaction 合并多个 Part 时，系统应按 `seriesID -> fieldID -> timestamp` 归并，并保留最大 `writeSeq`。

## 开源实现借鉴

InfluxDB TSM 使用 `header / blocks / index / footer` 单文件结构，block 带 CRC，index 记录 key、类型、时间范围、offset 和 size。它的 WAL + Cache + TSM compaction 路径适合借鉴，尤其是写入先进入 WAL 和内存可查结构，再 flush 为不可变文件，后台 compaction 归并重复点。

Prometheus TSDB 使用 block 目录结构，包含 chunks、index、tombstones 和 meta。index 包含 symbol table、series、postings 和 TOC，chunks 文件保存压缩样本。它是单值 metric 模型，不适合直接照搬，但 symbol table、postings、chunk ref、TOC 和分层索引思路值得借鉴。

VictoriaMetrics 使用 partition、part、block 组织数据。part 目录中分离 `timestamps.bin`、`values.bin`、`index.bin`、`metaindex.bin`，block 按 TSID 和 timestamp 排序，后台 merge 负责压缩、去重和维护。这个结构最接近 `mts` 的目标，但 `mts` 需要在一个 series block 内支持多个 field column，并且第一版使用 CRC32C 进行 block 校验。

## 整体架构

第一版采用正确性优先的嵌入式引擎方案：

```text
internal/engine      打开/关闭引擎，database/rp/shard 路由，跨 shard 查询归并
internal/catalog     measurement+tags 到 seriesID，fieldID/schema，简单标签倒排索引
internal/wal         WAL segment、record 编码、fsync 策略、replay
internal/memtable    有序内存写入结构，LWW，snapshot
internal/sstable     Part 文件格式、block 编解码、metaindex/index reader/writer
internal/compaction  flush、level compaction、重复点归并、过期清理
internal/query       ColumnSeries 读取、MemTable/SSTable 合并、行式适配器
internal/encoding    基础类型编码、CRC32C、后续专用压缩接口
internal/storagefs   原子目录/文件创建、fsync、manifest、权限封装
```

外部 API 采用以下嵌入式库入口：

```go
type Engine interface {
    Open(ctx context.Context, opts Options) error
    Close(ctx context.Context) error
    Write(ctx context.Context, points []Point, opts WriteOptions) error
    QueryColumns(ctx context.Context, q Query) (ColumnIterator, error)
    QueryRows(ctx context.Context, q Query) (RowIterator, error)
}
```

## 数据模型

写入点：

```text
Point
  database
  retentionPolicy
  measurement
  tags: map[string]string
  timestamp
  fields: map[string]FieldValue
```

内部标识：

```text
SeriesKey = measurement + sorted(tags)
SeriesID  = uint64

FieldKey  = measurement + fieldName
FieldID   = uint32
FieldType = float64 | int64 | string | bool
```

字段类型绑定到 `measurement + fieldName`，同一个字段不能在同一 measurement 内写入不同类型。

## 写入路径

```text
Point
  -> validate database/rp/measurement/tags/fields
  -> Catalog 分配 seriesID、fieldID
  -> Engine 按 timestamp 路由到 TimeShard
  -> Shard 分配 writeSeq
  -> WAL append，按配置 fsync
  -> MemTable apply
  -> 达到阈值后 snapshot
  -> flush 成 L0 SSTable Part
```

关键约束：

- WAL append 成功但 MemTable apply 失败时，shard 必须进入受限状态，避免可恢复状态和内存状态分叉。
- 普通写入按默认 WAL 策略确认。
- 强同步写入必须等待对应 WAL segment fsync 完成。
- `writeSeq` 必须写入 WAL，replay 后不能改变 LWW 结果。
- MemTable snapshot 后，新写入进入新的 mutable MemTable。
- snapshot flush 完成并提交 manifest 后，才能 checkpoint 并回收对应 WAL segment。

## 查询路径

```text
Query
  -> Catalog 根据 measurement/tags 解析 seriesID 集合
  -> Engine 根据时间范围选择 shards
  -> 每个 Shard 查询 MemTable + SSTable parts
  -> metaindex 粗筛 part/block
  -> index 精确定位 ColumnChunk
  -> 只读取目标 fields 的 timestamp/value block
  -> 按 writeSeq 合并重复点
  -> 返回 ColumnSeries
  -> 可选转换为 RowIterator
```

查询规则：

- 查询层不读取 WAL，WAL 只服务恢复。
- 查询合并不依赖来源层级，统一按 `(seriesID, fieldID, timestamp) -> max(writeSeq)` 选择结果。
- `QueryColumns` 返回按 `seriesID, fieldID, timestamp` 有序的列式结果。
- `QueryRows` 基于列式结果按 `seriesID, timestamp` 聚合 fields。

## Sharding 与 Retention

第一版使用纯时间分片：

```text
database + retentionPolicy + shard time range
```

每个 shard 独立管理：

- WAL。
- mutable MemTable。
- immutable MemTable snapshots。
- SSTable Parts。
- compaction。

retention 以 shard 为删除边界。当 shard 的 `maxTime < now - retentionDuration` 时：

- 停止该 shard 写入。
- 从 manifest 标记 shard dropped。
- 关闭该 shard 文件句柄。
- 异步删除 shard 目录。
- 第一版不强制清理 catalog 中的 series/field 元信息。

## WAL 设计

每个 time shard 独立 WAL：

```text
shard/
  wal/
    000001.wal
    000002.wal
```

WAL record：

```text
WriteBatchRecord
  database
  retentionPolicy
  measurement
  tags
  seriesID
  fields
  fieldIDs
  timestamp
  writeSeq

DeleteSeriesRecord      预留
DeleteRangeRecord       预留
CheckpointRecord        标记已 flush 的 memtable/snapshot
```

durability 策略：

- `BatchFsync{MaxRecords, MaxBytes, Interval}`。
- 写入可指定 `Sync=true` 强制 fsync。
- segment 达到大小阈值后滚动。
- record 包含 length、version、type、payload、CRC32C。
- replay 遇到尾部半条 record 时允许截断。
- replay 遇到中间损坏时返回 corruption error。

## MemTable 设计

内存结构：

```text
MemTable
  seriesID -> FieldTable

FieldTable
  fieldID -> OrderedSamples

OrderedSamples
  timestamp -> VersionedValue

VersionedValue
  value
  fieldType
  writeSeq
```

规则：

- 相同 `(seriesID, fieldID, timestamp)` 只保留最大 `writeSeq`。
- MemTable 维护内存估算值，超过阈值触发 snapshot。
- snapshot 是只读视图，flush 期间仍参与查询。
- mutable MemTable 与 snapshot MemTable 都参与查询。

## Catalog 设计

第一版 catalog 内置并保持简单，负责：

- `seriesKey -> seriesID`。
- `seriesID -> measurement/tags`。
- `fieldKey -> fieldID/type`。
- `fieldID -> fieldName/type`。
- 简单倒排：`measurement/tagKey/tagValue -> seriesID list`。

持久化策略：

- 使用独立 catalog WAL + snapshot 文件。
- Catalog 更新必须先持久化，再允许数据 WAL 引用新的 `seriesID/fieldID`。
- 数据 WAL 保存 resolved `seriesID/fieldID`，并附带原始 measurement/tags/fieldName 用于恢复校验。
- 允许存在已创建但未写入数据的 series。
- 不允许数据 WAL 引用 catalog 中不存在的 series 或 field。

## SSTable Part 设计

每个 SSTable 是不可变目录：

```text
sst-000001/
  metadata.json
  metaindex.bin
  index.bin
  timestamps.bin
  values.bin
  strings.bin
```

建议目录布局：

```text
data/
  <database>/
    <retention_policy>/
      shards/
        <shard_id>/
          MANIFEST
          l0/
            sst-000001/
              metadata.json
              metaindex.bin
              index.bin
              timestamps.bin
              values.bin
              strings.bin
```

创建权限：

- 目录：`0700`。
- 文件：`0600`。
- 只有确需执行权限的文件才使用 `0700`。

`metadata.json`：

```text
format_version
level
part_id
min_time / max_time
min_series_id / max_series_id
rows_count
series_count
block_count
created_at
encoding_version
crc_manifest
```

`metaindex.bin`：

```text
MetaIndexRow
  minSeriesID
  maxSeriesID
  minTime
  maxTime
  fieldIDs: sorted uint32 list
  indexOffset
  indexSize
  checksum
```

`index.bin`：

```text
IndexBlock
  rows[] IndexRow
  checksum

IndexRow
  seriesID
  minTime / maxTime
  timeRef
  columns[] ColumnRef
```

`timestamps.bin`：

```text
TimeBlock
  encoding
  minTime
  maxTime
  count
  encodedTimestamps
  checksum
```

`values.bin`：

```text
ValueBlock
  fieldID
  fieldType
  encoding
  count
  presenceBitmap
  encodedValues
  writeSeqs
  checksum
```

`strings.bin`：

- 第一版使用 length-prefixed bytes。
- 格式预留 block-local dictionary encoding。
- 大字符串超过阈值时使用外置引用。

## SeriesBlock 与 ColumnChunk

flush 的逻辑写入单元是 `SeriesBlock`：

```text
SeriesBlock
  seriesID
  timestamps[]
  columns:
    fieldID
    type
    presenceBitmap
    values[]
    writeSeq[]
```

一个 `SeriesBlock` 写成：

- 一份 `TimeBlock`。
- 多份 `ValueBlock`。
- 一条 `IndexRow`。
- 若干 `MetaIndexRow`。

设计收益：

- 同一 series 的多个字段共享 timestamp。
- 查询单个字段时只读取该字段的 value block。
- 后续可对 timestamp、不同字段类型分别优化编码。

建议块大小：

- `SeriesBlock` 目标点数：`1024-8192` 个 timestamp。
- 单个压缩前 block 最大：`256KB-1MB`。
- `IndexBlock` 最大：`64KB-256KB`。
- L0 Part 大小：`32MB-128MB` 可配置。

## 编码与校验

第一版采用分阶段压缩策略：

- 文件格式从第一天预留 `encoding` 字段。
- 第一版先实现简单可靠的类型基础编码 + CRC32C。
- 后续逐步引入类型专用压缩。

后续目标编码：

- timestamp：delta-of-delta。
- `float64`：Gorilla XOR 或 XOR2 风格编码。
- `int64`：delta、zigzag、bitpacking。
- `bool`：bitset 或 RLE。
- `string`：block-local dictionary。

兼容规则：

- 每个 block 使用 CRC32C。
- `metadata.json` 记录格式版本。
- 读端遇到未知 encoding 返回明确错误。
- 写入 Part 先写临时目录，fsync 后通过 manifest 原子注册。

## Compaction 设计

第一版采用简化 leveled compaction：

```text
L0: flush 产生的小 Part，时间范围可能重叠
L1+: compaction 后的 Part，同一 level 内尽量保证 series/time 范围不重叠
```

compaction 输入：

- 多个 L0 Part。
- 同 level 小 Part。
- 包含大量重复点或墓碑的 Part。

compaction 输出：

- 新临时 Part。
- 完成写入、校验、fsync 后注册到 manifest。
- 将旧 Part 标记为 obsolete。
- 异步安全清理旧目录。

合并规则：

- 按 `seriesID -> fieldID -> timestamp` 归并。
- 相同点保留最大 `writeSeq`。
- 输出按 `seriesID, minTime` 排序生成 SeriesBlock。
- 输出 Part 的 block index 重新构建。
- 查询通过 manifest view/snapshot 控制可见性，避免新旧 Part 重复影响结果。

## 恢复流程

启动流程：

```text
open engine
  -> load manifest
  -> load catalog snapshot + catalog WAL
  -> open each shard
  -> load committed SSTable parts
  -> delete uncommitted temp parts
  -> replay shard WAL records newer than checkpoint
  -> rebuild mutable/snapshot memtables
  -> resume flush/compaction scheduler
```

一致性规则：

- `writeSeq` 从 engine 元数据恢复，必须大于已持久化最大值。
- checkpoint 只能在 SSTable Part 已 fsync 且 manifest 已提交后写入。
- WAL segment 删除必须晚于 checkpoint 和 manifest 提交。
- 如果 catalog 已有 series 但数据 WAL 未写成功，允许存在空 series。
- 如果数据 WAL 引用缺失 series，恢复必须失败并返回 corruption error。

## 错误处理

- 所有返回错误必须显式处理。
- 明确无关紧要的错误也必须用 `_` 显式忽略。
- 文件格式错误、CRC 错误、未知 encoding、catalog 缺失引用必须返回明确错误。
- WAL 尾部半条 record 属于可恢复场景，应截断并继续。
- WAL 中间损坏属于数据损坏，应停止恢复并返回错误。
- flush 或 compaction 失败不得影响已提交 manifest 中的旧 Part。

## 测试验收

第一版测试至少覆盖：

- WAL：record 编解码、CRC、尾部截断、fsync 策略、segment rollover、replay。
- MemTable：有序写、乱序写、LWW、snapshot、查询合并。
- Catalog：seriesID 分配、field type 冲突、倒排查询、恢复。
- SSTable：Part 写入/读取、block CRC、unknown encoding、字段裁剪、string 编码。
- Flush：MemTable snapshot 到 L0 Part，WAL checkpoint。
- Compaction：多 Part 合并、重复点 LWW、输出顺序、manifest 原子切换。
- Recovery：正常关闭、崩溃模拟、半写 WAL、临时 Part 清理。
- Retention：过期 shard 删除、查询自动跳过 dropped shard。
- Cross-shard query：多 shard 结果归并。
- Coverage：项目整体代码行覆盖率目标 `>=90%`。

代码完成后需要执行：

- `go test ./... -timeout <bounded>`。
- `golangci-lint`。
- `goimports-reviser`。
- 若存在 `tests/e2e`，需要按项目要求执行全部 e2e 用例，并清理临时构建产物。

## 设计决策记录

- 第一版选择嵌入式存储引擎库，服务层后置。
- 数据模型选择多值点模型，而不是 Prometheus 风格单值样本模型。
- MemTable 选择有序 map 结构，优先正确性和可测试性。
- Sharding 选择纯时间分片，优先 retention 和时间范围查询。
- SSTable 选择目录式 Part，借鉴 VictoriaMetrics 的分离 timestamp/value/index 文件结构。
- 查询返回同时支持底层列式和上层行式适配。
- 压缩策略分阶段推进，第一版预留 encoding 并先保证正确性。

## 实现计划细化项

以下内容不改变本规格的设计方向，将在 implementation plan 中拆成可验收任务：

- 乱序窗口的默认时间范围和配置项命名。
- shard duration 的默认值和与 retention policy 的关系。
- MemTable ordered map 的具体实现。
- Manifest 文件格式和原子提交细节。
- Catalog snapshot 周期和 compaction 规则。
- 第一版基础编码的精确二进制格式。
- benchmark 指标和数据集规模。
