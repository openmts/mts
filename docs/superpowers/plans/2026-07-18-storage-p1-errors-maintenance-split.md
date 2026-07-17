# Storage P1-07/08 + P1-06 Touch Split Closure Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans

**Goal:** 闭环 P1-07 公共错误契约、P1-08 后台维护可控可观测，并对本轮相关 engine 大文件做最小职责拆分，无遗留弱实现。

**Architecture:**
- public 稳定 sentinel：`ErrResourceExhausted`、`ErrEngineBusy`、`ErrStorageMemoryLimitExceeded` 等，`publicError` 全量映射可 `errors.Is`。
- server `classifyAPIError` 优先 `errors.Is`，去掉字符串猜测依赖关键路径。
- 维护：统一 `MaintenanceStats`（compact/downsample 并发、跳过、失败）；downsample 全局 inflight 上限；后台 compact 绑定可取消 context。
- 拆分：`engine.go` 抽出 write memory、shard routing、health。

## Tasks
- [x] P1-07 errors + server classify + tests
- [x] P1-08 maintenance stats/limits + tests + metrics
- [x] P1-06 engine.go split
- [x] full make test/e2e/lint/bench
- [x] update review


## 实现备注

### P1-07（已完成）
- `errors.go`：`ErrResourceExhausted` / `ErrEngineBusy` / `ErrStorageMemoryLimitExceeded`，`publicError` 用 `errors.Join` 映射 cardinality/memory/budget/busy。
- `cmd/mts-server/http.go`：`classifyAPIError` 优先 `errors.Is` 资源耗尽类。
- 测试：`public_error_map_test.go`、`error_contract_test.go`、`cmd/mts-server/classify_error_test.go`。

### P1-08（已完成）
- `MaintenanceStats` + `MaintenanceStatsSnapshot`（internal + public）。
- downsample 全局并发：`MaxConcurrentDownsample`（默认 2），skip 计数。
- 后台 compact 使用 `compactCtx`，Close 时取消。
- metrics：`mts_maintenance_*` gauge/counter。
- 测试：`maintenance_stats_test.go`（internal + public）。

### P1-06（本轮最小拆分完成）
- 从 `internal/engine/engine.go` 抽出：
  - `write_memory.go`（估算/限额/flush 触发）
  - `health.go`
  - `shard_routing.go`
- `engine.go` 从 ~1063 行降至 **262 行**（已满足 300 上限）；另抽出 `maintenance.go`/`metrics_snapshot.go`；WAL/SSTable/MemTable 等其它超大文件留待后续。

### 验证（2026-07-18）
- `make test`：通过
- `make e2e`：通过
- `golangci-lint run ./internal/engine/... ./internal/catalog/... ./cmd/mts-server/... .`：0 issues
- bench median vs `/tmp/mts-bench-baseline.txt`：
  - WriteBatch +0.68%
  - WriteWideBatch -0.30%
  - QueryRow -1.23%
  - QueryColumn +0.56%
  - 最差劣化 <1%，无 >10% 回归
