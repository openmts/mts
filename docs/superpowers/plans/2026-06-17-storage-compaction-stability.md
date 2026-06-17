# Compaction 长期稳定性与放大控制 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完整闭环专项三，使 mts compaction 在长期写入下可调度、可观测、可降载、可恢复，并能约束读放大、写放大和空间放大。

**Architecture:** 在现有 `engine` 存储边界内增强 compaction planner、scheduler、executor stats、metrics/health 和 scale 报告，不改变 SSTable 主体格式。Compaction 仍保持单 shard 生命周期锁下提交 Manifest，新增候选集合去重、level backlog/score、磁盘空间预检、手动任务状态和指标输出。故障与恢复继续通过 Manifest 原子切换、孤儿 part 清理和 WAL checkpoint 协议保证一致性。

**Tech Stack:** Go、现有 `internal/engine`、`internal/sstable`、`internal/observability`、`tests/e2e`、`tests/scale`。

**预计耗时与硬超时:** 计划与代码阅读 10 分钟；实现 90-150 分钟；单包测试每次 `-timeout 240s`；全量 `go test ./... -timeout 10m`；`golangci-lint run --timeout 10m`；每个 e2e build/run 使用 `timeout 120s`。

---

## EARS 覆盖矩阵

| EARS | 覆盖任务 |
| --- | --- |
| L0 part 数超过阈值触发 compaction | Task 1 |
| L0 总大小超过阈值生成计划 | Task 1 |
| L1+ overlap 暴露指标并触发修复 | Task 1、Task 6 |
| 同 level 不重叠时查询最多命中必要 part | Task 7 |
| 输出超过目标 part 大小时切分 | Task 4、Task 8 |
| 下一级达到上限时级联并限制步数 | Task 1、Task 3 |
| 前台写入压力升高时后台降并发或暂停 | Task 3 |
| backlog 超阈值 health degraded | Task 2、Task 6 |
| retention 与 compaction 使用调度锁避免同 part 冲突 | Task 3、Task 8 |
| reader 引用旧文件时允许继续读取到关闭 | Task 8 |
| compaction 错误保留输入并清理输出 | Task 4、Task 8 |
| 中途退出重启只加载 Manifest 并清理孤儿输出 | Task 8 |
| Manifest 成功后 checkpoint 或记录安全删除旧 part | Task 4、Task 6 |
| planner 记录 level、candidate count、input bytes、output estimate、score、reason | Task 1、Task 6 |
| executor 记录 active、duration、bytes、dropped rows、errors | Task 2、Task 4、Task 6 |
| tombstone/重复点保留最新 writeSeq 并删除样本 | Task 8 |
| 损坏 part 返回错误并停止提交输出 | Task 8 |
| 不同 level compression 策略 | Task 4 |
| 长期大量小 SSTable 维持可配置 part 数 | Task 7 |
| 读放大超预算优先调度目标 shard/level compaction | Task 1、Task 3 |
| 磁盘空间不足拒绝启动 compaction | Task 4 |
| 手动 compact 返回任务状态、耗时、影响 part 数和错误 | Task 5 |
| 后台周期运行避免重复选择同一候选集合 | Task 3 |

## 文件结构

- Modify: `internal/model/types.go`，扩展 compaction 配置与结果类型。
- Modify: `internal/engine/compaction_planner.go`，新增 plan reason、score、estimate、backlog、读放大优先级和候选签名。
- Modify: `internal/engine/compaction_stats.go`，新增 planner/executor 快照、task status、dropped rows、backlog、score、error 统计。
- Modify: `internal/engine/lifecycle.go`，接入调度 guard、磁盘预检、stats recorder、手动结果。
- Modify: `internal/engine/background.go`，后台 compaction 支持降载、去重和 health backlog。
- Modify: `internal/engine/engine.go`，暴露手动 compact 结果、health、metrics 汇总。
- Modify: `internal/engine/metrics.go`，输出 compaction planner/executor/backlog/health 指标。
- Modify: `internal/engine/read_amplification.go`，补充 level score 与严格读放大调度入口。
- Modify: `internal/engine/ports.go`，为磁盘空间与文件删除注入测试口。
- Modify/Test: `internal/engine/*_test.go`，补齐 TDD 单测。
- Modify: `tests/scale/storage_10m/main.go`、`tests/scale/storage_soak/main.go`，输出机器可读放大与 level 分布数据。
- Create/Modify: `tests/e2e/compaction_integrity` 或新增 `tests/e2e/compaction_longrun`，覆盖调度、恢复、metrics、读放大和结果正确性。

