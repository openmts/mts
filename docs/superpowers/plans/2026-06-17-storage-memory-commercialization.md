# Storage Memory Commercialization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将专项 2 的样本数阈值升级为字节级存储内存治理，控制 MemTable、写入、flush、query 和 compaction 的主要可控内存路径。

**Architecture:** 在 `internal/memtable` 提供容量级内存估算，在 `internal/engine` 新增 `storageMemoryLimiter` 统一管理 soft/hard bytes 和临时 reservation。Engine 写入前估算 incoming bytes，写入后按 MemTable estimated bytes 触发 flush 或拒写；flush/query/compaction 使用命名 reservation 约束临时内存。

**Tech Stack:** Go、现有 mts engine/memtable/queryexec/sstable、TDD、`go test`、`goimports-reviser`、`golangci-lint`。

---

## Task 1: MemTable 字节级估算

**预计耗时:** 45m  
**硬超时:** 90m  
**下一次进度更新时间:** 开始后 30 分钟内

**Files:**
- Modify: `internal/memtable/memtable.go`
- Modify: `internal/memtable/memtable_test.go`
- Modify: `internal/engine/ports.go`

**EARS:**
- When MemTable 已写入 float/int/string/bool 样本时，系统应按 slice 容量估算 timestamps、writeSeq、value buffer 和字符串内容字节数。
- When MemTable snapshot reset 后，系统应将 active MemTable estimated bytes 降低到空表水平。
- When buffer 容量扩展时，估算值应按 capacity 增长，而不是按 len 增长。

**Steps:**
- [x] 写失败测试 `TestMemTableApproxMemoryBytesAccountsForColumnBuffersAndStrings`。
- [x] 实现 `MemTable.ApproxMemoryBytes()`、`Snapshot.ApproxMemoryBytes()` 和 `columnBuffer.approxMemoryBytes()`。
- [x] 将 `memStore.ApproxMemoryBytes()` 加入 engine 端口。
- [x] 运行 `go test ./internal/memtable ./internal/engine -run 'TestMemTableApproxMemoryBytes|TestMemTableStore' -timeout 180s`。

## Task 2: StorageMemoryOptions 字节阈值与错误类型

**预计耗时:** 45m  
**硬超时:** 90m  
**下一次进度更新时间:** 开始后 30 分钟内

**Files:**
- Modify: `internal/model/types.go`
- Modify: `types.go`
- Modify: `types_test.go`
- Modify: `internal/engine/paths.go`
- Create: `internal/engine/memory_budget.go`
- Create: `internal/engine/memory_budget_test.go`

**EARS:**
- When 用户配置 `SoftBytesLimit` 时，系统应在 estimated bytes 达到软阈值时触发 flush。
- When 用户配置 `HardBytesLimit` 时，系统应在预计写入后超过硬阈值时拒绝写入。
- When 用户配置 query/flush/compaction 子预算时，系统应为对应临时内存路径独立校验。
- When 内存被拒绝时，系统应返回可识别的 `ErrStorageMemoryLimitExceeded`。

**Steps:**
- [x] 写失败测试覆盖 option DTO、normalize 和错误识别。
- [x] 实现 `StorageMemoryOptions` 字节字段和 `toModelStorageMemoryOptions` 映射。
- [x] 实现 `storageMemoryLimiter`、`Reserve/Release/Snapshot` 和错误类型。
- [x] 运行 `go test ./internal/engine . -run 'TestStorageMemory|TestNormalizeStorageMemory|TestStorageMemoryLimiter' -timeout 180s`。

## Task 3: 写入路径字节预算

**预计耗时:** 60m  
**硬超时:** 120m  
**下一次进度更新时间:** 开始后 30 分钟内

**Files:**
- Modify: `internal/engine/engine.go`
- Modify: `internal/engine/shard.go`
- Modify: `internal/engine/engine_test.go`

