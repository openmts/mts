# mts Storage Engine Phase 2 Design

## 背景

当前 `mts` 存储层已经完成第一版可运行垂直切片：公共嵌入式 API、Catalog、WAL、MemTable、SSTable Part、Manifest、Flush、Query、Compaction、Retention、e2e 与 pprof smoke 均可运行。该版本适合作为功能骨架，但还不能称为完善或性能最优。

Phase 2 的目标是在继续建设上层能力之前，先把存储层关键路径做硬化：崩溃一致性更清晰、SSTable/WAL 编码更接近生产形态、查询能通过索引减少无效读取、compaction 有策略、有测试、有性能基线。

## 范围

### 本阶段包含

- WAL 二进制 batch payload 与 replay 性能改进。
- SSTable v2 二进制 block encoding，覆盖 `float64`、`int64`、`string`、`bool` 四种字段类型。
- SSTable metaindex/index 增强，使查询能按 time、series、field 剪枝。
- Manifest 原子提交与 flush/compaction 崩溃恢复语义硬化。
- Size-tiered compaction 策略、后台调度接口和手动触发兼容。
- Retention 删除流程与 compaction/flush 的互斥边界。
- Benchmark 与 pprof 基线，覆盖写入、查询、flush、compaction、replay。
- e2e 用例覆盖崩溃恢复、数据完整性、compaction、retention、查询正确性。

### 本阶段不包含

- SQL/InfluxQL/PromQL 查询语言。
- 网络服务层、gRPC/HTTP API、认证授权。
- 分布式复制、Raft、集群分片迁移。
- 多租户配额、上层 database/measurement 管理 API。
- 高级压缩算法的最终形态；本阶段只实现稳定的二进制编码和可插拔 encoding 标识。

## 目标

1. 存储层在单机嵌入式模式下具备明确、可测试的崩溃恢复语义。
2. 读写路径去掉主要 JSON 热点，减少 CPU 与分配压力。
3. 查询能通过 Part 元数据、metaindex 和 index 跳过无关数据块。
4. Compaction 从手动全量合并升级为可配置策略。
5. 每次性能相关改动都有 benchmark 或 pprof 证据。

## 非目标

- 不承诺“绝对最优性能”。Phase 2 的验收方式是相对基线的可测改进，以及结构上允许后续继续优化。
- 不引入复杂外部依赖。生产代码仍优先使用标准库，除非某个依赖能显著降低风险并经单独确认。
- 不改变用户可见的数据模型：仍只支持 `float64`、`int64`、`string`、`bool`。

## 需求

### Durability 与 WAL

- When `Write` 返回成功时，系统 shall 已将对应 batch 按配置的 durability 策略写入 WAL。
- When `WriteOptions.Sync=true` 时，系统 shall 在返回成功前完成 WAL fsync。
- When WAL 最后一条 record 不完整时，系统 shall 在 replay 时截断尾部半条 record 并保留之前完整数据。
- When WAL 中间 record 损坏或 CRC 不匹配时，系统 shall 返回错误，不得静默跳过。
- When MemTable flush 成功提交 manifest 后，系统 shall 截断对应 shard 的 WAL。
- If flush 写入 Part 成功但 manifest 未提交，系统 shall 在重启后不暴露该未提交 Part。
- If manifest 已提交但 WAL 截断前崩溃，系统 shall 在重启 replay 后仍保持 Last Write Wins，不产生错误重复结果。

### SSTable v2 Encoding

- When 写入 SSTable Part 时，系统 shall 使用版本化 block encoding，并在 metadata 中记录 format version 与 encoding。
- When 字段类型是 `float64` 时，系统 shall 使用二进制 float block encoding 存储值。
- When 字段类型是 `int64` 时，系统 shall 使用二进制 int block encoding 存储值。
- When 字段类型是 `bool` 时，系统 shall 使用紧凑 bool block encoding 存储值。
- When 字段类型是 `string` 时，系统 shall 使用 string table 或 length-prefixed encoding 存储值。
- When 读取旧 format version 的 Part 时，系统 shall 明确返回 unsupported format 错误，除非实现了兼容读取器。
- When block CRC 校验失败时，系统 shall 返回错误，不得返回部分结果。