---

## Task 1: Planner 评分、backlog 与优先级

**EARS:** L0 part 数阈值、L0 size 阈值、L1+ overlap 修复、级联前置、读放大超预算优先调度、planner 记录 level/candidate/input/output/score/reason。

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/engine/compaction_planner_test.go`
  - 新增测试：
    - `TestNextCompactionPlanReportsReasonScoreAndEstimates`
    - `TestNextCompactionPlanPrioritizesReadAmplificationLevel`
    - `TestCompactionBacklogSnapshotCountsLevelsAndOverlaps`
  - 命令：`timeout 240s go test ./internal/engine -run 'TestNextCompactionPlanReportsReasonScoreAndEstimates|TestNextCompactionPlanPrioritizesReadAmplificationLevel|TestCompactionBacklogSnapshotCountsLevelsAndOverlaps' -timeout 240s`
  - Expected: FAIL，原因是 plan 还没有 reason/score/backlog/priority 字段或函数。

- [x] **Step 2: 实现 planner metadata**
  - `compactionPlan` 增加 `reason string`、`score float64`、`inputBytes int64`、`outputEstimateBytes int64`、`candidateSignature string`。
  - `nextCompactionPlan` 通过 `buildCompactionBacklog` 选择最高分计划，优先处理读放大超预算或 overlap。
  - `compactionTriggered` 拆分为返回 reason/score，避免 bool 难以观测。

- [x] **Step 3: 实现 backlog snapshot**
  - 新增内部类型 `compactionBacklogSnapshot`，包含总 pending、level stats、max score、overlap count、estimated input/output。
  - 估算输出：默认等于 input bytes；存在 tombstone 时使用 input bytes 的 80% 下限估计，避免使用 0 掩盖空间需求。

- [x] **Step 4: 运行验证**
  - 命令：`timeout 240s go test ./internal/engine -run 'Test.*CompactionPlan|Test.*CompactionBacklog' -timeout 240s`
  - Expected: PASS。

- [x] **实现备注:** 已新增 compaction plan reason、score、input/output bytes estimate、candidate signature 和 backlog snapshot。`nextCompactionPlan` 通过 backlog 选择最高分计划，读放大层级会叠加 overlap 分数以优先修复查询影响更大的层级。验证：`timeout 240s go test ./internal/engine -run 'Test.*CompactionPlan|Test.*CompactionBacklog' -timeout 240s` 通过。

## Task 2: Executor stats 与 task status

**EARS:** executor active/duration/input/output/dropped rows/error，manual compact 状态的底层统计，Manifest 后 checkpoint/安全删除可观测。

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/engine/compaction_stats_test.go`
  - 新增测试：
    - `TestCompactionStatsRecorderTracksPlanAndDroppedRows`
    - `TestCompactionTaskStatusSnapshotReportsLastTask`
  - 命令：`timeout 240s go test ./internal/engine -run 'TestCompactionStatsRecorderTracksPlanAndDroppedRows|TestCompactionTaskStatusSnapshotReportsLastTask' -timeout 240s`
  - Expected: FAIL，原因是 stats 未记录 plan reason/score/dropped rows/task status。

