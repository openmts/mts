# Downsample Commercial Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补齐单机降采样的商用主链路，使其具备可取消调度、分批执行、增量 checkpoint、一致性治理、运维 API、per-policy 观测、故障矩阵和规模压测入口。

**Architecture:** 在不改变 SSTable 格式、不引入分布式和 SQL 的前提下，扩展 `DownsamplePolicy` 配置、LocalMetadataStore 二进制持久化、Engine scheduler/executor/API。执行路径使用现有 `QueryColumnIterator` 和 `Write`，以小窗口、列流消费、批量写入和 context 取消控制内存与运行边界。

**Tech Stack:** Go、LocalMetadataStore、Engine、queryexec ColumnIterator、WAL/MemTable/SSTable 写入主链路、observability、faultinject、tests/e2e、tests/fault、tests/scale。

---

## 文件职责

- `internal/model/types.go` / `types.go`：扩展 downsample policy、result、status、range/dry-run/reset 公开模型。
- `internal/engine/downsample_validate.go`：默认值、兼容性校验、策略 tag 归一化、窗口对齐。
- `internal/catalog/downsample.go`：新增 policy 字段和 reset allowance 的本地二进制持久化。
- `internal/engine/downsample_metadata.go`：policy 管理 API、非兼容变更拒绝、reset/backfill/dry-run 入口。
- `internal/engine/downsample_scheduler.go`：scheduler root context、run timeout、关闭取消。
- `internal/engine/downsample_executor.go`：窗口规划、checkpoint、列流消费、分批写入、policy tag。
- `internal/engine/downsample_stats.go` / `internal/engine/metrics.go`：per-policy status 和指标。
- `internal/engine/downsample_*_test.go`：TDD 覆盖核心行为。
- `tests/fault/downsample_policy/main.go`：扩展 fault 矩阵。
- `tests/scale/downsample_policy/main.go`：扩展参数与报告。
- `docs/storage/downsample-runbook.md`：更新运维手册。

## Task 1: 模型、默认值和兼容性治理

**状态:** 已完成。

**EARS:**
- When policy 没有设置 batch/checkpoint/run timeout/policy tag 时，系统应填充稳定默认值。
- When 同名 policy 的 source/target/interval/functions/group tags/policy tag/initial start 发生变化时，系统应拒绝非兼容覆盖。
- When 用户 reset policy 且允许替换时，系统应允许下一次非兼容 policy 覆盖。

**验收:** 新增模型字段可 public/internal 双向转换；policy 校验和兼容性单测通过。

**实现备注:** 已扩展 internal/public downsample 模型，新增默认 `BatchSize`、`CheckpointInterval`、`RunTimeout`、`PolicyTagName`，新增非兼容 policy 更新校验和 reset allowance。定向验证：
`timeout 180s go test ./internal/engine -run 'TestNormalizeDownsamplePolicy|TestDownsamplePolicyCompatibility|TestCreateDownsamplePolicyRejects' -count=1 -timeout 3m`；
`timeout 180s go test . -run 'TestDownsamplePolicyPublicAPI' -count=1 -timeout 3m`。

## Task 2: Scheduler context cancellation

**状态:** 已完成。

**EARS:**
- When Engine 关闭时，scheduler 应 cancel root context 并停止触发新任务。
- When policy run 已经进入查询链路时，context cancellation 应能让运行返回取消错误。
- When 自动运行 policy 时，系统应使用 policy run timeout 或默认超时。

**验收:** scheduler 取消单测通过，Close 不依赖无边界等待。

**实现备注:** 已新增 scheduler root context、Close cancel、自动运行 timeout context，并将扫描和运行链路改为可取消 context。定向验证：
`timeout 180s go test ./internal/engine -run 'TestDownsampleSchedulerCloseCancelsRootContext|TestDownsampleRunContextUsesPolicyTimeout|TestDownsampleSchedulerRunsEnabledPolicyAndStopsOnClose' -count=1 -timeout 3m`。

## Task 3: Window planning、initial backfill、lookback refresh 和 checkpoint

**状态:** 已完成。

**EARS:**
- When watermark 为空且设置了 `InitialStartTime` 时，系统应从该时间对齐窗口开始；未设置时应从 lookback 起点或最后一个完整窗口开始。
- When lookback 覆盖已完成窗口时，系统应生成 refresh windows 且不倒退 watermark。
- When advance window 成功时，系统应按 checkpoint interval 更新 watermark。
- When 中途失败时，下次应从最近 checkpoint 继续，不重跑全部成功窗口。

**验收:** watermark/refresh/checkpoint/失败续跑单测通过。

**实现备注:** 已实现 initial start、refresh/advance 窗口分类、按 checkpoint interval 推进 watermark、失败保留最近 checkpoint，并修复 context canceled 时失败状态无法落 metadata 的问题。定向验证：
`timeout 180s go test ./internal/engine -run 'TestDownsampleWindowsToRunUsesInitialStartAndMarksRefresh|TestRunDownsamplePolicyAdvancesWatermarkAndRefreshesLookback|TestRunDownsamplePolicySkipsIncompleteWindow' -count=1 -timeout 3m`；
`timeout 180s go test ./internal/engine -run 'TestRunDownsamplePolicyKeepsCheckpointAfterLaterWindowFailure|TestDownsampleWindowsToRunUsesInitialStartAndMarksRefresh' -count=1 -timeout 3m`。