**EARS:**
- When incoming batch estimated bytes 超过 hard limit 时，系统应在 catalog resolve 后、MemTable apply 前拒绝写入。
- When current MemTable estimated bytes 加 incoming bytes 达到 soft limit 时，系统应先 flush 再写入。
- When 写入后 active MemTable estimated bytes 达到 soft limit 时，系统应 flush 所有 shard。
- When flush 后仍超过 hard limit 时，系统应返回内存限制错误。

**Steps:**
- [x] 写失败测试 `TestEngineStorageMemoryHardBytesLimitRejectsBeforeApply`。
- [x] 写失败测试 `TestEngineStorageMemorySoftBytesLimitFlushesAfterWrite`。
- [x] 实现 resolved point incoming bytes 估算和 active MemTable bytes 汇总。
- [x] 运行 `go test ./internal/engine -run 'TestEngineStorageMemory.*Bytes' -timeout 180s`。

## Task 4: Flush、Query、Compaction 临时预算

**预计耗时:** 90m  
**硬超时:** 180m  
**下一次进度更新时间:** 开始后 30 分钟内

**Files:**
- Modify: `internal/engine/shard.go`
- Modify: `internal/engine/query.go`
- Modify: `internal/engine/lifecycle.go`
- Modify: `internal/queryexec/budget.go`
- Modify: `internal/engine/engine_test.go`

**EARS:**
- When flush snapshot estimated bytes 超过 flush budget 时，系统应返回内存限制错误并恢复 MemTable。
- When query result samples 估算超过 query budget 时，系统应停止 stream 并返回 query memory 错误。
- When compaction output batch estimated bytes 超过 compaction budget 时，系统应停止本次 compaction，保留输入 part。
- When reservation release 被调用时，系统应归还临时内存额度，不影响后续操作。

**Steps:**
- [x] 写失败测试覆盖 flush budget 恢复 MemTable。
- [x] 写失败测试覆盖 query memory budget。
- [x] 写失败测试覆盖 compaction reservation 错误不会提交 manifest。
- [x] 在 shard flush、engine query、streaming compaction 输出处接入 reservation。
- [x] 运行 `go test ./internal/engine ./internal/queryexec -run 'Test.*Memory.*Budget|Test.*Compaction.*Memory' -timeout 240s`。

## Task 5: Metrics 快照、文档与最终验证

**预计耗时:** 45m  
**硬超时:** 90m  
**下一次进度更新时间:** 开始后 30 分钟内

**Files:**
- Modify: `internal/model/types.go`
- Modify: `types.go`
- Modify: `tests/pprof/storage_engine/main.go`
- Modify: `docs/superpowers/plans/2026-06-17-storage-memory-commercialization.md`

**EARS:**
- When 用户采集 storage memory snapshot 时，系统应暴露 current bytes、peak bytes、soft/hard limits、reservation bytes、rejected writes 和 flush triggered。
- When pprof storage_engine 运行时，系统应输出 storage memory 相关配置和快照。
- When 验证完成时，系统应清理 profile、coverage、二进制和临时数据目录。

**Steps:**
- [x] 实现 storage memory snapshot API。
- [x] 将 pprof 用例参数暴露 `-storage-soft-bytes-limit`、`-storage-hard-bytes-limit`。
- [x] 运行 `go test ./... -timeout 10m`。
- [x] 运行 `goimports-reviser -rm-unused -format ./...`。
- [x] 运行 `golangci-lint run --timeout 10m`。
- [x] 清理临时产物并更新本计划勾选状态。

**实现备注：**
- MemTable 新增容量级 `ApproxMemoryBytes()`，按 slice capacity、字符串内容和列缓冲估算。
- StorageMemoryOptions 新增 `SoftBytesLimit`、`HardBytesLimit`、`QueryBytesLimit`、`FlushBytesLimit`、`CompactionBytesLimit`。
- Engine 写入前按 incoming bytes 拒写或预 flush，写入后按 active MemTable bytes 触发 flush。
- Flush snapshot、QueryColumns 物化、streaming compaction 输出均接入字节预算。
- 新增 `ErrStorageMemoryLimitExceeded` 和 `StorageMemorySnapshot()`，pprof 用例已输出 storage memory 快照字段。
- 验证命令 `go test ./... -timeout 10m`、`goimports-reviser -rm-unused -format ./...`、`golangci-lint run --timeout 10m` 均通过。

