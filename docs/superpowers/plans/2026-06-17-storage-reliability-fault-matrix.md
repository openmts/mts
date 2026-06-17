# Storage Reliability Fault Matrix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完善专项一可靠性与异常故障矩阵，使 WAL、Manifest、SSTable、Flush、Compaction、Retention、Recovery 的关键失败路径可注入、可恢复、可诊断。

**Architecture:** 以 `storagefs` 作为统一文件故障边界，补齐短写、ENOSPC 和可分类持久化错误；在 `engine` 打开 shard 时执行恢复审计，区分可清理孤儿 part 与致命 Manifest/metadata 不一致；扩展 `tests/fault/storage_fault_matrix` 为机器可读矩阵，每个用例都在故障后执行重启恢复和查询一致性验证。

**Tech Stack:** Go、现有 `internal/storagefs`、`internal/faultinject`、`internal/engine`、`internal/sstable`、`internal/wal`、`tests/fault/storage_fault_matrix`、TDD、`go test`、`goimports-reviser`、`golangci-lint`。

---

## Task 1: 文件系统故障语义与短写检测

**预计耗时:** 60m  
**硬超时:** 120m  
**下一次进度更新时间:** 开始后 30 分钟内

**Files:**
- Modify: `internal/storagefs/fs.go`
- Modify: `internal/storagefs/internal_test.go`
- Modify: `internal/faultinject/fs.go`
- Modify: `internal/faultinject/fs_test.go`

**EARS:**
- When 文件系统出现短写时，系统应检测写入字节不足并返回结构化错误。
- When 磁盘返回 ENOSPC 时，系统应返回磁盘空间不足错误，并避免 Manifest 指向未完整 part。
- When 文件权限不足导致目录或文件创建失败时，系统应返回 operation、path 和底层错误。

**Steps:**
- [x] 写失败测试：`storagefs.WriteFull` 遇到短写时返回 `ErrShortWrite`，错误包含 operation 和 path。
- [x] 写失败测试：faultinject 支持 `ShortWriteNext` 和 `FailNext(OpWrite, syscall.ENOSPC)`。
- [x] 实现 `storagefs.OpError`、`ErrShortWrite`、`IsNoSpace(err)`。
- [x] 将 WAL block 写入、atomic file 写入、manifest/metadata 写入统一改为 full-write 检测。
- [x] 运行 `go test ./internal/storagefs ./internal/faultinject -run 'Test.*ShortWrite|Test.*NoSpace|Test.*OpError' -timeout 180s`。

**实现备注：**
- 新增 `storagefs.WriteFull`、`OpError`、`ShortWriteError`、`ErrShortWrite`、`IsNoSpace`，统一保留 operation、path 和底层错误链。
- `storagefs.WriteFileAtomic`、WAL frame、catalog WAL、SSTable block 写入均改为完整写检测。
- `faultinject.FS` 新增 `ShortWriteNext` 和 `BeforeWrite`，可同时用于直接 FS 测试和 `storagefs.SetFaultController`。
- 将 catalog、manifest、engine 的缺失文件判断改为 `errors.Is(err, os.ErrNotExist)`，保持 wrapped error 可识别。
- 验证：`go test ./internal/storagefs ./internal/faultinject -run 'Test.*ShortWrite|Test.*NoSpace|Test.*ENOSPC|Test.*OpError' -timeout 180s` 通过；`go test ./internal/storagefs ./internal/faultinject ./internal/wal ./internal/catalog ./internal/sstable ./internal/engine -timeout 240s` 通过。

## Task 2: Shard 恢复审计与 Manifest 一致性

**预计耗时:** 90m  
**硬超时:** 180m  
**下一次进度更新时间:** 开始后 30 分钟内

**Files:**
- Create: `internal/engine/recovery_audit.go`
- Create: `internal/engine/recovery_audit_test.go`
- Modify: `internal/engine/shard.go`
- Modify: `internal/sstable/types.go`
- Modify: `internal/sstable/read.go`

**EARS:**
- When 恢复流程发现孤儿 part 时，系统应清理可安全删除的文件，并记录清理结果。
- When 恢复流程发现 Manifest 引用缺失 part 时，系统应返回致命恢复错误，而不是静默丢数据。
- When 恢复流程发现 part metadata 与 Manifest 元数据不一致时，系统应返回结构化一致性错误。
- When 恢复流程发现临时 Manifest 或临时 part 输出时，系统应按安全规则清理或记录。

