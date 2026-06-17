# 商用级查询执行器与语义完整性 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完整闭环专项四，使 mts 查询执行器具备语义完整、可取消、可下推、可统计、可 explain、可控内存的商用级能力。

**Architecture:** 保留现有 `engine -> queryexec -> memtable/sstable` 流式边界，新增查询计划/统计上下文和 streaming row executor。Catalog 阶段解析 series/field，Engine 阶段生成 explain plan 并统计 shard/part/page/samples，Shard/SSTable 阶段继续执行 metadata、series index、fieldID、value page 下推。Row 查询改为基于 column stream 的 bounded row merger，避免 `QueryRows` 先全量物化 columns。

**Tech Stack:** Go、`internal/engine`、`internal/queryexec`、`internal/sstable`、`tests/e2e`、`tests/pprof`。

**预计耗时与硬超时:** 计划与代码阅读 10 分钟；实现 90-150 分钟；单包测试每次 `-timeout 240s`；全量 `go test ./... -timeout 10m`；`golangci-lint run --timeout 10m`；每个 e2e build/run 使用 `timeout 120s`。

---

## EARS 覆盖矩阵

| EARS | 覆盖任务 |
| --- | --- |
| database/rp/measurement/tags 在 catalog 阶段解析精确 seriesID | Task 1、Task 6 |
| fields 在进入 SSTable value page 前完成 fieldID 过滤 | Task 1、Task 3、Task 6 |
| 时间范围跳过不相交 shard | Task 1、Task 6 |
| 时间范围通过 PartMeta 跳过不相交 part | Task 3、Task 6 |
| seriesID 通过 SSTable series index 定位 index row | Task 3 |
| 时间范围只读相交 value page | Task 3 |
| context cancellation 覆盖 catalog/shard/part/page/merge/aggregate/row | Task 2、Task 5、Task 6 |
| deadline 返回 `context.DeadlineExceeded` 并关闭 reader | Task 2、Task 5、Task 6 |
| limit/offset 流式早停 | Task 2、Task 4 |
| count/sum/min/max/avg/first/last 聚合按 series/field/window 正确输出 | Task 5、Task 6 |
| 聚合不支持字段类型返回明确错误 | Task 5 |
| 窗口跨 shard/part 正确合并边界 | Task 5、Task 6 |
| 乱序样本输出按 timestamp 排序 | Task 5 |
| 重复 timestamp 保留最新 writeSeq | Task 5、Task 6 |
| tombstone 过滤删除样本 | Task 5、Task 6 |
| row 查询按 `(seriesID,timestamp)` 流式合并字段，避免全量 map 物化 | Task 4 |
| MaxSamples 超限返回读预算错误 | Task 2、Task 6 |
| query stats 记录 shard/part/index row/value page/samples | Task 3、Task 6 |
| metadata 可判断空时避免打开 value 文件 | Task 3 |
| first/last 边界快路径 | Task 5 |
| 内部节点出错通过 iterator `Err()` 暴露并停止后续节点 | Task 2、Task 5 |
| iterator `Close()` 释放 snapshot/part payload/page buffer/聚合状态 | Task 2、Task 4、Task 5 |
| 并发查询避免共享可变 buffer 数据竞争 | Task 4、Task 6 |
| 多 level 重叠按 writeSeq/tombstone 合并 | Task 5、Task 6 |
| 查询计划输出 explain 信息 | Task 1、Task 6 |

## 文件结构

- Modify: `internal/model/types.go`，新增 `QueryStats`、`QueryExplain`、`QueryOptions` 或扩展 `Query`。
- Modify: `types.go`、`engine.go`，导出 query stats/explain 和 streaming row wrapper。
- Modify: `internal/engine/query.go`，生成计划、解释下推条件、接入 stats recorder、context 检查、streaming row iterator。
- Create/Modify: `internal/engine/query_stats.go`，聚合 stats/explain 和可测试 hook。
- Modify: `internal/engine/shard_scan.go`，统计 shard/part skip、samples，向下传递 query stats。
- Modify: `internal/sstable/types.go`、`internal/sstable/scan.go`、`internal/sstable/read.go`，公开 query stats recorder，记录 index row/value page 读取与跳过。
- Modify: `internal/queryexec/*`，补 context-aware stream、row merge stream、pagination early stop、aggregate close/error 语义。
- Modify/Test: `internal/engine/*_test.go`、`internal/queryexec/*_test.go`、`internal/sstable/*_test.go`。
- Modify/Create: `tests/e2e/query_semantics` 或扩展现有 `query_aggregate_window`、`query_pruning`、`read_amplification`。
- Modify: `tests/pprof/storage_engine`，增加 row streaming / query stats 入口或测试。