- [x] **Step 2: 扩展 stats**
  - `CompactionStats` 增加 `Backlog`、`MaxScore`、`LastReason`、`LastLevel`、`LastOutputLevel`、`DroppedRows`、`LastTask`。
  - 新增 `CompactionTaskStatus`，包含 `ID`、`State`、`StartedAt`、`FinishedAt`、`Duration`、`InputParts`、`OutputParts`、`InputBytes`、`OutputBytes`、`DroppedRows`、`Error`。

- [x] **Step 3: 在 executor 填充 dropped rows**
  - compaction merge 前后计算输入样本与输出样本差值。
  - 失败时保持输入 part 不变，stats 只记录失败 task。

- [x] **Step 4: 运行验证**
  - 命令：`timeout 240s go test ./internal/engine -run 'TestCompactionStats|TestCompactionStreamsOneSeriesAtATime' -timeout 240s`
  - Expected: PASS。

- [x] **实现备注:** 已扩展 `CompactionStats` 和 `CompactionTaskStatus`，新增 `beginPlan`、`finishWithRows`，并在 streaming compaction 中统计 merge/tombstone 后被剔除的样本数。验证：`timeout 240s go test ./internal/engine -run 'TestCompactionStats|TestCompactionStreamsOneSeriesAtATime|TestStreamingCompactionAbortsOpenOutputOnBatchQueryError' -timeout 240s` 通过。

## Task 3: 调度 guard、后台去重与降载

**EARS:** 前台写入压力升高时后台降并发或暂停、backlog degraded、retention/compaction 锁、后台周期避免重复候选、级联步数限制。

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/engine/engine_test.go`
  - 新增测试：
    - `TestBackgroundCompactionSkipsWhenStorageMemoryBusy`
    - `TestCompactionSchedulerRejectsDuplicateCandidateSignature`
    - `TestApplyRetentionWaitsForShardLifecycleLock`
  - 命令：`timeout 240s go test ./internal/engine -run 'TestBackgroundCompactionSkipsWhenStorageMemoryBusy|TestCompactionSchedulerRejectsDuplicateCandidateSignature|TestApplyRetentionWaitsForShardLifecycleLock' -timeout 240s`
  - Expected: FAIL，原因是后台没有 busy 检查和 candidate in-flight 去重。

- [x] **Step 2: 实现调度状态**
  - `Engine` 增加 `compactionScheduler`，内部维护 in-flight candidate signatures、skip counters、last skip reason。
  - `Shard` compaction 前先注册 signature，结束后释放。
  - retention 继续使用 `lifecycleMu`，补测试证明不会和 compaction 同时删除同一 shard part。

- [x] **Step 3: 实现后台降载**
  - storage memory 当前 reservations 或 active bytes 达到 soft limit 时，后台 compaction 跳过本轮并记录 stats。
  - 手动 compact 不被后台 busy 策略跳过，但仍受硬预算和磁盘预检约束。

- [x] **Step 4: 运行验证**
  - 命令：`timeout 240s go test ./internal/engine -run 'TestBackgroundCompaction|TestCompactionScheduler|TestApplyRetention' -timeout 240s`
  - Expected: PASS。

- [x] **实现备注:** 已新增 `compactionScheduler`，用 candidate signature 防止重复调度同一候选集合；后台 compaction 在调用原 `Compact()` 前检查 storage memory soft limit，忙时跳过并记录 skip reason；retention 与 compaction 继续通过 shard lifecycle 锁串行。验证：`timeout 240s go test ./internal/engine -run 'TestBackgroundCompactionSkipsWhenStorageMemoryBusy|TestCompactionSchedulerRejectsDuplicateCandidateSignature|TestApplyRetentionWaitsForShardLifecycleLock|TestBackgroundCompactionLifecycle' -timeout 240s` 通过。

## Task 4: 磁盘空间预检、输出切分与 level compression 证明

**EARS:** 磁盘空间不足拒绝启动、输出按目标大小切分、不同 level compression、错误清理输出保留输入、Manifest 成功后记录安全删除。

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/engine/engine_test.go`
  - 新增测试：
    - `TestCompactionRejectsWhenDiskSpaceInsufficient`
    - `TestCompactionUsesTargetLevelCompressionAndSplitLimit`
    - `TestCompactionRecordsSafeDeleteAfterManifestCommit`
  - 命令：`timeout 240s go test ./internal/engine -run 'TestCompactionRejectsWhenDiskSpaceInsufficient|TestCompactionUsesTargetLevelCompressionAndSplitLimit|TestCompactionRecordsSafeDeleteAfterManifestCommit' -timeout 240s`
  - Expected: FAIL，原因是文件接口没有可注入 disk free，safe-delete 统计不存在。

