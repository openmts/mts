# 生产级 Metrics、服务化运维与长稳门禁 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 闭环专项六，使 mts 存储层具备生产可观测指标、健康/运维接口、pprof 入口、scale gate 和长稳报告能力。

**Architecture:** 复用现有 `internal/observability` registry、`internal/service` HTTP server 和 `tests/scale` 压测框架。Engine 负责聚合低基数字段指标与 ready 状态，service 负责 HTTP 暴露和 admin 审计，scale tests 负责 quick/standard/soak 三档门禁和 JSON 报告。

**Tech Stack:** Go、`internal/engine`、`internal/wal`、`internal/observability`、`internal/service`、`tests/e2e/service_ops`、`tests/scale/storage_10m`、`tests/scale/storage_soak`。

**预计耗时与硬超时:** 计划 10 分钟；实现 120-180 分钟；单包测试 `-timeout 240s`；全量 `go test ./... -timeout 10m`；`golangci-lint run --timeout 10m`；每个 e2e build/run 使用 `timeout 120s`。

---

## EARS 覆盖矩阵

| EARS | 覆盖任务 |
| --- | --- |
| Engine 打开时注册 WAL/MemTable/SSTable/Query/Compaction/Retention/Recovery/StorageMemory/Runtime 指标 | Task 1、Task 2 |
| WAL append/fsync/segment/pending/checkpoint/replay 指标 | Task 1 |
| MemTable sample/bytes/series/field/flush reason 指标 | Task 2 |
| SSTable part/level/data/index/compression 指标 | Task 2 |
| Query duration/shards/parts/pages/samples/budget/cancel 指标 | Task 2 |
| Compaction active/backlog/score/bytes/duration/errors 指标 | Task 2 |
| Retention expired/delete errors、Recovery replay/orphan/error 指标 | Task 2 |
| StorageMemory 和 Runtime 指标 | Task 2 |
| `/metrics`、`/healthz`、`/readyz`、`/debug/pprof/`、`/admin/compact` | Task 3、Task 6 |
| `/readyz` 检查 WAL/Manifest/disk/compaction/memory/maintenance 并输出结构化原因 | Task 3 |
| `/admin/compact` 要求 timeout，返回 task id/status/error，记录审计日志 | Task 3 |
| pprof 地址可通过配置限制 | Task 3 |
| metrics 避免高基数标签 | Task 2、Task 4 |
| 10M write/query/compact/restart JSON 报告和 baseline 回归 | Task 4 |
| quick/standard/soak 三档 gate | Task 4、Task 5 |
| 长稳周期输出机器可解析报告，覆盖 write/query/compact/restart/recovery | Task 5 |
| 文档列出指标名称、类型、标签、含义、告警建议 | Task 6 |

## Task 1: WAL 生产指标

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/wal/internal_test.go`、`internal/engine/engine_test.go`
  - 测试：`TestWALMetricsSnapshotRecordsAppendSyncCheckpointReplay`、`TestEngineMetricsSnapshotIncludesWALSignals`
  - 命令：`timeout 240s go test ./internal/wal ./internal/engine -run 'TestWALMetricsSnapshotRecordsAppendSyncCheckpointReplay|TestEngineMetricsSnapshotIncludesWALSignals' -timeout 240s`

- [x] **Step 2: 实现 WAL metrics**
  - `wal.Log` 增加 `MetricsSnapshot()`，记录 append latency、fsync latency、segment count、pending bytes、checkpoint count、replay records、append errors。
  - `engine.walStore` 通过可选接口读取 WAL metrics 并聚合到 Engine metrics。

- [x] **Step 3: 验证并记录实现备注**
  - 实现备注：`wal.Log` 新增 `MetricsSnapshot`，记录 append/sync/checkpoint/replay 计数与累计耗时、segment count、pending bytes；Engine 通过可选 `walMetricsProvider` 聚合并暴露 `mts_wal_*` 指标。验证命令通过。

## Task 2: Engine 指标聚合扩展

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/engine/engine_test.go`
  - 测试：`TestEngineMetricsSnapshotIncludesProductionSignals`
  - 命令：`timeout 240s go test ./internal/engine -run 'TestEngineMetricsSnapshotIncludesProductionSignals' -timeout 240s`
  - 实现备注：测试先失败于缺少 `mts_memtable_samples` 等生产指标，失败点符合预期。

- [x] **Step 2: 实现**
  - MetricsSnapshot 增加 MemTable、SSTable、Query、Retention、Recovery、Runtime 指标。
  - 不引入 raw series key、measurement 等高基数字段标签。
  - 实现备注：Engine 聚合 shard 内 MemTable/SSTable/Recovery/Retention 快照，Query 使用最近一次查询统计，Runtime 使用 Go runtime 与 `/proc/self/fd`；指标名不携带 series、measurement、tag 等高基数字段。

- [x] **Step 3: 验证并记录实现备注**
  - 实现备注：`timeout 240s go test ./internal/engine -run 'TestEngineMetricsSnapshotIncludesProductionSignals' -timeout 240s` 通过。

