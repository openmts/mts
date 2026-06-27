# MTS Module Engineering Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 按模块化与工程设计检视报告的优先级完成可验证结构优化，避免新增待处理项。

**Architecture:** 本轮采用低风险闭环策略：先补架构门禁，再抽共享契约与内部 runtime 组合层，随后拆出用户模块细粒度接口和配置映射测试。避免一次性重写 engine/query 主链路，但通过测试和接口让后续演进具备硬边界。

**Tech Stack:** Go、`go test`、项目现有 `internal` 包结构、Markdown EARS 任务跟踪。

---

## EARS 任务清单

### Task 1：架构依赖门禁

**EARS:** When 项目包依赖发生变化时，系统应通过自动化测试阻止 storage/query/model/root 出现违反分层的依赖方向。

**文件：**

- Create: `internal/archtest/architecture_test.go`

- [x] 编写架构约束测试。
- [x] 运行 `timeout 180s go test ./internal/archtest -count=1 -timeout 180s`。
- [x] 更新本计划实现备注。

**实现备注:** 已新增 `internal/archtest/architecture_test.go`，使用 `go list -json ./...` 检查 storage/query/model/root 的禁止依赖方向。验证通过：`timeout 180s go test ./internal/archtest -count=1 -timeout 180s`。

### Task 2：共享 Storage Query Contract

**EARS:** When MemTable 或 SSTable 执行扫描时，系统应使用同一个 `storagequery.Query` 契约承载 context、budget、stats、boundary、series/field 过滤和时间范围。

**文件：**

- Create: `internal/storagequery/query.go`
- Modify: `internal/memtable/memtable.go`
- Modify: `internal/sstable/types.go`
- Test: `internal/storagequery/query_test.go`

- [x] 编写 storagequery 契约测试。
- [x] 确认测试红灯。
- [x] 实现 `storagequery.Query` 并将 memtable/sstable Query 改为别名。
- [x] 运行 `timeout 180s go test ./internal/storagequery ./internal/memtable ./internal/sstable -count=1 -timeout 180s`。
- [x] 更新本计划实现备注。

**实现备注:** 已新增 `internal/storagequery.Query`，`memtable.Query` 和 `sstable.Query` 改为别名，保留现有调用兼容并消除重复契约。红灯验证为 storagequery 包无实现导致构建失败；绿灯验证通过指定测试命令。

### Task 3：Runtime 用户组合层

**EARS:** When 根包需要默认本地用户管理器时，系统应通过 `internal/runtime` 打开本地用户运行时，根包只保留公共接口适配。

**文件：**

- Create: `internal/runtime/user.go`
- Test: `internal/runtime/user_test.go`
- Modify: `local_user_adapter.go`
- Modify: `engine_types.go`

- [x] 编写 runtime 用户打开测试。
- [x] 确认测试红灯。
- [x] 实现 runtime 用户组合层。
- [x] 修改根包默认用户打开逻辑使用 runtime。
- [x] 运行 `timeout 180s go test . ./internal/runtime -run 'Test.*User|TestEngineUses.*UserManager' -count=1 -timeout 180s`。
- [x] 更新本计划实现备注。

**实现备注:** 已新增 `internal/runtime.OpenUserManager` 与 runtime 级 `UserOptions`，根包 `local_user_adapter.go` 改为通过 runtime 打开默认本地用户管理器。红灯验证为 runtime 包无实现导致构建失败；绿灯验证通过指定测试命令。

### Task 4：用户模块细粒度接口与角色策略

**EARS:** When 后续对接第三方权限系统时，系统应能按用户仓储、凭证、token、认证、授权和角色策略粒度替换组件；When 检查角色能力时，系统应明确管理员和普通用户的可管理边界。

**文件：**

- Create: `internal/user/contracts.go`
- Create: `internal/user/role_policy.go`
- Test: `internal/user/contracts_test.go`
- Test: `internal/user/role_policy_test.go`

- [x] 编写接口断言与角色策略测试。
- [x] 确认测试红灯。
- [x] 实现接口定义、组件视图和默认角色策略。
- [x] 运行 `timeout 180s go test ./internal/user -count=1 -timeout 180s`。
- [x] 更新本计划实现备注。

**实现备注:** 已新增 `UserStore`、`PermissionStore`、`CredentialStore`、`TokenStore`、`Authenticator`、`Authorizer` 和 `RolePolicy`，并提供 `DefaultRolePolicy`。红灯验证为接口和策略未定义导致构建失败；绿灯验证通过完整 `internal/user` 测试。

### Task 5：Options 映射一致性测试

**EARS:** When 公共 Options 转换为内部 model Options 时，系统应完整保留 storage 相关显式配置，并避免用户模块配置污染 storage model。

**文件：**

- Modify: `options_test.go`

- [x] 编写 Options 映射测试。
- [x] 确认测试红灯。
- [x] 补齐缺失的映射或默认值逻辑。
- [x] 运行 `timeout 180s go test . -run 'Test.*Options' -count=1 -timeout 180s`。
- [x] 更新本计划实现备注。

**实现备注:** 已新增 `convert_options_test.go`，覆盖公共 Options 到内部 model Options 的 storage 相关字段映射。红灯验证为测试辅助 logger 类型缺失导致构建失败，修正测试后映射验证通过；当前无需修改生产映射逻辑。

### Task 6：文档状态闭环

**EARS:** When 本轮结构优化完成时，系统应在评审报告和计划文档中记录处理状态、验证命令和后续演进原则，且不得保留“待处理”状态。

**文件：**

- Modify: `docs/review/code-review-2026-06-27-0913.md`
- Modify: `docs/superpowers/plans/2026-06-27-mts-module-engineering-closure.md`

- [x] 将报告发现项状态更新为已处理，并写明本轮处理方式。
- [x] 将计划所有任务打勾并补实现备注。
- [x] 运行 `timeout 120s git diff --check`。
- [x] 运行必要的 Go 测试与 lint/格式化。

**实现备注:** 已更新检视报告状态并补充闭环说明。最终验证命令记录在本轮交付总结中。
