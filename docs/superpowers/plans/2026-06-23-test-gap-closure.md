# Test Gap Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补齐 MTS 当前端测与性能用例中面向外部用户的关键缺口。

**Architecture:** 新增一个只依赖根包 `github.com/openmts/mts` 的 e2e 包，验证 typed batch、Builder、Row/Column iterator、元数据列表和跨重启读取。扩展 `internal/bench`，补充查询迭代器读路径 benchmark，并接入现有 benchmark gate 与 Makefile。

**Tech Stack:** Go `testing`、Go benchmark、现有 `make` 目标、现有 `scripts/storage_benchmark_gate.sh`。

---

### Task 1: 测试检视报告

**Files:**
- Create: `docs/review/code-review-2026-06-23-0040.md`

- [x] **Step 1: 梳理覆盖矩阵**

实现备注：已梳理根包公开 API、CLI、e2e、fault、scale、pprof、benchmark 覆盖现状。

- [x] **Step 2: 记录缺口与验收**

实现备注：已记录公开 typed API 跨重启端测缺口和查询迭代器性能基准缺口。

### Task 2: 公开 API 端测

**Files:**
- Create: `tests/e2e/public_api_workflow/main.go`
- Create: `tests/e2e/public_api_workflow/assertions.go`
- Create: `tests/e2e/public_api_workflow/main_test.go`
- Modify: `tests/e2e/README.md`
- Modify: `tests/README.md`
- Modify: `Makefile`

- [x] **Step 1: 新增 public API workflow e2e**

验收：`go test ./tests/e2e/public_api_workflow -count=1 -timeout 10m` 通过。

实现备注：新增 `public_api_workflow`，覆盖 `DefaultOptions`、`WriteTypedBatch`、Builder、Row/Column iterator、元数据列表、跨重启读取、`QueryWithExplain` 和 `HealthSnapshot`。

- [x] **Step 2: 接入 Makefile 与测试文档**

验收：`make help` 能显示 `e2e-public-api`，且 `make e2e-public-api` 通过。

实现备注：已新增 `e2e-public-api` 和 `public-api-workflow` 目标，并更新 README、`tests/README.md`、`tests/e2e/README.md`。

### Task 3: 查询迭代器性能基准

**Files:**
- Modify: `internal/bench/storage_bench_test.go`
- Modify: `scripts/storage_benchmark_gate.sh`
- Modify: `Makefile`

- [x] **Step 1: 新增 Row/Column iterator benchmark**

验收：`go test ./internal/bench -run '^$' -bench 'BenchmarkEngineQuery(Row|Column)Iterator/points=1000$' -benchmem -count=1 -timeout 5m` 通过。

实现备注：已新增 `BenchmarkEngineQueryRowIterator` 和 `BenchmarkEngineQueryColumnIterator`，benchmark 计时阶段遍历完整结果并校验返回数量。

- [x] **Step 2: 接入 benchmark gate**

验收：`make bench-query` 与 `make bench` 均能执行查询迭代器性能用例。

实现备注：已扩展 `scripts/storage_benchmark_gate.sh` 的 benchmark 正则，并新增 `make bench-query`。

### Task 4: 验证与收尾

**Files:**
- Modify: `docs/review/code-review-2026-06-23-0040.md`

- [x] **Step 1: 运行定向验证**

验收：新增 e2e、benchmark、Makefile 目标通过。

实现备注：`go test ./tests/e2e/public_api_workflow -count=1 -timeout 10m -v`、`make help`、`make e2e-public-api`、`make bench-query`、`make bench` 均通过。

- [x] **Step 2: 运行完整门禁**

验收：`make ci`、`git diff --check`、临时产物扫描通过。

实现备注：`make fmt`、`make ci`、`git diff --check`、临时产物扫描均通过。

- [x] **Step 3: 更新检视报告**

验收：检视报告记录实际验证命令与结果。

实现备注：已更新 `docs/review/code-review-2026-06-23-0040.md` 的验证记录。