**Steps:**
- [x] 写失败测试：Manifest 引用缺失 part 时 `OpenShard` 返回 `ErrRecoveryFatal`。
- [x] 写失败测试：Manifest 中 PartMeta 与磁盘 metadata 的 id/level/time/series/row count 不一致时返回一致性错误。
- [x] 写失败测试：孤儿 part 被清理后 `MaintenanceErrors` 暴露清理结果。
- [x] 实现 `RecoveryIssue`、`RecoveryReport`、`ErrRecoveryFatal`。
- [x] 在 `OpenShard` 加载 part 前运行 manifest 引用校验，加载 part 后校验 metadata。
- [x] 运行 `go test ./internal/engine -run 'TestShardRecovery|TestOpenShard.*Manifest|TestOpen.*Recovery' -timeout 240s`。

**实现备注：**
- 新增 `internal/engine/recovery_audit.go`，用 `RecoveryIssue` 区分 missing part、part open failed、metadata mismatch、orphan/temp cleanup 等恢复问题。
- `OpenShard` 打开 Manifest part 失败时返回可 `errors.Is(err, ErrRecoveryFatal)` 识别的 fatal issue。
- 打开 part 后校验 manifest 元数据与磁盘 metadata 的 id、level、时间范围、series 范围、row/series/block count、max write seq。
- 孤儿 part 与 `.tmp-*` 临时文件清理结果进入 `RecoveryReport`，并通过 `MaintenanceErrors` 暴露非 fatal maintenance issue。
- 新增 `Engine.RecoveryReports` 和 `Shard.RecoveryReport` 供后续运维观测读取结构化恢复报告。
- 验证：`go test ./internal/engine -run 'TestOpenShardReturnsRecoveryFatal|TestOpenShardRecordsMaintenanceIssue|TestOpenShardCleansOrphanParts' -timeout 240s` 通过；`go test ./internal/engine -timeout 240s` 通过。

## Task 3: Flush 和 Compaction 失败原子性增强

**预计耗时:** 120m  
**硬超时:** 240m  
**下一次进度更新时间:** 开始后 30 分钟内

**Files:**
- Modify: `internal/engine/shard.go`
- Modify: `internal/engine/lifecycle.go`
- Modify: `internal/engine/engine_test.go`
- Modify: `tests/fault/storage_fault_matrix/main.go`

**EARS:**
- When SSTable part 写入任一组件失败时，系统应删除未提交 part 或标记为孤儿，Manifest 不应引用它。
- When Manifest fsync/rename 失败时，系统应返回明确错误，并不得 checkpoint WAL。
- When Compaction 输出 part 写失败或 Manifest 切换失败时，系统应保留输入 parts 可读，并清理未引用输出 part。
- When Compaction Manifest 切换成功但删除旧 part 失败时，系统应保持新 Manifest 可读，并将旧 part 作为可清理垃圾暴露给 maintenance。

**Steps:**
- [x] 写失败测试：flush part 写失败后目录不包含 manifest 未引用的半成品 part。
- [x] 写失败测试：flush manifest 写失败后 WAL 未 checkpoint，重启可 replay。
- [x] 写失败测试：compaction manifest 写失败后输入 part 仍可查询，输出 part 被清理。
- [x] 写失败测试：compaction 删除旧 part 失败后新 manifest 可读，maintenance 记录旧 part 清理失败。
- [x] 实现 flush/compaction 失败清理与 maintenance issue 记录。
- [x] 运行 `go test ./internal/engine -run 'Test.*Flush.*Failure|Test.*Compaction.*Failure|Test.*Recovery' -timeout 240s`。

**实现备注：**
- flush 在 part 已写但 manifest 未提交的任一失败路径会关闭并删除未提交 part；删除失败进入 `RecoveryIssueOrphanRemoveFailed`。
- flush manifest 写失败时不会 checkpoint WAL，并会移除新 part，保证重启仍可通过 WAL replay 恢复。
- compaction manifest 写失败会关闭新输出 part 并删除输出目录，输入 parts 保持可查询。
- compaction manifest 切换成功后，旧 part 删除失败不再让已切换的新 manifest 回滚；删除失败作为 maintenance issue 记录，查询继续走新 part。
- 验证：`go test ./internal/engine -run 'TestFlushManifestFailure|TestCompactionManifestFailure|TestCompactionOldPartDeleteFailure|TestFlushFailureBeforeManifest' -timeout 240s` 通过；`go test ./internal/engine -timeout 240s` 通过。

## Task 4: WAL replay 和 checkpoint 矩阵闭环

