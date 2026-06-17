# 文件格式治理、恢复协议与工具化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 闭环专项五，使 WAL、SSTable、Manifest 具备可识别、可校验、可诊断、可修复的商用级文件格式治理能力。

**Architecture:** 复用现有 `codec.Envelope` 作为持久化 envelope 基础，在 WAL segment、SSTable metadata/component、Manifest 上补齐 format id、sequence、引用校验和明确错误。新增 `internal/storagecheck` 作为离线 check/repair/migrate 核心包，CLI 仅做参数解析和输出，Engine 打开时执行目录权限与 manifest/part 一致性校验。

**Tech Stack:** Go、`internal/codec`、`internal/wal`、`internal/sstable`、`internal/engine`、`internal/storagecheck`、`cmd/mts-storage`、`tests/e2e`。

**预计耗时与硬超时:** 计划和代码定位 10 分钟；实现 120-180 分钟；单包测试 `-timeout 240s`；全量 `go test ./... -timeout 10m`；`golangci-lint run --timeout 10m`；每个 e2e build/run 使用 `timeout 120s`。

---

## EARS 覆盖矩阵

| EARS | 覆盖任务 |
| --- | --- |
| WAL segment 包含 magic、format id、checksum、record 边界 | Task 1 |
| WAL 读取验证 magic、checksum、record length、截断状态 | Task 1 |
| SSTable metadata 包含 magic、format id、part id、level、time range、series range、row count、block refs、checksum | Task 2 |
| SSTable metadata 读取验证 block ref 不越界 | Task 2 |
| Manifest 包含 magic、format id、manifest sequence、parts 列表、checksum | Task 3 |
| Manifest 读取验证 sequence 单调性和 part 引用存在 | Task 3、Task 5 |
| 新增 SSTable 文件组件在 metadata 中引用并由 OpenPart 校验存在 | Task 2 |
| 未知/不兼容格式拒绝打开并返回明确错误 | Task 1、Task 2、Task 3 |
| 兼容扩展提供字段默认值和测试覆盖 | Task 2、Task 3 |
| 离线 check 扫描 WAL、Manifest、SSTable、series index、value pages 并输出报告 | Task 5 |
| check 发现孤儿 part、Manifest 缺失 part、checksum 错误并定位 | Task 5 |
| repair dry-run/apply 只执行显式安全修复动作 | Task 6 |
| 迁移工具具备备份/checkpoint 和中断恢复 | Task 6 |
| e2e 验证存储目录不包含 JSON 数据文件 | Task 7 |
| 目录 0700、普通文件 0600 | Task 4、Task 7 |
| 工具诊断报告包含路径、part id、level、time range、series range、错误原因 | Task 5 |
| 未知文件按 ignore/warn/fatal 策略处理 | Task 5 |
| Engine 打开数据目录时校验目录结构和权限 | Task 4 |
| SSTable series index、metaindex、index、timestamps、values 不一致时拒绝加载 | Task 2、Task 5 |

## 文件结构

- Modify: `internal/wal/wal.go`，写入和读取 WAL segment header，校验 segment magic/format/checksum。
- Modify/Test: `internal/wal/wal_test.go`、`internal/wal/internal_test.go`。
- Modify: `internal/sstable/types.go`、`internal/sstable/metadata_encoding.go`、`internal/sstable/read.go`、`internal/sstable/write.go`、`internal/sstable/manifest.go`，补齐 Manifest sequence、part component refs、block ref 边界和引用存在校验。
- Modify/Test: `internal/sstable/sstable_test.go`、`internal/sstable/internal_test.go`。
- Modify: `internal/engine/lifecycle.go`、`internal/engine/paths.go`，打开时校验基础目录结构和权限。
- Create: `internal/storagecheck/checker.go`、`internal/storagecheck/repair.go`、`internal/storagecheck/migrate.go`，实现离线校验、修复计划和迁移 checkpoint。
- Create: `cmd/mts-storage/main.go`，提供 `check`、`repair --dry-run`、`repair --apply`、`migrate --dry-run/--apply` 工具入口。
- Create/Modify: `tests/e2e/format_governance/main.go`、`tests/e2e/no_json_storage/main.go`。
- Create: `docs/storage/file-format.md`、`docs/storage/recovery-protocol.md`。

---