---

## Task 1: 查询计划与 Explain

**EARS:** catalog 精确 seriesID、fieldID 下推、shard 跳过、查询计划 explain。

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/engine/query_plan_test.go`
  - 新增测试：
    - `TestBuildQueryPlanExplainsCatalogAndShardPruning`
    - `TestBuildQueryPlanReturnsEmptyWhenCatalogMisses`
  - 命令：`timeout 240s go test ./internal/engine -run 'TestBuildQueryPlanExplainsCatalogAndShardPruning|TestBuildQueryPlanReturnsEmptyWhenCatalogMisses' -timeout 240s`
  - Expected: FAIL，原因是 `BuildQueryPlan` / `QueryExplain` 不存在。

- [x] **Step 2: 实现 query plan**
  - 新增 `QueryExplain`：database、retention policy、measurement、tag filters、field filters、series count、field count、candidate shards、matched shards、skipped shards、pushdowns、budget。
  - Engine 查询入口先生成 plan，再将 `seriesIDs`、`fieldIDs`、`matchedShards` 传入执行链。

- [x] **Step 3: 运行验证**
  - 命令：`timeout 240s go test ./internal/engine -run 'TestBuildQueryPlan|TestEngineLifecycleAndQueries' -timeout 240s`
  - Expected: PASS。

- [x] **实现备注:** 已新增 `model.QueryExplain`、`engine.QueryPlan` 和 `Engine.BuildQueryPlan`，将 catalog series/field 解析、shard 时间裁剪、预算和 pushdown 信息写入 explain；`QueryColumnIterator` 改为复用 plan。验证：`timeout 240s go test ./internal/engine -run 'TestBuildQueryPlanExplainsCatalogAndShardPruning|TestBuildQueryPlanReturnsEmptyWhenCatalogMisses|TestEngineLifecycleAndQueries' -timeout 240s` 通过。

## Task 2: Query context、deadline 和 iterator 错误传播

**EARS:** cancellation/deadline 覆盖 catalog、merge、aggregate、row materialization；内部错误通过 iterator `Err()` 暴露；Close 释放资源。

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/queryexec/context_test.go`、`internal/engine/query_context_test.go`
  - 新增测试：
    - `TestContextColumnStreamStopsAndClosesInnerOnDeadline`
    - `TestQueryColumnIteratorReturnsDeadlineDuringCatalog`
    - `TestAggregateStreamPropagatesSourceErrorAndClose`
  - 命令：`timeout 240s go test ./internal/queryexec ./internal/engine -run 'TestContextColumnStreamStopsAndClosesInnerOnDeadline|TestQueryColumnIteratorReturnsDeadlineDuringCatalog|TestAggregateStreamPropagatesSourceErrorAndClose' -timeout 240s`
  - Expected: FAIL，原因是 column series stream/aggregate 还缺 context wrapper 或 close 状态。

- [x] **Step 2: 实现 context-aware streams**
  - 新增 `WithContextColumnStream`、`WithContextRowStream`。
  - `QueryColumnIterator` 在 catalog 前后、plan 构建后、stream 装饰/聚合/分页前检查 context。
  - `QueryRowIterator` 使用 context-aware row stream。

- [x] **Step 3: 运行验证**
  - 命令：`timeout 240s go test ./internal/queryexec ./internal/engine -run 'Test.*Context|Test.*Deadline|TestAggregate.*Error' -timeout 240s`
  - Expected: PASS。

- [x] **实现备注:** 已新增 `WithContextColumnStream`、`WithContextRowStream`，并将 `WithContextColumnDataStream` 改为 context 取消后幂等关闭底层 stream；`QueryColumnIterator` 在 plan 之后、raw stream 创建后和最终 column stream 外层检查/包装 context，`QueryRowIterator` 使用 context-aware row stream。验证：`timeout 240s go test ./internal/queryexec ./internal/engine -run 'Test.*Context|Test.*Deadline|TestAggregate.*Error' -timeout 240s` 通过。