**预计耗时:** 90m  
**硬超时:** 180m  
**下一次进度更新时间:** 开始后 30 分钟内

**Files:**
- Modify: `internal/wal/wal.go`
- Modify: `internal/wal/wal_test.go`
- Modify: `tests/fault/storage_fault_matrix/main.go`

**EARS:**
- When WAL append 返回错误时，系统应拒绝写入，并保证 MemTable 不应用该批数据。
- When WAL fsync 失败且请求要求同步时，系统应返回明确错误，并不得声明已持久化。
- When WAL replay 遇到截断尾部记录时，系统应跳过未完整尾部记录并保留此前完整记录。
- When WAL replay 遇到 checksum 不匹配的中间记录时，系统应停止恢复并返回数据损坏错误。
- When WAL checkpoint 失败时，系统应保留可 replay 的 WAL 文件。

**Steps:**
- [x] 写失败测试：sync write fsync 失败后重启不依赖 MemTable，数据通过 WAL 一致恢复或明确未确认。
- [x] 写失败测试：checkpoint remove/sync 失败后 WAL 仍可 replay。
- [x] 扩展 fault matrix 覆盖 WAL write、sync、checkpoint remove、checkpoint sync。
- [x] 运行 `go test ./internal/wal ./internal/engine ./tests/fault/storage_fault_matrix -run 'Test.*WAL|Test.*Fault' -timeout 300s`。

**实现备注：**
- 新增 engine 层 WAL append/sync 失败测试，确认 `Shard.Write` 在 WAL 失败时返回错误且 MemTable 不应用该批数据。
- 新增 WAL checkpoint remove/sync 失败测试，确认 checkpoint 失败后旧 WAL 仍可 replay。
- fault matrix 新增 `wal-write-failure`、`wal-sync-failure`、`wal-checkpoint-remove-failure`、`wal-checkpoint-sync-failure` 直接 WAL 层用例，避免故障先命中 SSTable/Manifest fsync。
- 现有 WAL replay 测试已覆盖尾部截断跳过与中间 checksum 损坏返回错误。
- 验证：`go test ./internal/wal ./internal/engine ./tests/fault/storage_fault_matrix -run 'Test.*WAL|Test.*Fault|TestShard.*WAL.*MemTable' -timeout 300s` 通过。

## Task 5: Fault Matrix 报告与最终门禁

**预计耗时:** 90m  
**硬超时:** 180m  
**下一次进度更新时间:** 开始后 30 分钟内

**Files:**
- Modify: `tests/fault/storage_fault_matrix/main.go`
- Modify: `tests/fault/storage_fault_matrix/main_test.go`
- Modify: `tests/README.md`
- Modify: `docs/superpowers/plans/2026-06-17-storage-reliability-fault-matrix.md`

**EARS:**
- When 故障注入测试执行时，系统应覆盖 WAL、Manifest、PartWriter、FileOps、Compaction、Retention 的成功和失败路径。
- When 每个故障用例完成时，报告应记录失败点、预期行为、恢复状态和查询一致性。
- When 验证完成时，系统应清理临时目录、二进制、profile 和 coverage。

**Steps:**
- [x] 扩展 `faultReport` 输出 case name、operation、stage、expected behavior、recovered、rows、maintenance issues。
- [x] 增加 `go test ./tests/fault/... -timeout 600s` 覆盖报告 schema。
- [x] 更新 `tests/README.md` 中 fault matrix 执行命令。
- [x] 运行 `goimports-reviser -rm-unused -format ./...`。
- [x] 运行 `go test ./... -timeout 10m`。
- [x] 运行 `golangci-lint run --timeout 10m`。
- [x] 扫描并清理 `testbin`、`*.prof`、`*.cover`、`coverage.out`。

**实现备注：**
- `faultReport` 已扩展为输出 `name`、`operation`、`stage`、`expected`、`recovered`、`rows`、`maintenance_issues`。
- `tests/fault/storage_fault_matrix` 新增 schema 测试，校验所有 case 的报告字段、恢复行数和 WAL checkpoint case 覆盖。
- `tests/README.md` 已补充 fault matrix 单独执行命令和 JSON 字段说明。
- 验证：`go test ./tests/fault/... -timeout 600s` 通过；`goimports-reviser -rm-unused -format ./...` 通过；`go test ./... -timeout 10m` 通过；`golangci-lint run --timeout 10m` 通过；独立 e2e 二进制用例全部通过。
- 清理：未发现 `testbin`、`*.prof`、`*.cover`、`coverage.out` 临时产物。