## Task 1: WAL Segment Header 与严格读取

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/wal/wal_test.go`
  - 测试：`TestWALSegmentHasHeaderAndRejectsUnknownFormat`、`TestWALReplayTruncatesPartialLastRecordAfterHeader`。
  - 命令：`timeout 240s go test ./internal/wal -run 'TestWALSegmentHasHeaderAndRejectsUnknownFormat|TestWALReplayTruncatesPartialLastRecordAfterHeader' -timeout 240s`
  - Expected: FAIL，当前 segment 没有 header。

- [x] **Step 2: 实现**
  - WAL segment 首部写入固定 magic、format id、header length、CRC。
  - replay 先校验 header，再按 frame length、record type、CRC 读取；最后一个 segment 的尾部截断仍可安全 truncate。

- [x] **Step 3: 验证并记录实现备注**
  - 命令：`timeout 240s go test ./internal/wal -run 'TestWAL|Test.*Segment|Test.*Replay' -timeout 240s`
  - 实现备注：已新增 WAL segment header，包含 `MTSWAL2` magic、format id、header length、CRC；`Open` 创建空 segment 时立即写 header，`Replay` 先校验 header，再按 frame length、record type、record CRC 读取。未知/损坏 header 返回明确错误；最后一个 segment 仅对 header 后的 partial record 执行 truncate。验证命令通过。

## Task 2: SSTable Metadata 与 Component 完整性校验

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/sstable/sstable_test.go`
  - 测试：`TestOpenPartRejectsOutOfBoundsMetadataRefs`、`TestOpenPartRejectsMissingComponent`、`TestOpenPartRejectsComponentChecksumCorruption`。
  - 命令：`timeout 240s go test ./internal/sstable -run 'TestOpenPartRejectsOutOfBoundsMetadataRefs|TestOpenPartRejectsMissingComponent|TestOpenPartRejectsComponentChecksumCorruption' -timeout 240s`
  - Expected: FAIL，缺少完整 component/ref 校验。

- [x] **Step 2: 实现**
  - metadata 中明确记录 required components，保留兼容默认值：老 metadata 未记录时默认 `index/timestamps/values/metaindex/series_index/strings`。
  - `OpenPart` 校验 metadata、index、metaindex、series index、timestamps、values、strings 文件存在且 block refs 不越界。
  - 读取 metaindex、series index、index row、value page 时将 checksum 错误包装成包含文件、offset、block type 的错误。

- [x] **Step 3: 验证并记录实现备注**
  - 命令：`timeout 240s go test ./internal/sstable -run 'TestOpenPart|Test.*Metadata|Test.*Checksum|TestPart' -timeout 240s`
  - 实现备注：metadata 已新增 required components 列表，老 metadata 解码时默认要求 `metadata.bin/metaindex.bin/index.bin/series_index.bin/timestamps.bin/values.bin/strings.bin`；`OpenPart` 现在校验 component 存在、metadata block refs 不越界，并读取验证 index、series index、timestamps、value page index/value page 的 block CRC。损坏 part 在加载阶段拒绝。验证命令通过。

## Task 3: Manifest Sequence、Format 与引用验证

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/sstable/manifest_test.go`
  - 测试：`TestManifestPersistsSequenceAndRejectsRegression`、`TestLoadManifestRejectsMissingReferencedPart`。
  - 命令：`timeout 240s go test ./internal/sstable -run 'TestManifestPersistsSequenceAndRejectsRegression|TestLoadManifestRejectsMissingReferencedPart' -timeout 240s`
  - Expected: FAIL，Manifest 缺少 sequence 和引用校验入口。

- [x] **Step 2: 实现**
  - `Manifest` 增加 `Sequence uint64`，写入时持久化 sequence；兼容老文件默认 sequence 为 0。
  - 提供 `LoadManifestStrict(dir string, previousSequence uint64)`，校验 sequence 不回退并校验 Manifest 引用 part 存在。
  - Engine 使用 strict load。

- [x] **Step 3: 验证并记录实现备注**
  - 命令：`timeout 240s go test ./internal/sstable ./internal/engine -run 'TestManifest|Test.*Recovery|Test.*Lifecycle' -timeout 240s`
  - 实现备注：Manifest 新增 `Sequence`，新格式通过 envelope flags 写入 sequence，旧格式解码为 sequence 0；新增 `LoadManifestStrict(dir, previousSequence)`，可拒绝 sequence 回退和缺失 part 引用。Engine flush/compaction 提交 Manifest 时递增 sequence，同时保留原 recovery audit 路径用于结构化 `RecoveryIssue`。验证命令通过。

## Task 4: 数据目录结构与权限校验

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/engine/lifecycle_test.go`、`internal/storagefs/fs_test.go`
  - 测试：`TestOpenRejectsUnsafeStoragePermissions`、`TestStorageFilesUseStrictPermissions`。
  - 命令：`timeout 240s go test ./internal/engine ./internal/storagefs -run 'TestOpenRejectsUnsafeStoragePermissions|TestStorageFilesUseStrictPermissions' -timeout 240s`
  - Expected: FAIL，Engine 打开还未统一权限校验。

- [x] **Step 2: 实现**
  - Engine 打开时校验 root、wal、sstable 目录权限必须不宽于 `0700`。
  - storagefs 新增 `ValidateStrictPermissions`，普通文件必须不宽于 `0600`，目录必须不宽于 `0700`。

- [x] **Step 3: 验证并记录实现备注**
  - 命令：`timeout 240s go test ./internal/engine ./internal/storagefs -run 'Test.*Permission|TestOpen|TestNew' -timeout 240s`
  - 实现备注：新增 `storagefs.ValidateStrictPermissions`，目录权限不得宽于 `0700`，文件权限不得宽于 `0600`；Engine/OpenShard 对已存在 root、data root、shard 目录先校验再打开，避免自动 chmod 掩盖不安全权限。验证命令通过。