- [x] **Step 2: 扩展文件接口**
  - `fileOps` 增加 `AvailableBytes(path string) (int64, error)`。
  - `defaultFileOps` 使用 `syscall.Statfs` 读取可用空间。
  - `CompactionOptions` 增加 `DiskSpaceReserveBytes` 和 `MinFreeBytes`，默认 0 表示只使用估算输出大小预检。

- [x] **Step 3: 接入 preflight**
  - compaction 写输出前检查 `available >= outputEstimate + reserve + minFree`。
  - 空间不足返回明确错误，不修改 Manifest，不删除输入。

- [x] **Step 4: 运行验证**
  - 命令：`timeout 240s go test ./internal/engine -run 'TestCompaction.*Disk|TestCompaction.*Compression|TestCompaction.*Output|TestCompaction.*Manifest' -timeout 240s`
  - Expected: PASS。

- [x] **实现备注:** 已在 `fileOps` 增加 `AvailableBytes`，默认实现使用 `syscall.Statfs`；compaction 写输出前检查 `outputEstimate + reserve + minFree`，空间不足返回 `ErrCompactionDiskSpaceExceeded` 且不切换 Manifest；成功删除旧 part 后记录 `SafeDeleteParts`。验证：`timeout 240s go test ./internal/engine -run 'TestCompactionRejectsWhenDiskSpaceInsufficient|TestCompactionUsesTargetLevelCompressionAndSplitLimit|TestCompactionRecordsSafeDeleteAfterManifestCommit|TestCompactionOldPartDeleteFailureKeepsNewManifestAndRecordsMaintenance' -timeout 240s` 通过。

## Task 5: 手动 Compact API 返回任务结果

**EARS:** 手动 compact 返回任务状态、耗时、影响 part 数和错误。

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/engine/engine_test.go`
  - 新增测试：
    - `TestEngineCompactWithResultReportsTaskStatus`
    - `TestShardCompactWithResultReportsNoop`
  - 命令：`timeout 240s go test ./internal/engine -run 'TestEngineCompactWithResultReportsTaskStatus|TestShardCompactWithResultReportsNoop' -timeout 240s`
  - Expected: FAIL，原因是只有 `Compact(context.Context) error`。

- [x] **Step 2: 新增 API**
  - 新增 `Engine.CompactWithResult(ctx context.Context) (model.CompactionResult, error)`。
  - 保持 `Engine.Compact(ctx)` 向后兼容，内部调用 `CompactWithResult` 并只返回 error。
  - `Shard.CompactWithResult()` 返回 shard 级状态，包含 noop、success、failed。

- [x] **Step 3: 运行验证**
  - 命令：`timeout 240s go test ./internal/engine -run 'TestEngineCompact|TestShardCompact|TestBackgroundCompactionLifecycle' -timeout 240s`
  - Expected: PASS。

- [x] **实现备注:** 已新增 `CompactionResult`、`Engine.CompactWithResult` 和 `Shard.CompactWithResult`，旧 `Compact()` 内部调用新 API 并只返回错误。结果包含状态、耗时、输入/输出 part、bytes、dropped rows 和 last task。验证：`timeout 240s go test ./internal/engine -run 'TestEngineCompactWithResultReportsTaskStatus|TestShardCompactWithResultReportsNoop|TestEngineCompact|TestBackgroundCompactionLifecycle' -timeout 240s` 通过。

## Task 6: Metrics 与 health degraded

**EARS:** backlog degraded、planner/executor metrics、overlap metrics、health degraded。

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/engine/engine_test.go`、`internal/engine/read_amplification_test.go`
  - 新增测试：
    - `TestEngineMetricsSnapshotIncludesCompactionSignals`
    - `TestEngineHealthSnapshotDegradedByCompactionBacklog`
    - `TestLevelHealthExposesOverlapAndScore`
  - 命令：`timeout 240s go test ./internal/engine -run 'TestEngineMetricsSnapshotIncludesCompactionSignals|TestEngineHealthSnapshotDegradedByCompactionBacklog|TestLevelHealthExposesOverlapAndScore' -timeout 240s`
  - Expected: FAIL，原因是 metrics 只有 storage memory，health 不含 compaction。