## Task 3: Query stats 与 part/page 读放大统计

**EARS:** stats 记录 shard、part、index row、value page、samples；metadata 空结果避免打开 value 文件；PartMeta/series index/page 下推可验证。

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/engine/query_stats_test.go`、`internal/sstable/query_stats_test.go`
  - 新增测试：
    - `TestQueryStatsRecordsShardPartPageAndSampleCounts`
    - `TestPartScanStatsRecordsSkippedPagesAndIndexRows`
    - `TestMetadataEmptyQueryDoesNotReadValues`
  - 命令：`timeout 240s go test ./internal/engine ./internal/sstable -run 'TestQueryStatsRecordsShardPartPageAndSampleCounts|TestPartScanStatsRecordsSkippedPagesAndIndexRows|TestMetadataEmptyQueryDoesNotReadValues' -timeout 240s`
  - Expected: FAIL，原因是 stats recorder 不存在或未公开。

- [x] **Step 2: 实现 stats recorder**
  - `model.QueryStats` 增加 scanned/skipped shards、parts、index rows、value pages、samples、errors。
  - `sstable.Query` 增加 stats recorder 接口，Part 扫描时记录 reads/skips。
  - `Engine.QueryStatsSnapshot()` 返回最近一次 query stats，`QueryWithExplain` 返回结果和 stats。

- [x] **Step 3: 运行验证**
  - 命令：`timeout 240s go test ./internal/engine ./internal/sstable -run 'Test.*QueryStats|Test.*MetadataEmpty|Test.*ReadStats' -timeout 240s`
  - Expected: PASS。

- [x] **实现备注:** 已新增 `model.QueryStats`、Engine 最近查询 stats 快照和最终 column stream 统计 wrapper；`sstable.Query`/`memtable.Query` 向下传递 stats 指针，SSTable 在 part metadata 过滤、index row、time/value block、value page 和样本读取处记录读放大。验证：`timeout 240s go test ./internal/engine ./internal/sstable -run 'Test.*QueryStats|Test.*MetadataEmpty|Test.*ReadStats|TestPartQueryReadsOnlyMatchingValuePages|TestPartSeriesIndexReadsOnlyMatchingIndexRows' -timeout 240s` 通过。

## Task 4: Streaming Row Executor 与 limit/offset 早停

**EARS:** row 查询流式合并 `(seriesID,timestamp)`，limit/offset 早停，避免全量 map 物化，Close 释放状态，并发查询无共享可变 buffer。

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/queryexec/row_merge_test.go`、`internal/engine/query_row_stream_test.go`
  - 新增测试：
    - `TestRowMergeStreamMergesColumnsBySeriesAndTimestamp`
    - `TestQueryRowIteratorDoesNotMaterializeAllColumnsBeforeNext`
    - `TestRowIteratorLimitStopsSourceEarly`
    - `TestConcurrentRowIteratorsDoNotShareMutableRows`
  - 命令：`timeout 240s go test ./internal/queryexec ./internal/engine -run 'TestRowMergeStream|TestQueryRowIteratorDoesNotMaterializeAllColumnsBeforeNext|TestRowIteratorLimitStopsSourceEarly|TestConcurrentRowIteratorsDoNotShareMutableRows' -timeout 240s`
  - Expected: FAIL，原因是 row iterator 仍走 `QueryColumns` 全量物化。

- [x] **Step 2: 实现 row stream**
  - 新增 `queryexec.NewRowMergeStream(ColumnStream, Query)`。
  - `Engine.QueryRowIterator` 直接基于 column iterator 构建 row stream，不再调用 `QueryColumns`。
  - limit/offset 在 row stream 层早停并关闭上游。

- [x] **Step 3: 运行验证**
  - 命令：`timeout 240s go test ./internal/queryexec ./internal/engine -run 'Test.*Row.*Stream|TestQueryRows|TestColumnsToRows' -timeout 240s`
  - Expected: PASS。