### Index 与 Query Pruning

- When query 指定 time range 时，系统 shall 先通过 Part metadata 和 metaindex 判断是否可能命中。
- When query 指定 series filter 时，系统 shall 跳过 series 范围不相交的 index row。
- When query 指定 field filter 时，系统 shall 跳过不包含目标 field 的 value block。
- When query 只命中少数 series/field 时，系统 should 避免读取无关 value block。
- When query 合并 MemTable 与多个 Part 时，系统 shall 对同一 `seriesID+fieldID+timestamp` 使用最大 `writeSeq` 保留最新值。
- When query 返回 rows 时，系统 shall 保持稳定排序：先 `seriesID`，再 `timestamp`。

### Manifest 与崩溃一致性

- When 写 manifest 时，系统 shall 使用临时文件、fsync 文件、rename、fsync 目录的顺序完成原子替换。
- When 打开 shard 时，系统 shall 只加载 manifest 中引用的 Part。
- When shard 目录存在孤儿 Part 时，系统 shall 不暴露它；清理可以延迟到后续维护流程。
- When manifest 损坏时，系统 shall 返回明确错误，阻止继续打开 shard。
- When compaction 写入新 Part 成功但 manifest 未提交时，系统 shall 在重启后继续使用旧 Part。
- When compaction manifest 提交成功后，系统 shall 允许删除旧 Part；删除失败 shall 返回错误并保留可恢复状态。

### Compaction

- When shard 的 level-0 Part 数量或总大小超过配置阈值时，系统 shall 触发 size-tiered compaction。
- When compaction 运行时，系统 shall 与 flush、retention 删除同一 shard 的操作互斥。
- When compaction 合并多个 Part 时，系统 shall 保持 LWW 语义。
- When compaction 结束后，系统 shall 更新 manifest，使查询只读取当前有效 Part 集合。
- If compaction 失败，系统 shall 保持旧 manifest 可用，不得丢失数据。

### Retention

- When retention cutoff 早于 shard end time 时，系统 shall 保留该 shard。
- When shard end time 早于 cutoff 时，系统 shall 关闭 shard 资源并删除 shard 目录。
- When retention 与 compaction/flush 同时触发时，系统 shall 通过 shard 生命周期锁避免删除正在写入的目录。
- If retention 删除失败，系统 shall 返回错误并保留内存中的 shard 状态。

### Benchmark 与 Pprof

- When 改动 WAL encoding、SSTable encoding、query merge 或 compaction 时，系统 shall 提供对应 benchmark 或 pprof smoke 结果。
- When benchmark 运行时，系统 shall 报告 `ns/op`、`B/op`、`allocs/op`，并记录数据规模。
- When pprof workload 生成 profile 时，系统 shall 以 `0600` 权限创建 profile 文件，并在测试后清理临时产物。
- When 性能优化声称提升时，系统 shall 基于前后对比数据说明，不得只凭直觉。

## 架构设计

### WAL v2

WAL 保留现有 frame 外壳：length、version、record type、payload、CRC32C。Phase 2 改造 payload：从 JSON 编码切换为内部二进制 batch encoding。batch payload 包含 record count，每条 point 包含 database/policy/measurement/tag 已解析后的 seriesID、timestamp、writeSeq 和 fields。字段值按类型编码，避免 JSON marshal/unmarshal 热点。

### SSTable v2

Part 仍采用目录式结构，保留 `metadata`、`metaindex`、`index`、`timestamps`、`values`、`strings` 分文件边界。Phase 2 将 block payload 从 JSON 改为二进制：

- timestamp block：按 int64 序列编码，预留 delta encoding 标识。
- float64 block：按 little-endian float64 编码。
- int64 block：按 little-endian int64 编码。
- bool block：bitset 或 byte-packed 编码。
- string block：offset table + bytes，或 length-prefixed bytes。