- [x] **Step 2: 实现 metrics**
  - 新增 `recordCompactionMetrics`，输出 active、backlog、total、success、failure、input/output bytes、dropped rows、last duration、last score、overlap count。
  - `MetricsSnapshot` 合并 storage memory 与 compaction metrics。

- [x] **Step 3: 实现 health snapshot**
  - 新增 `Engine.HealthSnapshot()`，返回包含 `Healthy`、`Ready`、`Reasons` 的 engine 级结构，避免依赖 service 包造成循环导入。
  - backlog 超过 `CompactionOptions.BacklogDegradedThreshold` 或 level overlap 存在时标记 degraded。

- [x] **Step 4: 运行验证**
  - 命令：`timeout 240s go test ./internal/engine -run 'TestEngine.*Metrics|TestEngine.*Health|TestLevelHealth' -timeout 240s`
  - Expected: PASS。

- [x] **实现备注:** 已新增 compaction metrics 输出和 engine `HealthSnapshot`，并在 stats snapshot 中合并 backlog、overlap 与 scheduler skip。health 在 backlog degraded、overlap 或 maintenance error 时降级。验证：`timeout 240s go test ./internal/engine -run 'TestEngineMetricsSnapshotIncludesCompactionSignals|TestEngineHealthSnapshotDegradedByCompactionBacklog|TestLevelHealthExposesOverlapAndScore|TestEngineCompactionStatsSnapshotAggregatesShards' -timeout 240s` 通过。

## Task 7: 长期 scale 报告与读/写/空间放大门禁

**EARS:** 长期小 SSTable part 数可控、同 level 不重叠降低读放大、10M+ 报告包含写放大、空间放大和查询延迟。

- [x] **Step 1: 写 failing tests**
  - 文件：`tests/scale/storage_10m/main_test.go`、`tests/scale/storage_soak/main_test.go`
  - 新增测试：
    - `TestReportIncludesAmplificationAndLevelDistribution`
    - `TestSoakReportIncludesCompactionHealth`
  - 命令：`timeout 240s go test ./tests/scale/... -run 'TestReportIncludesAmplificationAndLevelDistribution|TestSoakReportIncludesCompactionHealth' -timeout 240s`
  - Expected: FAIL，原因是报告缺少 amplification 与 level distribution 字段。

- [x] **Step 2: 扩展报告**
  - `storage_10m` 输出 `level_distribution`、`read_parts`、`read_amplification`、`write_amplification`、`space_amplification`、`query_latency_nanos`、`compaction_stats`。
  - `storage_soak` 输出 rows、part count、level distribution、health degraded、compaction backlog。

- [x] **Step 3: 运行小规模验证**
  - 命令：`timeout 240s go test ./tests/scale/... -timeout 240s`
  - 命令：`timeout 300s go run ./tests/scale/storage_10m -mode compact -points 10000 -batch-size 1000`
  - Expected: JSON 中包含上述字段且查询结果正确。