## Task 4: 列流消费和分批写入 executor

**状态:** 已完成。

**EARS:**
- When executor 查询窗口时，系统应使用 `QueryColumnIterator` 逐列消费聚合结果。
- When 目标点数达到 `BatchSize` 时，系统应立即写入目标 RP 并复用 buffer。
- When 写入目标点时，系统应附加 policy tag。

**验收:** executor 单测证明不再调用 `QueryColumns` 路径，batch 写入和 tag 正确。

**实现备注:** 已将窗口执行改为 `QueryColumnIterator` 列流消费，目标点按 `BatchSize` 分批写入，并写入默认或自定义 policy tag。定向验证：
`timeout 180s go test ./internal/engine -run 'TestDownsampleExecutorWritesAggregatedPoints|TestDownsampleExecutorWritesCompletePointsWithSmallBatch' -count=1 -timeout 3m`。

## Task 5: Backfill、repair、dry-run 和 reset API

**状态:** 已完成。

**EARS:**
- When 用户执行 dry-run 时，系统应返回窗口数和范围，不写入、不推进 watermark。
- When 用户执行 range backfill 时，系统应处理指定范围并按选项决定是否推进 watermark。
- When 用户执行 repair 时，系统应重算指定范围且不推进 watermark。
- When 用户执行 reset 时，系统应更新 watermark 并可允许 policy 替换。

**验收:** public/internal API 测试通过。

**实现备注:** 已新增 dry-run、repair、range backfill 和 reset API，range backfill 支持可选推进 watermark，并阻止从当前 watermark 之后跳跃推进。定向验证：
`timeout 180s go test ./internal/engine -run 'TestEngineDownsampleRangeDryRunRepairAndBackfill' -count=1 -timeout 3m`。

## Task 6: Per-policy status、metrics 和 runbook

**状态:** 已完成。

**EARS:**
- When 用户查看 status 时，系统应返回每个 policy 的 active、lag、watermark、next run、last error、duration、windows、points。
- When metrics snapshot 被采集时，系统应输出全局指标和 per-policy 指标。
- When HealthSnapshot 退化时，reason 应包含失败 policy。

**验收:** metrics/status/health 单测通过，runbook 更新。

**实现备注:** 已新增 per-policy status API、runtime stats、per-policy metrics、Health reason policy 名称和 runbook 指南。定向验证：
`timeout 180s go test ./internal/engine -run 'TestDownsamplePolicyStatusesAndMetrics|TestDownsampleMetricsAndHealthExposeStats|TestDownsampleStatsExposeWatermarkAndFailures' -count=1 -timeout 3m`。

## Task 7: Fault 矩阵

**状态:** 已完成。

**EARS:**
- When 目标写失败时，watermark 不应越过失败窗口。
- When metadata 保存失败时，watermark 不应被错误推进。
- When context cancellation 发生时，run 应返回取消错误并保留 checkpoint。
- When policy 非兼容变更发生时，系统应拒绝覆盖。

**验收:** unit fault 与 `tests/fault/downsample_policy` 覆盖通过。

**实现备注:** 已补充存储/metadata rename 失败不推进水位、context cancellation 后保留 checkpoint、非兼容 policy 更新拒绝等故障测试。定向验证：
`timeout 180s go test ./internal/engine -run 'TestRunDownsamplePolicyRecordsFailureWithoutAdvancingWatermark|TestRunDownsamplePolicyMetadataFailureDoesNotAdvanceWatermark|TestRunDownsamplePolicyKeepsCheckpointAfterLaterWindowFailure|TestCreateDownsamplePolicyRejectsIncompatibleUpdateUntilReset' -count=1 -timeout 3m`。

## Task 8: Scale 用例

**状态:** 已完成。

**EARS:**
- When scale workload 运行时，用户应能配置 points、series、policy count、batch size、checkpoint interval、run timeout 和 initial start。
- When scale workload 完成时，报告应包含 write/downsample/query duration、RSS、windows、points、watermark、status 和 policy count。

**验收:** `tests/scale/downsample_policy` 单元测试通过，小规模 smoke 可运行。

**实现备注:** 已扩展 scale flags 和 JSON 报告，支持 policy count、batch size、checkpoint interval、run timeout、initial start 和 status count。定向验证：
`timeout 300s go test ./tests/scale/downsample_policy -count=1 -timeout 5m`。

## Task 9: 收尾验证

**状态:** 已完成。

**EARS:**
- When 代码完成后，系统应运行 goimports-reviser、go test、golangci-lint 和关键 e2e/fault/scale smoke。
- When 验证失败时，系统应修复后重新运行对应命令。

**验收:** 最终验证命令结果在交付回复中如实报告。

**实现备注:** 已执行 goimports-reviser、全量测试和 golangci-lint。验证命令：
`timeout 300s goimports-reviser -rm-unused -format ./...`；
`timeout 720s golangci-lint run ./...`；
`timeout 600s go test ./... -count=1 -timeout 10m`。

## 实现备注

- 2026-06-20：Task 1 到 Task 9 已按本计划完成并通过验证。