- [x] **实现备注:** 已新增 `queryexec.NewRowMergeStream`，按 series 分组、按 timestamp 合并字段，并在 row 层执行 offset/limit；limit 达到后立即关闭上游 column stream。`Engine.QueryRowIterator` 不再调用 `QueryColumns` 全量物化，而是复用 column iterator 并清空 column 层分页。验证：`timeout 240s go test ./internal/queryexec ./internal/engine -run 'Test.*Row.*Stream|TestQueryRows|TestColumnsToRows|TestEngineLifecycleAndQueries|TestQueryRowIterator' -timeout 240s` 通过。

## Task 5: 聚合、排序、重复点、tombstone 和 first/last 快路径语义

**EARS:** 聚合函数完整、类型错误明确、跨 shard/part window、乱序排序、重复 timestamp LWW、tombstone、多 level 重叠、first/last 快路径。

- [x] **Step 1: 写 failing tests**
  - 文件：`internal/queryexec/aggregate_semantics_test.go`、`internal/engine/query_semantics_test.go`
  - 新增测试：
    - `TestAggregateWindowMergesAcrossShardAndPartBoundaries`
    - `TestQueryDeduplicatesOverlappingLevelsByWriteSeq`
    - `TestFirstLastAggregatesUseBoundaryTimestamps`
    - `TestQueryTombstoneFiltersOverlappingLevels`
  - 命令：`timeout 240s go test ./internal/queryexec ./internal/engine -run 'TestAggregateWindowMergesAcrossShardAndPartBoundaries|TestQueryDeduplicatesOverlappingLevelsByWriteSeq|TestFirstLastAggregatesUseBoundaryTimestamps|TestQueryTombstoneFiltersOverlappingLevels' -timeout 240s`
  - Expected: FAIL，原因是跨 column/window 全局聚合和 first/last 快路径仍不完整。

- [x] **Step 2: 补强语义**
  - 在 merge 阶段统一按 series/field/timestamp/writeSeq 合并。
  - 聚合前保证每列 timestamp 排序并去重。
  - first/last 对无窗口聚合使用边界样本，窗口聚合使用窗口内边界样本。

- [x] **Step 3: 运行验证**
  - 命令：`timeout 240s go test ./internal/queryexec ./internal/engine -run 'Test.*Aggregate|Test.*Deduplicate|Test.*Tombstone|Test.*FirstLast' -timeout 240s`
  - Expected: PASS。

- [x] **实现备注:** 已在 aggregate 入口按 timestamp 稳定排序并归一化重复 timestamp，保证 first/last 与窗口聚合使用边界时间语义；新增跨 shard 窗口、重复 writeSeq、tombstone 和 first/last 快路径测试。Engine 对纯 first/last 无窗口聚合下推 `QueryBoundaryMode`，SSTable 基于 value page index 只读取首/尾匹配 page，并将未读 page 计入 stats。验证：`timeout 240s go test ./internal/queryexec ./internal/engine ./internal/sstable -run 'Test.*Aggregate|Test.*Deduplicate|Test.*Tombstone|Test.*FirstLast|TestFirstAggregateUsesBoundaryPageFastPath|TestFirstLastBoundaryQueryReadsOnlyBoundaryValuePages|TestMergeColumnDataStreamsKeepsNewestSequence|TestShardDeleteRangeReplaysAndCompactsTombstone' -timeout 240s` 通过。

## Task 6: E2E 查询语义与读放大门禁

**EARS:** tag/field/time/series/window/aggregate/limit/offset/cancel/tombstone/重复点/跨 shard 语义；读放大断言跳过 shard/part/page；错误路径可通过 API 或 iterator Err 返回。

- [x] **Step 1: 写/扩展 e2e**
  - 修改：`tests/e2e/query_aggregate_window/main.go`、`tests/e2e/query_pruning/main.go`、`tests/e2e/read_amplification/main.go`。
  - 新增或扩展：
    - `query_semantics`：覆盖 tag、field、cross shard window、duplicate writeSeq、tombstone、limit/offset。
    - `query_pruning`：断言 explain/stats 的 skipped shard/part/page。
    - `read_amplification`：断言 MaxSamples/MaxParts 错误路径。
  - 命令：`timeout 240s go test ./tests/e2e/... -run 'TestRun' -timeout 240s`
  - Expected: 先 RED 或新增场景失败，再实现后 PASS。