## Task 5: 离线 Check 报告

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/storagecheck/checker_test.go`
  - 测试：`TestCheckReportsOrphanPartMissingReferencedPartAndChecksum`、`TestCheckUnknownFilePolicies`。
  - 命令：`timeout 240s go test ./internal/storagecheck -run 'TestCheckReportsOrphanPartMissingReferencedPartAndChecksum|TestCheckUnknownFilePolicies' -timeout 240s`
  - Expected: FAIL，包不存在。

- [x] **Step 2: 实现**
  - `Check(root, Options) (Report, error)` 扫描 WAL、Manifest、SSTable part components、series index、metaindex、index、timestamps、values。
  - report item 包含 severity、path、part id、level、time range、series range、reason、offset、block type。
  - unknown file 支持 `ignore`、`warn`、`fatal`。

- [x] **Step 3: 验证并记录实现备注**
  - 命令：`timeout 240s go test ./internal/storagecheck -run 'TestCheck|Test.*Unknown|Test.*Report' -timeout 240s`
  - 实现备注：新增 `internal/storagecheck.Check`，输出结构化 `Report/Issue`，扫描 WAL segment header、Manifest、SSTable part components 和未知文件；可报告孤儿 part、Manifest 引用缺失 part、SSTable 打开/CRC 错误、WAL header 格式错误，未知文件支持 `ignore/warn/fatal`。验证命令通过。

## Task 6: Repair 与 Migrate 工具入口

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/storagecheck/repair_test.go`、`cmd/mts-storage/main_test.go`
  - 测试：`TestRepairDryRunAndApplyRemovesOnlyOrphans`、`TestMigrateCreatesCheckpointAndCanResume`、`TestStorageToolCheckRepairCommands`。
  - 命令：`timeout 240s go test ./internal/storagecheck ./cmd/mts-storage -run 'TestRepairDryRunAndApplyRemovesOnlyOrphans|TestMigrateCreatesCheckpointAndCanResume|TestStorageToolCheckRepairCommands' -timeout 240s`
  - Expected: FAIL，工具入口不存在。

- [x] **Step 2: 实现**
  - `Repair(root, RepairOptions)` 默认 dry-run，只对孤儿 part 删除生成计划；`Apply` 为 true 才执行。
  - `Migrate(root, MigrateOptions)` 创建 manifest 备份和 checkpoint；再次运行时根据 checkpoint 完成恢复。
  - CLI 输出 JSON，失败时非零退出码。

- [x] **Step 3: 验证并记录实现备注**
  - 命令：`timeout 240s go test ./internal/storagecheck ./cmd/mts-storage -run 'TestRepair|TestMigrate|TestStorageTool' -timeout 240s`
  - 实现备注：新增 `Repair` 和 `Migrate` 核心能力；repair 默认 dry-run，仅对 `Check` 标记的孤儿 part 生成删除动作，`Apply` 为 true 才删除；migrate 生成 `MANIFEST.bin.bak` 和二进制 `MIGRATION.checkpoint`，重复运行可识别 resume。新增 `cmd/mts-storage`，支持 `check`、`repair --dry-run/--apply`、`migrate --dry-run/--apply`，输出 JSON。验证命令通过。

## Task 7: E2E、文档与最终门禁

- [x] **Step 1: 写/扩展 e2e 与文档**
  - 新增 `tests/e2e/format_governance` 覆盖 check、repair dry-run/apply、缺失 part、无 JSON 数据文件、权限。
  - 更新 `tests/e2e/no_json_storage`，确认 WAL、Manifest、SSTable 数据文件均不是 JSON。
  - 新增 `docs/storage/file-format.md` 和 `docs/storage/recovery-protocol.md`。

- [x] **Step 2: 运行最终验证**
  - `timeout 300s goimports-reviser -rm-unused -format ./...`
  - `timeout 600s go test ./... -timeout 10m`
  - `timeout 600s golangci-lint run --timeout 10m`
  - `timeout 900s bash -c 'set -euo pipefail; for dir in tests/e2e/*; do [ -d "$dir" ] || continue; (cd "$dir" && go build -o testbin . && timeout 120s ./testbin; status=$?; rm -f testbin; exit $status); done'`
  - `timeout 60s git diff --check`
  - `timeout 60s find . -type f \( -name testbin -o -name "*.prof" -o -name "*.cover" -o -name coverage.out \) -print`
  - 实现备注：新增 `tests/e2e/format_governance`，覆盖 check、repair dry-run/apply、权限拒绝和非 JSON 数据文件；新增 `docs/storage/file-format.md` 与 `docs/storage/recovery-protocol.md`。上述最终验证命令均通过。额外执行 `timeout 600s go test ./... -cover -timeout 10m` 也通过，但多个包覆盖率仍低于 90%，例如 root 83.2%、`internal/engine` 86.8%、`internal/sstable` 87.8%、`internal/storagecheck` 78.3%、`cmd/mts-storage` 46.0%，属于全仓覆盖率门禁缺口。

- [x] **Step 3: 提交**
  - Commit: `feat(storage): 完善文件格式治理`
  - 实现备注：本任务所有源码、测试、e2e 和文档改动将以该提交信息提交；提交前已确认无 `testbin`、profile、coverage 等临时产物。
