# Single Node Commercial Gap EARS Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在单机、LocalMetadata、Builder 查询边界内，闭环 MTS 与开源时序数据库可使用能力之间的 10 个商用短板。

**Architecture:** 不实现分布式、不接入外部元数据系统、不实现 SQL/InfluxQL/PromQL parser。所有任务围绕单机存储可靠性、Builder 查询主链路、内存与 compaction 长稳、备份恢复、运维安全和质量门禁展开；每个任务必须以测试、压测或文档化运行手册作为验收证据。

**Tech Stack:** Go、LSM/WAL/MemTable/SSTable、LocalMetadataStore、querylang Builder、queryexec、queryservice、engine、storagecheck、tests/e2e、tests/fault、tests/scale、tests/pprof、golangci-lint、goimports-reviser。

---

## 固定项目边界

- 不做分布式系统实现：不实现跨节点查询、跨节点副本、故障转移、数据再平衡和分布式一致性协议。
- 不做外部元数据系统接入：只支持 `LocalMetadataStore` 本地元数据管理。
- 不做 SQL 语句支持：不实现 SQL、InfluxQL、PromQL parser；查询构造以 Builder/API 为主。

## 执行总门禁

- 每个任务完成后必须更新本文件对应状态和实现备注。
- 每个任务不得留下“后续增强”“临时实现”“弱实现”描述；若能力不属于固定边界，应明确移除或稳定拒绝，而不是半支持。
- 每轮代码改动后必须执行：
  - `timeout 300s goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .`
  - `timeout 600s go test ./... -count=1 -timeout 10m`
  - `timeout 720s golangci-lint run ./...`
  - `timeout 720s go test ./... -cover -count=1 -timeout 10m`
  - `timeout 60s git diff --check`
  - `timeout 60s find . -type f \( -name '*.test' -o -name '*.prof' -o -name '*.pprof' -o -name 'coverage.out' -o -name '*.coverprofile' \) -not -path './.git/*' -print`
- 覆盖率低于 90% 的包必须在 Task 10 闭环，不允许只报告不处理。

## Task 1: P0 长期稳定性与 Soak Gate

**状态:** 已完成。

**EARS:**
- When 运行单机长期 write/query/compact/restart/recovery 混合 workload 时，系统应输出可机器解析的 JSON 报告，包含吞吐、p50/p95/p99、RSS peak、heap peak、GC 次数、GC pause、SSTable 数、compaction backlog、read/write/space amplification 和错误数。
- When soak gate 超过配置阈值时，系统应返回非零退出码，并在报告中列出具体失败指标。
- When workload 结束时，系统应验证写入数据可查询、compaction 后数据一致、重启恢复后数据不丢失。

**实现边界:**
- 只覆盖单机本地目录，不引入集群或远端节点。
- 复用 `tests/scale/storage_soak` 和 `tests/scale/storage_10m`，必要时新增 `tests/scale/storage_soak_gate`。

**验收标准:**
- 支持 quick/standard/long 三档配置。
- quick 档纳入常规 `go test ./...` 或独立短命令；standard/long 可通过显式命令运行。
- 报告落盘到 `docs/benchmarks/`，并包含阈值通过/失败状态。

**实现备注:** 已完成 `storage_soak` 和 `storage_10m` quick 验证，并为 `storage_10m` 增加 `-verify`、`-out` 和 `write-query-compact` 标准链路模式。standard 1M write/query/compact/restart 数据校验命令已通过，并将报告落盘到 `docs/benchmarks/storage-10m-standard-2026-06-20.json`；报告包含吞吐、写入耗时、查询延迟、RSS peak、GC、SSTable 前后数量、层级分布、compaction backlog 和 amplification 指标。

**验证命令:**
- `timeout 300s go test ./tests/scale/storage_soak ./tests/scale/storage_10m -count=1 -timeout 5m`
- `timeout 1200s go run ./tests/scale/storage_10m -mode=write-query-compact -profile=standard -points=1000000 -verify=true`

## Task 2: P0 异常恢复矩阵闭环

**状态:** 已完成。

**EARS:**
- When WAL append、flush part 写入、manifest commit、compaction commit、tombstone commit、retention cleanup 任一阶段发生注入故障时，系统应返回明确错误，不产生静默数据损坏。
- When 故障后重启 Engine 时，系统应恢复到最后一个一致状态，并通过查询验证已确认写入的数据仍可读。
- When 发现孤儿 part、半写 part、CRC 错误或 manifest 不一致时，系统应通过 storagecheck 输出诊断结果，repair 行为必须可验证且不会删除有效数据。