所有 block 继续使用 CRC32C framing。metadata 中记录 format version，便于未来兼容 v1/v2。

### Query Path

查询路径按三层剪枝执行：

1. Engine 根据 shard time range 跳过不相交 shard。
2. Part 根据 metadata/metaindex 跳过不相交 Part 或 index block。
3. Index row 根据 series/time/field 跳过无关 value block。

读取命中的 value block 后，再按时间过滤 sample。最终在 shard 或 engine 层进行 LWW 合并。

### Compaction Strategy

Phase 2 使用 size-tiered compaction 起步。每个 shard 维护 CompactionOptions：

- `Level0PartLimit`
- `Level0SizeLimit`
- `MaxOutputPartBytes`
- `BackgroundInterval`
- `Enabled`

手动 `Compact` 保留，并调用同一策略实现。后台调度只负责触发，不改变 compaction 的一致性语义。

### Locking 与生命周期

Engine 保留全局 shard map 锁，但 shard 内增加生命周期互斥，确保 flush、compaction、retention delete 不并发破坏目录状态。后续可以再细化为读写锁和 per-shard 锁；Phase 2 先以正确性优先。

## 测试设计

### Unit Tests

- WAL v2 encode/decode、CRC、tail truncation、中间损坏。
- SSTable v2 各字段类型 block encode/decode。
- Manifest 原子写、损坏 manifest、孤儿 Part 不可见。
- Query pruning：field/time/series 不命中时不读取 value block。
- Compaction LWW、失败回滚、旧 Part 清理失败。

### E2E Tests

- `tests/e2e/wal_recovery`：写入未 flush 后重启，验证数据完整。
- `tests/e2e/flush_manifest_recovery`：模拟 manifest 提交前后状态，验证可见性。
- `tests/e2e/compaction_integrity`：多 Part 重复写入后 compaction，验证 LWW。
- `tests/e2e/retention`：多 shard 写入后按 cutoff 删除。
- `tests/e2e/query_pruning`：大量 series/field 下查询少量目标，验证结果正确。

### Benchmarks

- `BenchmarkWALAppendReplay`
- `BenchmarkSSTableWritePart`
- `BenchmarkSSTableQueryPointLookup`
- `BenchmarkEngineWriteBatch`
- `BenchmarkEngineQueryRows`
- `BenchmarkShardCompaction`

基线保存在文档中，性能优化提交需要说明与基线的对比。

## 验收标准

- `go test ./... -coverprofile=coverage.out -timeout 600s` 通过，总覆盖率 `>=90%`。
- `golangci-lint run --timeout 12m` 通过。
- `goimports-reviser -project-name codeberg.org/mts/mts -recursive -format -rm-unused .` 通过。
- 所有新增 e2e 用例可以按 `cd tests/e2e/<case> && go build && ./<binary>` 方式运行通过。
- `tests/pprof/storage_engine` 能生成 CPU 和 heap profile，运行后无残留产物。
- 对 WAL/SSTable/query/compaction 的核心改动有 benchmark 或 pprof 结果。
- 新增目录权限为 `0700`，新增文件权限为 `0600`。

## 风险与权衡

- 二进制 encoding 会提高性能，但会增加格式兼容复杂度；通过 format version 和集中 codec 包控制风险。
- Compaction 后台调度会引入并发风险；Phase 2 先使用保守锁策略，性能优化在正确性稳定后再做。
- 旧 v1 JSON Part 是否兼容读取会增加实现成本；当前仓库还未发布稳定版本，Phase 2 可以选择不兼容旧 Part，但必须明确错误。
- “性能最优”不可一次性达成；本阶段以可测基线和显著去 JSON 热点作为目标。

## 退出条件

完成 Phase 2 后，存储层可以作为上层 metadata、database/measurement 管理、服务层 API 的稳定依赖。若 Phase 2 验收未通过，不进入上层实现。
