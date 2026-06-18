# 专项闭环复检优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补齐专项一到专项六复检中发现的专项六结构化 ready、细分 metrics 和文档状态缺口。

**Architecture:** 保持专项一到专项五现有实现不变，只在 Engine 快照层和指标聚合层补低基数字段能力。MemTable 通过只读 stats snapshot 暴露 series/field 计数；SSTable 文件大小在 metrics snapshot 时按 manifest part 目录统计；QueryStats 记录 duration、budget error 和 cancellation；HealthSnapshot 增加结构化 checks 并映射到 public DTO。

**Tech Stack:** Go、`internal/engine`、`internal/memtable`、`internal/model`、`internal/observability`、public `mts` DTO、e2e。

**预计耗时与硬超时:** 计划 10 分钟；实现 60-90 分钟；单包测试 `-timeout 240s`；全量测试和 lint `-timeout 10m`；e2e build/run 总超时 900s。

---

## Task 1: Engine ready 结构化 checks

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/engine/engine_test.go`、`engine_test.go`
  - 测试：`TestEngineHealthSnapshotIncludesStructuredOperationalChecks`、`TestPublicHealthSnapshotIncludesStructuredChecksAndQueryStatsDetails`
  - 命令：`timeout 240s go test ./internal/engine . -run 'TestEngineHealthSnapshotIncludesStructuredOperationalChecks|TestPublicHealthSnapshotIncludesStructuredChecksAndQueryStatsDetails' -timeout 240s`
  - 实现备注：测试先失败于 `HealthSnapshot.Checks`、`HealthCheck` 和 query stats 细分字段缺失，失败点符合预期。

- [x] **Step 2: 实现 HealthCheck DTO**
  - 在 `internal/engine.HealthSnapshot` 和 public `mts.HealthSnapshot` 增加 `Checks []HealthCheck`。
  - Engine 输出 `wal`、`manifest`、`disk`、`compaction`、`memory`、`maintenance` checks。
  - 实现备注：Engine 生成六类结构化 checks；hard memory、WAL、Manifest、disk、compaction、maintenance 失败会影响 ready，soft memory 标记 degraded 但不强制不可用；public DTO 保留 checks。

- [x] **Step 3: 验证并记录实现备注**
  - 实现备注：`timeout 240s go test ./internal/engine . -run 'TestEngineHealthSnapshotIncludesStructuredOperationalChecks|TestEngineMetricsSnapshotIncludesCommercialSignalDetails|TestPublicHealthSnapshotIncludesStructuredChecksAndQueryStatsDetails' -timeout 240s` 通过。

## Task 2: 专项六细分 metrics

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/engine/engine_test.go`
  - 测试：`TestEngineMetricsSnapshotIncludesCommercialSignalDetails`
  - 命令：`timeout 240s go test ./internal/engine -run 'TestEngineMetricsSnapshotIncludesCommercialSignalDetails' -timeout 240s`
  - 实现备注：测试先失败于缺少 MemTable/SSTable/Query/Retention/Runtime 细分指标，失败点符合预期。

- [x] **Step 2: 实现 MemTable/SSTable/Query/Retention/Runtime 指标**
  - MemTable：series count、field count、flush triggered。
  - SSTable：data bytes、index bytes、total bytes、compression ratio。
  - Query：duration seconds、budget errors、cancellations。
  - Retention：deleted bytes、delete errors。
  - Runtime：GC pause total seconds。
  - 实现备注：MemTable 新增只读 `StatsSnapshot`；Engine metrics 聚合 SSTable 文件大小、query duration/budget/cancel、retention deleted bytes/delete errors 和 runtime GC pause。

- [x] **Step 3: 验证并记录实现备注**
  - 实现备注：`timeout 240s go test ./internal/engine . -run 'TestEngineHealthSnapshotIncludesStructuredOperationalChecks|TestEngineMetricsSnapshotIncludesCommercialSignalDetails|TestPublicHealthSnapshotIncludesStructuredChecksAndQueryStatsDetails' -timeout 240s` 通过。

## Task 3: 文档状态、检视报告与最终门禁

- [x] **Step 1: 更新计划和检视报告状态**
  - 更新专项六原计划的提交备注。
  - 将本检视报告 P1/P2 项标记为已处理。
  - 实现备注：专项六原计划已更新提交哈希 `9b950af`；复检报告 3 个问题均已标记为已处理；`docs/storage/metrics.md` 已同步新增指标。

- [x] **Step 2: 运行最终验证**
  - `timeout 300s goimports-reviser -rm-unused -format ./...`
  - `timeout 600s go test ./... -timeout 10m`
  - `timeout 600s golangci-lint run --timeout 10m`
  - `timeout 900s bash -c 'set -euo pipefail; for dir in tests/e2e/*; do [ -d "$dir" ] || continue; (cd "$dir" && go build -o testbin . && timeout 120s ./testbin; status=$?; rm -f testbin; exit $status); done'`
  - `timeout 60s git diff --check`
  - `timeout 60s find . -type f \( -name testbin -o -name "*.prof" -o -name "*.cover" -o -name coverage.out \) -print`
  - 实现备注：上述命令均通过；额外执行 `timeout 600s go test ./... -cover -timeout 10m` 通过，但 root、engine、sstable、storagecheck、cmd 等历史包仍低于 90% 覆盖率门槛。

- [x] **Step 3: 提交**
  - Commit: `fix(storage): 闭环专项复检缺口`
  - 实现备注：已随本提交提交。