- [x] **Step 2: 补齐 public API**
  - root package 暴露 `QueryWithExplain`、`QueryStatsSnapshot`、`QueryExplain`、`QueryStats`。
  - e2e 只通过 public API 验证。

- [x] **Step 3: 运行 e2e**
  - 命令：`timeout 900s bash -c 'set -euo pipefail; for dir in tests/e2e/*; do [ -d "$dir" ] || continue; (cd "$dir" && go build -o testbin . && timeout 120s ./testbin; status=$?; rm -f testbin; exit $status); done'`
  - Expected: PASS，且无 `testbin` 遗留。

- [x] **实现备注:** 已扩展 `query_pruning` 通过 public `QueryWithExplain` 验证 shard explain 与 query stats，`read_amplification` 改用 public `ErrReadBudgetExceeded` 和 `QueryStatsSnapshot` 验证预算错误；root package 暴露 `QueryExplain`、`QueryStats`、`QueryResult`、`QueryWithExplain`、`QueryStatsSnapshot`。同时修复 context stream `Err()` 吞掉 inner error 的问题，确保 budget error 可通过 iterator/API 传播。验证：`timeout 900s bash -c 'set -euo pipefail; for dir in tests/e2e/*; do [ -d "$dir" ] || continue; (cd "$dir" && go build -o testbin . && timeout 120s ./testbin; status=$?; rm -f testbin; exit $status); done'` 通过。

## Task 7: Query pprof/scale 门禁

**EARS:** pprof 显示大查询不会一次性物化全部 rows。

- [x] **Step 1: 扩展 pprof 测试**
  - 文件：`tests/pprof/storage_engine/main.go`、`tests/pprof/storage_engine/main_test.go`
  - 新增参数或断言：
    - row streaming query mode 记录 query stats。
    - 输出 `query_rows_streamed`、`query_stats_samples`、`query_stats_value_pages`。
  - 命令：`timeout 240s go test ./tests/pprof/storage_engine -run 'Test.*Query.*Stats|Test.*Streaming' -timeout 240s`
  - Expected: PASS。

- [x] **Step 2: 小规模运行**
  - 命令：`timeout 300s go run ./tests/pprof/storage_engine -mode query -points 100000 -series 1000 -write-batch-size 1000`
  - Expected: JSON/log 包含 query stats，RSS 与 heap 不因 row materialization 线性暴涨。

- [x] **实现备注:** 已将 `tests/pprof/storage_engine` 的 query/read 路径改为 `QueryRowIterator` 流式读取 rows，并输出 `query_rows_streamed`、`query_stats_samples`、`query_stats_value_pages`、`query_stats_parts`、`query_stats_errors`。验证：`timeout 240s go test ./tests/pprof/storage_engine -run 'Test.*Query.*Stats|Test.*Streaming|TestRunReadMode|TestRunModes' -timeout 240s` 通过；`timeout 300s go run ./tests/pprof/storage_engine -mode query -points 100000 -series 1000 -write-batch-size 1000` 通过，输出 `query_rows_streamed=500 query_stats_samples=2000 query_stats_value_pages=20`。

## Final Verification

- [x] `timeout 300s goimports-reviser -rm-unused -format ./...`
- [x] `timeout 600s go test ./... -timeout 10m`
- [x] `timeout 600s golangci-lint run --timeout 10m`
- [x] `timeout 900s bash -c 'set -euo pipefail; for dir in tests/e2e/*; do [ -d "$dir" ] || continue; (cd "$dir" && go build -o testbin . && timeout 120s ./testbin; status=$?; rm -f testbin; exit $status); done'`
- [x] `timeout 60s git diff --check`
- [x] `timeout 60s find . -type f \( -name testbin -o -name "*.prof" -o -name "*.cover" -o -name coverage.out \) -print`
- [x] 更新本计划所有任务状态与实现备注。
- [x] 提交：`feat(storage): 完善查询执行器语义`

**Final Verification Notes:** `timeout 600s go test ./... -cover -timeout 10m` 执行通过，但仓库现有多个包覆盖率低于 90%（root 83.2%、internal/engine 86.9%、internal/sstable 88.4% 等），未作为本次专项阻塞门禁。