**实现边界:**
- 只测试本地文件系统和 faultinject FS。
- 不实现跨节点恢复。

**验收标准:**
- `tests/fault/storage_fault_matrix` 覆盖上述故障点，每个 case 包含 write/flush/compact/reopen/query 断言。
- `storagecheck` 对损坏场景输出稳定错误码和路径。
- repair 测试覆盖干跑和执行两种模式。

**实现备注:** 已完成 `tests/fault/storage_fault_matrix`、`internal/engine`、`internal/storagecheck` 的 Fault/Recovery/Repair/Atomic 定向验证；故障注入、重启恢复、repair dry-run/apply、storagecheck 损坏诊断均由测试覆盖。

**验证命令:**
- `timeout 600s go test ./tests/fault/storage_fault_matrix ./internal/engine ./internal/storagecheck -run 'Test.*Fault|Test.*Recovery|Test.*Repair|Test.*Atomic' -count=1 -timeout 10m`

## Task 3: P0 Builder 查询主链路边界与表达力

**状态:** 已完成。

**EARS:**
- When 用户通过 Builder 构造 SELECT/FROM/WHERE/GROUP/ORDER/LIMIT/OFFSET/CURSOR 查询时，系统应生成稳定 `QuerySpec` 并通过 Analyzer、Planner、Optimizer、Physical、Executor 主链路执行。
- When Builder 构造 AND/OR/NOT、tag in、tag exists、field compare、time range 等表达式时，系统应区分安全下推与 post-filter，不能错误跳过数据。
- When 代码中存在 SQL/InfluxQL/PromQL parser 入口时，系统应移除该入口或将其从 public/mainline 查询能力中剥离，并更新文档说明 Builder 是唯一查询构造入口。

**实现边界:**
- 不实现 SQL parser、InfluxQL parser、PromQL parser。
- 若保留内部实验性解析函数，会违反本任务边界；应删除函数、测试和计划文档中的支持描述，或改为未导出的测试辅助且不暴露为能力。

**验收标准:**
- public `mts.NewQuery()` 与 internal `querylang.NewBuilder()` 覆盖全部查询原语。
- 删除或隔离 `ParseSQLSubset`、`ParsePromQLSubset` 相关能力声明。
- Builder 查询集成测试覆盖复杂表达式、cursor、projection、聚合、窗口、错误路径。

**实现备注:** 已删除 `internal/querylang/sql.go`、`internal/querylang/sql_apply.go`、`internal/querylang/sql_test.go`，并移除 `ErrUnsupportedLanguage`。查询入口收敛为 public `mts.NewQuery()` 与 internal `querylang.NewBuilder()`；上一轮查询计划中的 SQL/PromQL 子集描述已改写为 Builder-only 边界说明。

**验证命令:**
- `timeout 300s go test . ./internal/querylang ./internal/queryanalyzer ./internal/queryplanner ./internal/queryoptimizer ./internal/queryphysical ./internal/queryservice ./internal/engine -run 'Test.*Builder|Test.*QuerySpec|Test.*Expression|Test.*Cursor|Test.*Analyze|Test.*Plan|Test.*Optimize|Test.*Physical' -count=1 -timeout 5m`
- `timeout 60s rg -n 'ParseSQLSubset|ParsePromQLSubset|func Parse.*SQL|func Parse.*Prom' internal query_builder.go types.go`

## Task 4: P0 单机查询执行优化与读放大控制

**状态:** 已完成。

**EARS:**
- When 查询包含 shard/time/series/field/tag predicate 时，系统应在 catalog、shard、part、page 层尽早剪枝，并在 stats/explain 中报告读取与跳过数量。
- When 查询使用 `ORDER BY time DESC LIMIT N` 时，系统应优先走反向或 TopN/early-stop 路径，避免全量 row materialization 和全量排序。
- When 查询预算限制 `MaxShards/MaxParts/MaxSamples` 被触发时，系统应返回稳定 budget error，并释放所有 iterator 和 admission slot。

**实现边界:**
- 单机多 shard、多 part 并行可实现，但不做跨节点查询。
- 不允许以全量扫描后过滤作为“优化已完成”的验收。

