# MTS Runtime Facade Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将默认用户模块适配和运行时组合职责从根包下沉到 `internal/runtime`，让根包只保留 public DTO、接口和 facade 方法。

**Architecture:** `internal/runtime.Engine` 组合 storage engine 和 runtime 用户管理器；根包 `Engine` 只持有 runtime facade。默认本地用户 manager 在 runtime 内打开，外部注入的 public `UserManager` 通过根包薄 adapter 转成 runtime 接口。

**Tech Stack:** Go、手动依赖注入、`internal/runtime`、`internal/user`、`internal/engine`、架构约束测试。

---

## EARS 任务清单

### Task 1：定义 runtime 用户合约和默认本地适配

**EARS:** When runtime 需要管理用户时，系统应使用 `internal/runtime.UserManager`，并在 runtime 内装配默认本地实现。

- [x] 新增 `internal/runtime/user_manager.go`，定义 runtime 用户 DTO、`UserManager` 接口和默认本地 adapter。
- [x] 修改 `internal/runtime/user.go`，保留 `OpenUserManager` 并新增 `openRuntimeUserManager`。
- [x] 新增 `internal/runtime/user_manager_test.go`，覆盖默认本地 manager 的 CRUD、权限、认证和关闭。
- [x] 运行 `timeout 180s go test ./internal/runtime -run 'TestRuntime.*User|TestOpenUserManager' -count=1 -timeout 180s`。

实现备注：默认本地用户适配已迁移到 runtime，根包不再承载本地实现 adapter。

### Task 2：定义 runtime Engine 组合层

**EARS:** When 根包打开 MTS 时，系统应通过 runtime facade 组合 storage engine 与用户 manager。

- [x] 新增 `internal/runtime/engine.go`，定义 `Engine`、`Options` 和 `OpenEngine`。
- [x] `OpenEngine` 先打开 storage engine，再打开默认或外部用户 manager；用户 manager 打开失败时关闭 storage engine。
- [x] `Engine.Close` 同时关闭 storage engine 和 runtime 创建的用户 manager；外部注入 manager 不由 runtime 关闭。
- [x] 新增 `internal/runtime/engine_test.go`，覆盖默认用户持久化和外部用户 manager 注入。
- [x] 运行 `timeout 180s go test ./internal/runtime -run 'TestRuntimeEngine' -count=1 -timeout 180s`。

实现备注：runtime facade 已组合 storage engine 和用户 manager，根包不再分别持有两个模块。

### Task 3：调整根包 Engine facade

**EARS:** When 外部用户调用根包 `Engine` 方法时，系统应通过 runtime facade 转发，并保持 public API 不变。

- [x] 修改 `engine_types.go`，将 `Engine` 字段替换为 `runtime *runtime.Engine`。
- [x] 新增 `runtime_user_adapter.go`，只负责把外部注入的 public `UserManager` 转成 runtime `UserManager`。
- [x] 删除根包 `local_user_adapter.go` 和 `local_user_adapter_internal_test.go`。
- [x] 修改 `engine.go`、`downsample.go`，通过 `e.runtime.Storage()` 和 `e.runtime.Users()` 转发。
- [x] 运行 `timeout 180s go test . -run 'TestEngine.*UserManager|TestEngineUser|TestLocalUser|TestDefaultOptionsOpenWriteAndQuery' -count=1 -timeout 180s`。

实现备注：public API 签名保持不变；根包只做 DTO 转换和 facade 转发。

### Task 4：补架构门禁和检视报告状态

**EARS:** When 架构测试运行时，系统应阻止根包重新导入 `internal/user`，并阻止 storage engine 依赖 runtime/user。

- [x] 找到现有 `internal/archtest` 规则文件并新增依赖约束。
- [x] 新增或更新测试，断言根包不导入 `internal/user`。
- [x] 更新 `docs/review/code-review-2026-06-27-1719.md`，标记长期方案已处理。
- [x] 运行 `timeout 180s go test ./internal/archtest -count=1 -timeout 180s`。

实现备注：新增架构规则阻止根包导入 `internal/user`，并阻止 storage engine 依赖 runtime/user。

### Task 5：最终质量门禁

**EARS:** When runtime facade 重构完成时，系统应通过格式化、lint、全量测试和 diff 检查。

- [x] 运行 `timeout 300s /root/go/bin/goimports-reviser -rm-unused -format ./...`。
- [x] 运行 `timeout 600s /root/go/bin/golangci-lint run ./...`。
- [x] 运行 `timeout 600s go test ./... -count=1 -timeout 10m`。
- [x] 运行 `timeout 120s git diff --check`。

实现备注：格式化、lint、全量测试和 diff 空白检查均已通过。