## Task 6: 专项二闭环补充 - WAL、Compression、Source Metrics 与 Iterator Budget

**预计耗时:** 120m  
**硬超时:** 240m  
**下一次进度更新时间:** 开始后 30 分钟内

**Files:**
- Modify: `internal/model/types.go`
- Modify: `types.go`
- Modify: `internal/engine/memory_budget.go`
- Modify: `internal/engine/engine.go`
- Modify: `internal/engine/shard.go`
- Modify: `internal/engine/query.go`
- Modify: `internal/engine/ports.go`
- Modify: `internal/wal/wal.go`
- Modify: `internal/sstable/write.go`
- Modify: `internal/sstable/compression.go`
- Modify: `internal/sstable/payload_compression.go`
- Create: `internal/engine/metrics.go`
- Modify: `tests/pprof/storage_engine/main.go`
- Modify: `tests/pprof/storage_engine/main_test.go`
- Modify: `internal/engine/engine_test.go`
- Modify: `internal/engine/memory_budget_test.go`

**EARS:**
- When WAL batch buffer 占用内存时，系统应将 pending records、encoded bytes 和 write frame 临时字节纳入存储内存预算。
- When payload compression 产生压缩目标和工作区时，系统应通过 `CompressionBytesLimit` 限制临时 buffer。
- When 查询使用 iterator 而不是 `QueryColumns` 物化时，系统仍应在 raw column stream 层执行 query bytes budget。
- When storage memory manager 统计内存时，系统应拆分 MemTable、WAL、write、flush、query、compaction、compression 来源。
- When RSS 与引擎估算内存存在差距时，系统应暴露 runtime heap、RSS 和 gap 指标。
- When metrics 被采集时，系统应输出低基数字段的 storage memory 指标。

**Steps:**
- [x] 写失败测试覆盖 source breakdown、compression reservation reject、WAL pending snapshot、query iterator budget 和 storage memory metrics。
- [x] 扩展 `StorageMemoryOptions`/`StorageMemorySnapshot`，增加 `CompressionBytesLimit`、source bytes、runtime bytes 和 rejected reservation 计数。
- [x] 扩展 WAL 端口和 `wal.Log.ApproxMemoryBytes()`，把 pending WAL frame 字节纳入 active bytes。
- [x] 在 `Shard.WriteBatch` 中对 WAL frame 编码临时内存做 write reservation。
- [x] 在 SSTable payload compression 中接入可选 `CompressionMemoryBudget`，snappy/lz4/zstd 写入压缩前申请 compression reservation。
- [x] 在 query raw `ColumnDataStream` 层接入 query memory budget，保持 `Next()` 不触发 catalog decoration。
- [x] 新增 Engine storage memory metrics 映射，pprof 输出 source breakdown 和 runtime gap。
- [x] 运行 `go test ./internal/engine ./internal/sstable ./tests/pprof/storage_engine . -timeout 5m`。

**实现备注：**
- WAL pending bytes 通过 `wal.Log.ApproxMemoryBytes()` 暴露，Engine active bytes 由 `MemTableBytes + WALBytes` 组成。
- 写入路径对 WAL frame 临时编码字节使用 `storageMemoryWrite` reservation；flush/compaction 仍使用各自临时预算。
- SSTable `WriteOptions` 新增小接口 `CompressionMemoryBudget`，保持 `sstable` 不依赖 `engine`。
- Query iterator budget 移到 raw column stream 层，避免预算检查提前触发 `Column()` 装饰。
- `StorageMemorySnapshot` 新增 MemTable/WAL/reservation source、runtime heap/RSS/gap 和 rejected reservation 指标。