**验收标准:**
- explain/profile 包含 shard/part/page/sample 读放大指标。
- `ORDER BY time DESC LIMIT 2000` 在中间窗口和最新窗口都能早停，并有测试断言读取 page 数小于全量。
- field page stats 对数值/bool/string 可安全跳过；无法证明安全时必须 post-filter。

**实现备注:** 现有查询执行链路已覆盖 catalog series/field 下推、shard time 剪枝、field page stats、post-filter explain、budget error、boundary first/last 快路径、cursor/order/limit 和 read amplification e2e。定向验证覆盖 `internal/engine`、`internal/sstable`、`internal/queryexec` 以及 `tests/e2e/query_pruning`、`tests/e2e/read_amplification`。

**验证命令:**
- `timeout 300s go test ./internal/engine ./internal/sstable ./internal/queryexec -run 'Test.*ReadAmplification|Test.*QueryPruning|Test.*Boundary|Test.*Limit|Test.*Order|Test.*Budget' -count=1 -timeout 5m`
- `timeout 600s go test ./tests/e2e/query_pruning ./tests/e2e/read_amplification -count=1 -timeout 10m`

## Task 5: P1 聚合、窗口与缺失值语义

**状态:** 已完成。

**EARS:**
- When Builder 查询使用 count/sum/avg/mean/min/max/first/last/rate/irate/increase/delta/difference/derivative/spread/median/mode/stddev/stdvar/top/bottom 时，系统应在 Analyzer 校验类型，并在 Executor 输出确定结果。
- When 查询使用 fill、interpolation、align、downsample、histogram、approx quantile、moving window 时，系统应要么实现完整语义并测试，要么在 Builder/Analyzer 阶段稳定拒绝，不允许执行期返回错误结果或半支持。
- When counter 类函数遇到 reset、乱序、重复 timestamp、空窗口、NaN/Inf 时，系统应按文档化语义处理。

**实现边界:**
- 以 Builder API 表达聚合/窗口，不引入 SQL/PromQL 语法。
- 不做预聚合物化层，除非完全证明与 LSM 可见性一致。

**验收标准:**
- 聚合函数支持矩阵写入文档。
- 每个函数至少覆盖正常、空输入、类型不匹配、窗口边界、重复 timestamp。
- 未实现函数必须有稳定 analyzer 错误码和测试。

**实现备注:** 已新增 `docs/query/builder-aggregate-functions.md`，明确 Builder 聚合函数支持矩阵、稳定拒绝函数和边界规则。现有 Analyzer/Executor 测试覆盖聚合、窗口、类型校验、重复 timestamp、counter reset 和未支持函数错误。

**验证命令:**
- `timeout 300s go test ./internal/queryanalyzer ./internal/queryexec ./internal/engine ./tests/e2e/query_aggregate_window -run 'Test.*Aggregate|Test.*Window|Test.*Function|Test.*Fill|Test.*Histogram|Test.*Quantile' -count=1 -timeout 5m`

## Task 6: P1 全路径内存治理与小内存保护

**状态:** 已完成。

**EARS:**
- When 配置存储层总内存阈值时，WAL、MemTable、flush、compression、compaction、query、cache、service buffer 应统一纳入预算或明确暴露为 runtime gap。
- When 内存预算不足时，系统应拒写、限流、延迟 compaction 或拒绝查询，而不是让进程 OOM。
- When 运行小内存 profile 时，RSS peak 应低于配置阈值，并在报告中说明各 source 的内存占用。

**实现边界:**
- 只控制单机进程内存；不实现 cgroup 管理器。
- 允许读取 runtime RSS/heap 指标，但不能依赖 Linux-only API 破坏跨平台构建。

**验收标准:**
- `StorageMemorySnapshot` 覆盖所有主要 source。
- result cache 和 query materialization 有上限。
- 512MiB quick gate 可通过，失败时输出明确 source 和阈值。

**实现备注:** 已完成 engine、memtable、queryservice 和 storage matrix 的 memory/budget/RSS/cache 定向验证。`storage_10m` 已补齐 `-verify` gate 开关并在报告中输出；1M standard profile 在 `-max-rss-bytes=536870912 -verify=true` 下通过，RSS peak 为 `69455872` bytes，低于 512MiB 阈值，查询窗口返回并校验 2000 行。

**验证命令:**
- `timeout 300s go test ./internal/engine ./internal/memtable ./internal/queryservice ./tests/scale/storage_matrix -run 'Test.*Memory|Test.*Budget|Test.*RSS|Test.*Cache' -count=1 -timeout 5m`
- `timeout 900s go run ./tests/scale/storage_10m -profile=standard -points=1000000 -max-rss-bytes=536870912 -verify=true`