- [x] **实现备注:** 已扩展 public `CompactionStatsSnapshot`、`CompactWithResult` 和 `HealthSnapshot` wrapper；`storage_10m` 输出 level distribution、read/write/space amplification、query latency 和 compaction stats；`storage_soak` 输出 part count、level distribution、health degraded 和 backlog。验证：`timeout 240s go test ./tests/scale/... -run 'TestReportIncludesAmplificationAndLevelDistribution|TestSoakReportIncludesCompactionHealth|TestRunWorkloadModes' -timeout 240s` 通过；`timeout 300s go run ./tests/scale/storage_10m -mode compact -points 10000 -batch-size 1000` 输出包含全部机器可读字段。

## Task 8: 故障、恢复、reader 与 e2e 闭环

**EARS:** reader 引用旧文件继续读、错误保留输入清理输出、重启清理孤儿、tombstone/重复点、损坏 part、输出分片、retention 冲突。

- [x] **Step 1: 写 failing/evidence tests**
  - 文件：`tests/e2e/compaction_integrity/main.go`、`tests/e2e/compaction_integrity/main_test.go`
  - 覆盖：
    - 查询 reader 打开后 compaction 删除旧 part，reader 仍可完成。
    - corrupt part compaction 返回错误，Manifest 未切换。
    - restart 后孤儿输出 part 被清理，Manifest 引用 parts 可读。
    - tombstone 与重复点保留最新 writeSeq。
  - 命令：`cd tests/e2e/compaction_integrity && timeout 120s go test ./... -timeout 120s`
  - Expected: RED 或 evidence FAIL，直到实现/补齐后变 GREEN。

- [x] **Step 2: 补齐实现缺口**
  - 如果 reader 生命周期缺少引用保护，则通过 Manifest 先切换、旧 part 延迟删除失败记录维护错误，保证已有 reader 文件句柄继续可读。
  - 如果 tombstone/drop 统计缺失，则接入 Task 2 dropped rows。

- [x] **Step 3: e2e build/run**
  - 命令：`timeout 900s bash -c 'set -euo pipefail; for dir in tests/e2e/*; do [ -d "$dir" ] || continue; (cd "$dir" && go build -o testbin . && timeout 120s ./testbin; status=$?; rm -f testbin; exit $status); done'`
  - Expected: PASS，且无 `testbin` 遗留。

- [x] **实现备注:** 已增强 `tests/e2e/compaction_integrity`，新增 reader 持锁期间 compaction 等待且 reader 可继续读、corrupt part compaction 失败且 Manifest 不切换、restart 清理孤儿 part、tombstone compact 后删除样本被剔除四个场景。定向验证：`cd tests/e2e/compaction_integrity && timeout 120s go test ./... -run 'TestReaderCorruptAndOrphanScenarios|TestRun' -timeout 120s` 通过。全量 e2e build/run：`timeout 900s bash -c 'set -euo pipefail; for dir in tests/e2e/*; do [ -d "$dir" ] || continue; (cd "$dir" && go build -o testbin . && timeout 120s ./testbin; status=$?; rm -f testbin; exit $status); done'` 通过。

## Final Verification

- [x] `timeout 300s goimports-reviser -rm-unused -format ./...`
- [x] `timeout 600s go test ./... -timeout 10m`
- [x] `timeout 600s golangci-lint run --timeout 10m`
- [x] `timeout 900s bash -c 'set -euo pipefail; for dir in tests/e2e/*; do [ -d "$dir" ] || continue; (cd "$dir" && go build -o testbin . && timeout 120s ./testbin; status=$?; rm -f testbin; exit $status); done'`
- [x] `timeout 60s git diff --check`
- [x] `timeout 60s find . -type f \( -name testbin -o -name "*.prof" -o -name "*.cover" -o -name coverage.out \) -print`
- [x] 更新本计划所有任务状态与实现备注。
- [ ] 提交：`feat(storage): 完善 compaction 长稳控制`