## Task 3: 服务端点、Ready 结构化原因和 Admin 审计

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/service/server_test.go`
  - 测试：`TestReadyzReturnsStructuredChecks`、`TestAdminCompactRequiresTimeoutAndReturnsTaskStatus`、`TestAdminCompactWritesAuditLog`
  - 命令：`timeout 240s go test ./internal/service -run 'TestReadyzReturnsStructuredChecks|TestAdminCompactRequiresTimeoutAndReturnsTaskStatus|TestAdminCompactWritesAuditLog' -timeout 240s`
  - 实现备注：测试先失败于缺少结构化 ready checks、admin task response、timeout 强制和审计类型，失败点符合预期。

- [x] **Step 2: 实现**
  - `Health` 增加 `Checks []HealthCheck`，ready 失败时返回结构化原因。
  - `/admin/compact` 要求 `AdminTimeout > 0` 或请求 context 有 deadline；响应包含 task id、state、duration、error。
  - `Options` 增加 `AuditLogger`，admin 操作写结构化审计日志。
  - 实现备注：`Health` 新增低基数字段检查列表；`/admin/compact` 强制 timeout/deadline，返回 `task_id/state/duration_ms/error`，成功与失败均写 `AdminAuditEvent`。

- [x] **Step 3: 验证并记录实现备注**
  - 实现备注：`timeout 240s go test ./internal/service -run 'TestReadyzReturnsStructuredChecks|TestAdminCompactRequiresTimeoutAndReturnsTaskStatus|TestAdminCompactWritesAuditLog' -timeout 240s` 通过。

## Task 4: Scale Gate 三档和阈值失败

- [x] **Step 1: 写 failing tests**
  - 文件：`tests/scale/storage_10m/main_test.go`
  - 测试：`TestParseConfigProfilesAndThresholds`、`TestRunFailsWhenThresholdExceeded`
  - 命令：`timeout 240s go test ./tests/scale/storage_10m -run 'TestParseConfigProfilesAndThresholds|TestRunFailsWhenThresholdExceeded' -timeout 240s`
  - 实现备注：测试先失败于缺少 profile、阈值字段和阈值校验函数，失败点符合预期。

- [x] **Step 2: 实现**
  - `storage_10m` 增加 `-profile quick|standard|soak`、`-max-rss-bytes`、`-max-sstable-count`、`-max-compaction-backlog`。
  - 报告补充 profile、errors、cold/hot query latency、backlog drain time。
  - 实现备注：`quick/standard/soak` 分别映射默认 10K/1M/10M 点位，可用 `-points` 覆盖；报告包含 profile、errors、cold/hot query latency、backlog drain nanos；阈值失败返回机器可解析的字段名和实际/限制值。

- [x] **Step 3: 验证并记录实现备注**
  - 实现备注：`timeout 240s go test ./tests/scale/storage_10m -run 'TestParseConfigProfilesAndThresholds|TestRunFailsWhenThresholdExceeded' -timeout 240s` 通过。

## Task 5: Soak 周期报告和工作负载覆盖

- [x] **Step 1: 写 failing tests**
  - 文件：`tests/scale/storage_soak/main_test.go`
  - 测试：`TestSoakPeriodicReportsAndWorkloadCoverage`
  - 命令：`timeout 240s go test ./tests/scale/storage_soak -run 'TestSoakPeriodicReportsAndWorkloadCoverage' -timeout 240s`
  - 实现备注：测试先失败于缺少 soak 配置、周期报告类型和 `runSoakWithReports`，失败点符合预期。

- [x] **Step 2: 实现**
  - `storage_soak` 增加周期 JSONL 报告，覆盖 write/query/compact/restart/recovery 计数。
  - 压测结束清理临时目录，显式 data-dir 时保留。
  - 实现备注：新增 `-report-interval`、`-report-jsonl`、`-data-dir`；周期 JSONL 输出 iteration/rows/workload counts/part count/backlog；soak 循环覆盖 write/query/compact，并至少执行一次 reopen/recovery；临时目录自动清理，显式 data-dir 保留。

- [x] **Step 3: 验证并记录实现备注**
  - 实现备注：`timeout 240s go test ./tests/scale/storage_soak -run 'TestSoakPeriodicReportsAndWorkloadCoverage' -timeout 240s` 通过。

## Task 6: E2E 运维与指标文档

- [x] **Step 1: 写/扩展 e2e 与文档**
  - 扩展 `tests/e2e/service_ops` 覆盖 `/metrics`、`/healthz`、`/readyz`、`/debug/pprof/`、`/admin/compact`。
  - 新增 `docs/storage/metrics.md`。
  - 实现备注：`service_ops` 覆盖 metrics、healthz、readyz 结构化 checks、pprof index、admin compact task response 和审计；新增 metrics 文档列出指标名称、类型、含义和告警建议。

- [x] **Step 2: 运行最终验证**
  - `timeout 300s goimports-reviser -rm-unused -format ./...`
  - `timeout 600s go test ./... -timeout 10m`
  - `timeout 600s golangci-lint run --timeout 10m`
  - `timeout 900s bash -c 'set -euo pipefail; for dir in tests/e2e/*; do [ -d "$dir" ] || continue; (cd "$dir" && go build -o testbin . && timeout 120s ./testbin; status=$?; rm -f testbin; exit $status); done'`
  - `timeout 60s git diff --check`
  - `timeout 60s find . -type f \( -name testbin -o -name "*.prof" -o -name "*.cover" -o -name coverage.out \) -print`
  - 实现备注：全部验证通过；额外执行 `timeout 600s go test ./... -cover -timeout 10m` 通过，但仓库仍存在若干历史包覆盖率低于 90%。

- [x] **Step 3: 提交**
  - Commit: `feat(storage): 完善生产运维门禁`
  - 实现备注：已提交，提交哈希 `9b950af`。