## Task 7: P1 Compaction 长稳与放大控制

**状态:** 已完成。

**EARS:**
- When L0 SSTable 数量、层级大小、read amplification 或 tombstone 比例超过阈值时，compaction scheduler 应选择明确任务并输出原因。
- When compaction 与读写并发时，系统应保证读者视图稳定、Manifest 原子切换、失败输出清理、有效数据不丢失。
- When 长期写入产生 backlog 时，系统应通过 metrics/report 暴露 backlog、last duration、input/output bytes、input/output parts 和 amplification。

**实现边界:**
- 单机 level compaction；不做跨节点 compaction。
- 不允许把所有 SSTable 无条件合并成一个文件作为常态策略。

**验收标准:**
- compaction planner/scheduler 单元测试覆盖 size/read amplification/tombstone/backlog 策略。
- e2e 覆盖 compaction 失败、reader 并发、重启清理孤儿 part。
- scale 报告记录 compaction 前后 SSTable 数和耗时。

**实现备注:** 已完成 compaction planner/scheduler、并发 reader、孤儿 part、read amplification 和 scale 报告定向验证。1M standard profile 报告显示 compaction 前 `979` 个 L0 SSTable，压缩后 `12` 个 L1 SSTable，compaction 耗时 `4590201803` ns，backlog 为 `0`，报告包含前后层级分布、input/output parts、last task 和 amplification 指标。

**验证命令:**
- `timeout 600s go test ./internal/engine ./tests/e2e/compaction_integrity ./tests/scale/storage_10m -run 'Test.*Compaction|Test.*Backlog|Test.*Amplification|Test.*Orphan|Test.*Reader' -count=1 -timeout 10m`

## Task 8: P1 单机备份、快照、恢复与 Repair 工具

**状态:** 已完成。

**EARS:**
- When 用户请求本地快照时，系统应生成一致的可校验快照，包含 Manifest、SSTable、metadata 和必要 WAL 边界。
- When 用户从快照恢复到新目录时，系统应能打开 Engine 并查询验证数据。
- When storagecheck 发现损坏文件时，repair dry-run 应输出将要执行的动作，repair apply 应只隔离损坏对象且保留可用数据。

**实现边界:**
- 只做本地目录快照/恢复；不做对象存储、远程备份或 PITR。
- 备份文件权限遵循目录 `0700`、文件 `0600`。

**验收标准:**
- 新增 `internal/storagecheck` 或 `cmd/mts-storage` 本地 snapshot/restore/check/repair 测试。
- 快照恢复后 query 结果与源目录一致。
- repair 有审计报告和稳定错误码。

**实现备注:** 已在 `cmd/mts-storage` 增加 `snapshot`/`restore` 命令，底层复用 `internal/storagecheck.Snapshot` 与 `Restore`，通过源目录健康检查、临时目录复制、原子发布和父目录 fsync 生成本地一致快照。已修正 storagecheck 的 SSTable part 识别，避免将 `catalog/metadata.bin` 误判为 part，同时保留 metadata 已落盘但组件缺失 part 的损坏诊断；已将 WAL 深校验限定到存储 WAL 段名，避免将 `catalog.wal` 按存储 WAL 格式解析。

**验证命令:**
- `timeout 600s go test ./internal/storagecheck ./cmd/mts-storage ./internal/engine -run 'Test.*Snapshot|Test.*Restore|Test.*Repair|Test.*Check' -count=1 -timeout 10m`

## Task 9: P1 生产运维、安全边界与 Runbook

**状态:** 已完成。

**EARS:**
- When 服务暴露 metrics、health、ready、pprof、admin compact、query audit 时，系统应有安全默认配置，pprof/admin 默认关闭或要求显式启用。
- When 管理端点被调用时，系统应执行权限检查、记录审计并限制高危操作并发。
- When 运维人员排查性能、磁盘、内存、compaction、恢复问题时，项目应提供可执行 Runbook 和告警指标说明。

**实现边界:**
- 单机 HTTP service 安全与运维，不做多租户 IAM 平台。
- 不引入外部认证系统；可支持静态 token 或本地配置。

**验收标准:**
- pprof/admin 端点默认安全，测试覆盖未授权访问。
- docs 增加 metrics、alert、backup/restore、compaction、memory、query slow path Runbook。
- 慢查询日志或 query profile 可配置开启。

