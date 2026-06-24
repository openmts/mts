# Time Precision API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 mts public API 增加秒、毫秒、微秒、纳秒时间精度声明，同时保持内部统一纳秒存储。

**Architecture:** public package 增加 `TimePrecision` 与转换函数；写入转换层把 `Point` 和 `TypedBatch` 时间归一化为纳秒；查询转换层把 public 查询范围归一化为纳秒，并在结果转换层按 `Query.Precision` 返回时间戳。内部 model、WAL、SSTable、downsample 和 stats 时间单位不变。

**Tech Stack:** Go、public `mts` package、`internal/model` 转换层、`tests/e2e/public_api_workflow`、`go test`、`goimports-reviser`、`golangci-lint`、Makefile。

**预计耗时与硬超时:** 规格和计划 10 分钟；实现 45-70 分钟；定向 Go 测试 `-timeout 180s`；全量 `make ci` 10-12 分钟；e2e public API `-timeout 300s`；lint `--timeout 10m`。

---

## 文件结构

- Create: `precision.go`：定义 `TimePrecision`、合法常量、转换到纳秒和从纳秒转换的溢出安全函数。
- Modify: `errors.go`：新增 `ErrInvalidPrecision`。
- Modify: `values.go`：`Point` 和 `TypedBatch` 增加 `Precision` 字段并更新注释。
- Modify: `query_types.go`：`Query` 增加 `Precision` 字段并更新注释。
- Modify: `query_builder.go`：新增 `Precision` 方法，`TimeRangeTime` 设置纳秒 precision。
- Modify: `convert_values.go`：写入转换返回 error，转换 timestamp。
- Modify: `convert_query.go`：查询转换返回 error，转换 start/end、predicate 和 expr。
- Modify: `convert_results.go`：结果转换接收 precision 并转换数据时间戳。
- Modify: `engine.go`：写入、查询和 iterator 接入转换错误与返回 precision。
- Modify: `query_builder_test.go`：覆盖 Builder precision。
- Modify: `engine_test.go` 或新增 `precision_test.go`：覆盖 public API 行为。
- Modify: `tests/e2e/public_api_workflow/*`：覆盖 e2e precision 场景。
- Modify: `README.md`、`doc.go`、`llms.txt`：补充公开 precision 用法。

## Task 1: Public Precision 类型与 Builder 合约

**状态:** 已完成。

**实现备注:** 已新增 `TimePrecision`、`ErrInvalidPrecision`、Builder `Precision` 方法和 `TimeRangeTime` 纳秒语义；定向测试先失败后通过。

**EARS:** When 调用方声明 precision 或保持零值时，系统应能稳定表达和校验时间单位。

- [x] Step 1: 写失败测试，覆盖 `QueryBuilder.Precision(PrecisionMillisecond)` 和 `TimeRangeTime` 固定纳秒。
  - Run: `timeout 180s go test . -run 'TestPublicQueryBuilder.*Precision' -count=1 -timeout 180s`
  - Expected: FAIL，原因是 `Precision` API 或字段不存在。
- [x] Step 2: 新增 `precision.go`、扩展 DTO 和 Builder 最小实现。
- [x] Step 3: 运行同一测试通过，并记录实现备注。

## Task 2: 写入时间精度转换

**状态:** 已完成。

**实现备注:** `Point` 和 `TypedBatch` 写入转换会先完整校验 precision 和溢出，再进入内部 engine；默认零值保持纳秒。

**EARS:** When 写入 point 或 typed batch 时，系统应按 precision 转换为内部纳秒，并拒绝非法或溢出 precision。

- [x] Step 1: 写失败测试，覆盖 Point 毫秒写入、TypedBatch 秒写入、默认纳秒兼容、非法 precision、溢出不写入。
  - Run: `timeout 180s go test . -run 'TestPrecision.*Write|TestPrecision.*Rejects' -count=1 -timeout 180s`
  - Expected: FAIL，原因是写入转换尚未实现。
- [x] Step 2: 修改 `convert_values.go` 和 `engine.go`，让转换函数返回 error 并在进入内部 engine 前完成全部 timestamp 转换。
- [x] Step 3: 运行同一测试通过，并记录实现备注。

## Task 3: 查询输入与结果时间精度转换

**状态:** 已完成。

**实现备注:** `toModelQuery` 现在返回内部查询、结果转换因子和错误；行、列、iterator、explain 结果均按查询 precision 转换数据时间戳，stats 纳秒字段不转换。

**EARS:** When 查询声明 precision 时，系统应按该 precision 解释查询范围并按该 precision 返回数据时间戳。

- [x] Step 1: 写失败测试，覆盖 `QueryRows`、`QueryColumns`、`QueryWithExplain`、`QueryRowIterator`、`QueryColumnIterator` 的返回 timestamp 精度。
  - Run: `timeout 180s go test . -run 'TestPrecision.*Query|TestPrecision.*Iterator|TestPrecision.*Explain' -count=1 -timeout 180s`
  - Expected: FAIL，原因是查询和结果转换尚未实现。
- [x] Step 2: 修改 `convert_query.go`、`convert_results.go` 和 `engine.go`，让查询转换返回 error，并在结果转换时保存 query precision。
- [x] Step 3: 运行同一测试通过，并记录实现备注。

## Task 4: Public E2E 与文档

**状态:** 已完成。

**实现备注:** `public_api_workflow` 已覆盖 TypedBatch 秒写入、Point 毫秒写入和毫秒查询返回；README、doc.go、llms.txt 已补充 precision 用法。

**EARS:** When 外部用户按 public API 使用秒或毫秒时间戳时，系统应在端到端工作流中正确写入、查询和返回指定精度。

- [x] Step 1: 扩展 `tests/e2e/public_api_workflow`，加入秒或毫秒写入和毫秒查询返回断言。
- [x] Step 2: 更新 README、doc.go、llms.txt 中公开时间精度说明。
- [x] Step 3: 运行 `timeout 300s go test ./tests/e2e/public_api_workflow -count=1 -timeout 300s` 通过，并记录实现备注。

## Task 5: 质量门禁与收尾

**状态:** 已完成。

**实现备注:** `make fmt`、定向测试、`make ci`、`make e2e-public-api`、`git diff --check` 和临时产物扫描均已执行；产物扫描无残留输出。

**EARS:** When 本功能完成时，系统应通过格式化、测试、lint、e2e 和产物清理检查。

- [x] Step 1: 运行 `timeout 300s make fmt`。
- [x] Step 2: 运行 `timeout 300s go test . -run 'TestPrecision|TestPublicQueryBuilder.*Precision' -count=1 -timeout 180s`。
- [x] Step 3: 运行 `timeout 900s make ci`。
- [x] Step 4: 运行 `timeout 300s make e2e-public-api`。
- [x] Step 5: 运行 `git diff --check`。
- [x] Step 6: 运行临时产物扫描并清理本次产生的构建或覆盖率产物。
- [x] Step 7: 更新本计划每个 Task 状态和实现备注。
