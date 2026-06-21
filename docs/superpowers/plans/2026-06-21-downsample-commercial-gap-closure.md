# Downsample Commercial Gap Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 闭环降采样商用差距，使单机规则物化能力具备明确内存边界、成本估算、标准观测、故障恢复、目标数据治理、函数扩展和规模测试入口。

**Architecture:** 保持当前单机 Engine、LocalMetadataStore、Builder/API 边界不变。降采样 executor 改为 bounded collector，dry-run 通过源列扫描估算 group 和样本数，observability registry 增加 label 支持，rollup cleanup 复用 shard tombstone，函数扩展复用 queryexec 聚合语义。

**Tech Stack:** Go、Engine、queryexec ColumnStream、LocalMetadataStore、observability、faultinject、tests/e2e、tests/fault、tests/scale。

---

## 文件职责

- `internal/engine/downsample_executor.go`：bounded 目标点构造和分批写入。
- `internal/engine/downsample_metadata.go`：dry-run 估算、cleanup API、drop/reset 目标数据治理。
- `internal/engine/downsample_validate.go`：扩展 policy 支持函数。
- `internal/queryexec/group_aggregate_state.go`：确认 group aggregate 函数支持和错误语义。
- `internal/observability/metrics.go`、`prometheus_text.go`：label metrics。
- `internal/engine/metrics.go`、`engine.go`：标准 per-policy metrics 和 status 采集错误指标。
- `types.go`、`downsample.go`：public API 与模型转换。
- `tests/fault/downsample_policy`、`tests/scale/downsample_policy`：故障和规模报告入口。
- `docs/storage/downsample-runbook.md`、`docs/query/builder-aggregate-functions.md`：边界和运维文档。

## Task 1: bounded downsample executor

**状态:** 已完成。

**EARS:** EARS-1、EARS-2。

**验收:** 高基数多函数窗口能按小 `BatchSize` 写出完整目标点；目标点字段完整；现有 executor 测试通过。

**步骤:**
- [x] 新增 executor 单测：多 series、多聚合、小 batch 时目标点字段完整且 rows 数正确。
- [x] 将 `downsampleIteratorToPoints` 替换为 bounded collector，按 timestamp/tags 聚合字段，达到 batch 后 flush。
- [x] 保留无法安全提前 flush 的序列函数正确性，确保错误时不推进 watermark。
- [x] 运行 `timeout 180s go test ./internal/engine -run 'TestDownsampleExecutor' -count=1 -timeout 3m`。

**实现备注:** 已新增 `internal/engine/downsample_aggregate.go`，executor 改为读取源列并在降采样层聚合，删除旧的 query aggregate 输出合并死代码。目标点按 `BatchSize` 写入，函数错误会返回并阻止 watermark 推进。

## Task 2: dry-run cost estimate

**状态:** 已完成。

**EARS:** EARS-3。

**验收:** dry-run 返回 group、sample、point 估算；高基数 policy 不再只返回窗口数量。

**步骤:**
- [x] 扩展 `DownsampleDryRunResult` internal/public 字段。
- [x] 新增 dry-run 估算单测。
- [x] 实现源列扫描估算 group cardinality 和样本数。
- [x] 运行 `timeout 180s go test ./internal/engine -run 'TestEngineDownsampleRangeDryRunRepairAndBackfill|TestDryRun' -count=1 -timeout 3m`。

**实现备注:** `DryRunDownsamplePolicy` 已返回 `GroupsEstimate`、`SamplesEstimate`、`PointsEstimate` 和 `EstimateComplete`，估算通过源列扫描完成，不写目标 RP，不推进 watermark。

## Task 3: labeled metrics and collection errors

**状态:** 已完成。

**EARS:** EARS-4。

**验收:** Prometheus 文本输出固定 metric name + `{policy="..."}`；status 采集错误有指标。

**步骤:**
- [x] 为 registry 增加 label key、label 输出和测试。
- [x] 将 downsample per-policy metrics 改为 label 模型。
- [x] 在 `MetricsSnapshot` 记录 status collection error。
- [x] 运行 `timeout 180s go test ./internal/observability ./internal/engine -run 'TestRegistry|TestDownsamplePolicyStatusesAndMetrics|TestDownsampleMetrics' -count=1 -timeout 3m`。

**实现备注:** observability registry 已支持 labels，per-policy metrics 改为固定名称加 `policy` label，`MetricsSnapshot` 会记录 `mts_downsample_status_collection_errors_total`。

## Task 4: cleanup API and fault matrix

**状态:** 已完成。

**EARS:** EARS-5、EARS-6。

**验收:** drop/reset 可选清理目标 rollup；写失败、metadata 失败、cancel、reset replace 有测试覆盖。

**步骤:**
- [x] 扩展 reset/drop options 和 public API。
- [x] 实现按 policy tag 清理目标数据的 tombstone 写入。
- [x] 补充 unit/fault e2e。
- [x] 运行 `timeout 300s go test ./internal/engine ./tests/fault/downsample_policy -count=1 -timeout 5m`。

**实现备注:** 新增 `DropDownsamplePolicyWithOptions`，`DownsampleReset` 增加 cleanup 字段。cleanup 使用 policy tag 查询目标列并对匹配 series/field 写 tombstone。既有 fault e2e 和新增 unit cleanup 测试已覆盖核心路径。

## Task 5: function set expansion

**状态:** 已完成。

**EARS:** EARS-7。

**验收:** policy 校验支持查询执行器已有常见函数；非数值错误可观察且不写错数据。

**步骤:**
- [x] 扩展 supported functions。
- [x] 新增 validate 和 executor 测试覆盖 `rate/irate/difference/mode/stddev`。
- [x] 运行 `timeout 180s go test ./internal/engine ./internal/queryexec -run 'TestNormalizeDownsamplePolicy|TestDownsample|TestAggregate' -count=1 -timeout 3m`。

**实现备注:** policy 校验已支持 `rate/irate/increase/delta/difference/derivative/spread/mode/stddev/stdvar/top/bottom/median`，并复用窗口内正确聚合语义。新增 executor 测试覆盖点名函数。

## Task 6: scale report and docs

**状态:** 已完成。

**EARS:** EARS-8、EARS-9、EARS-10。

**验收:** scale JSON 增加 GC/disk/SSTable；文档明确显式 Builder/目标 RP 查询和 page-level rollup 边界。

**步骤:**
- [x] 扩展 scale report 字段和测试。
- [x] 更新 runbook 与 query 文档。
- [x] 运行 `timeout 300s go test ./tests/scale/downsample_policy -count=1 -timeout 5m`。

**实现备注:** scale report 已增加 heap、GC、disk bytes 和 SSTable count。public Builder 增加 `FromDownsamplePolicy(policy)` 显式查询入口；runbook 已说明 label metrics、cleanup、dry-run 估算和 page-level rollup 边界。

## Task 7: final verification

**状态:** 已完成。

**EARS:** 全部。

**验收:** 格式化、lint、全量测试通过；review 文档状态更新。

**步骤:**
- [x] 执行 `timeout 300s goimports-reviser -rm-unused -format ./...`。
- [x] 执行 `timeout 600s go test ./... -count=1 -timeout 10m`。
- [x] 执行 `timeout 720s golangci-lint run ./...`。
- [x] 执行 `git diff --check`。

**实现备注:** 已执行 goimports-reviser、全量 go test 和 golangci-lint，均通过。`git diff --check` 在收尾阶段执行。