**实现备注:** 已将 admin compact 改为显式启用：`EnableAdmin=true` 时才注册 `/admin/compact`，且必须配置 `AdminToken` 并通过 `Authorization: Bearer <token>` 或 `X-MTS-Admin-Token` 认证；pprof 继续保持 `EnablePprof=false` 默认关闭。测试覆盖默认 admin 404、未授权 401、授权 compact、timeout、审计和 pprof 默认关闭。已新增 `docs/storage/operations-runbook.md`，覆盖 metrics、alert、backup/restore、compaction、memory 和 slow query 排查。

**验证命令:**
- `timeout 300s go test ./internal/service ./internal/queryservice ./internal/observability -run 'Test.*Admin|Test.*Auth|Test.*Audit|Test.*Metrics|Test.*Health|Test.*Pprof' -count=1 -timeout 5m`
- `timeout 60s rg -n 'pprof|admin|Runbook|alert|backup|restore|slow query|compaction backlog' docs internal/service internal/queryservice`

## Task 10: P0 覆盖率、CI Gate 与回归基线

**状态:** 已完成。

**EARS:**
- When 执行全量测试覆盖率时，每个核心包覆盖率应达到 90% 或以上；低于阈值时应输出失败并列出包名。
- When 执行 CI gate 时，系统应运行 goimports-reviser、go test、golangci-lint、coverage、scale quick、fault quick 和临时产物扫描。
- When benchmark 或 scale quick 相比基线退化超过阈值时，系统应输出回归报告，不允许静默通过。

**实现边界:**
- 只建设本地/CI gate，不要求发布流水线、Docker、GoReleaser 或制品签名。
- coverage gate 以核心包为目标；tests/e2e、tests/scale 包可单独设门槛，避免测试 harness 覆盖率污染核心判断。

**验收标准:**
- 新增可运行 gate 脚本或 Go test helper，失败时返回非零退出码。
- 核心包覆盖率补到 >=90%。
- 文档记录 gate 命令和性能基线阈值。

**实现备注:** 已补齐核心包覆盖率缺口，`internal/engine` 精确覆盖率为 `90.0882%`，全量 coverage 中核心包均达到 90% 或以上。`scripts/ci_gate.sh` 已作为统一 gate 执行 format、全量测试、lint、核心覆盖率、fault/scale/pprof smoke 和临时产物扫描，命令返回 0；`golangci-lint run ./...` 返回 `0 issues`。同时修正测试侧 nil context 与 sync.Pool 非 pointer-like 测试写法，避免 lint gate 报警且保留原分支覆盖。

**验证命令:**
- `timeout 180s go test ./internal/engine -coverprofile=/tmp/engine.out -count=1 -timeout 3m`
- `timeout 300s goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .`
- `timeout 600s go test ./... -count=1 -timeout 10m`
- `timeout 720s golangci-lint run ./...`
- `timeout 720s go test ./... -cover -count=1 -timeout 10m`
- `timeout 900s bash scripts/ci_gate.sh`

**验证命令:**
- `timeout 720s go test ./... -cover -count=1 -timeout 10m`
- `timeout 720s golangci-lint run ./...`
- `timeout 900s go test ./tests/fault/storage_fault_matrix ./tests/scale/storage_matrix ./tests/pprof/storage_engine -count=1 -timeout 15m`
- `timeout 60s find . -type f \( -name '*.test' -o -name '*.prof' -o -name '*.pprof' -o -name 'coverage.out' -o -name '*.coverprofile' \) -not -path './.git/*' -print`

## 下一轮执行顺序

1. 先执行 Task 3：清理查询边界，确保后续不再围绕 SQL/PromQL 做无效工作。
2. 执行 Task 4、Task 5：闭环 Builder 查询能力和单机查询执行性能。
3. 执行 Task 6、Task 7：闭环内存与 compaction 两条最容易导致商用事故的链路。
4. 执行 Task 2、Task 8：闭环异常恢复、快照恢复和 repair。
5. 执行 Task 1、Task 9、Task 10：补齐长期 gate、运维安全、覆盖率和回归基线。

## 自检

- Scope coverage：10 个短板分别映射 Task 1 到 Task 10。
- Boundary check：未包含分布式、外部元数据、SQL/InfluxQL/PromQL parser 实现。
- Placeholder scan：本文不使用 TBD/TODO；所有未开始项用明确 `状态: 待执行` 表达，供下一轮执行时逐项更新。
