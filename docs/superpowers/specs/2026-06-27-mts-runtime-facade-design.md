# MTS Runtime Facade 长期模块边界设计

## 背景

当前根包 `mts` 是外部用户直接导入的 public API facade。根包应只暴露 DTO、必要接口和对外方法，不应包含默认本地用户模块的实现细节。现状中 `local_user_adapter.go` 虽然未导出类型，但仍让根包直接依赖 `internal/user` 的细粒度接口，并负责默认本地用户管理器的适配与生命周期。

本设计承接 `docs/review/code-review-2026-06-27-1719.md` 的长期方案：用户模块不下沉到 `internal/engine`，而是和 storage engine 平行，由内部 runtime 组合层完成装配。

## 目标

- When 外部用户导入根包时，根包应只暴露 public DTO、必要接口和 facade 方法，不包含默认本地实现适配代码。
- When `Open` 打开 Engine 时，系统应通过 `internal/runtime` 创建默认本地用户模块。
- When 后续需要接入第三方用户系统时，系统应在 MTS 仓库内新增内部 user provider，并通过 runtime 组合层选择 provider。
- When 根包 `Engine` 需要执行存储或用户操作时，系统应通过内部 runtime facade 转发，而不是在根包分别持有 storage engine 和 user manager。
- When 后续新增用户、认证、授权或服务治理能力时，系统应优先放入 runtime/security 组合层，不下沉到 `internal/engine`。
- When 架构约束测试运行时，系统应阻止根包直接导入 `internal/user`，防止默认用户实现细节再次泄漏到根包。

## 非目标

- 不改变 public API 方法签名。
- 不把用户管理下沉到 `internal/engine`。
- 不引入新的 DI 框架；本轮继续使用手动构造函数注入。
- 不重写 `cmd/mts-server` 的 HTTP/gRPC API。
- 不改变用户模块持久化格式和默认管理员行为。

## 设计方案

### Runtime 组合层

在 `internal/runtime` 新增组合层对象：

- `Engine`：内部 runtime facade，持有 `*internal/engine.Engine` 和 `UserManager`。
- `Options`：包含 storage engine options 和默认用户选项。
- `UserManager`：runtime 层用户管理接口，使用 `internal/user` 类型，作为内部安全边界。
- `OpenEngine(ctx, opts)`：打开 storage engine，装配默认本地或外部用户 manager。

根包 `mts.Engine` 只持有 `*runtime.Engine`，不再直接持有 `*internal/engine.Engine`、`UserManager` 和 `closeUserManager`。

### 用户适配边界

由于 Go 会产生 import cycle，`internal/runtime` 不应导入根包 `mts`。因此 public DTO 转换只留在根包 facade，默认用户实现和 provider 接入都留在 `internal/runtime`：

- 根包保留 public 用户 DTO 与 Engine 方法，不暴露 `UserManager` 接口或 `Options.UserManager` 注入字段。
- 默认本地用户实现适配迁移到 `internal/runtime`，使用 `internal/user` 类型和接口，不出现在根包。
- 第三方用户系统接入时，在 MTS 仓库内新增 provider，并由 runtime 根据配置选择，不要求外部用户在业务进程里注入 interface 实现。

根包不再直接导入 `internal/user`，也删除 `local_user_adapter.go`。

### 存储能力转发

根包现有存储方法继续保持 public API 行为，内部改为通过 `e.runtime.Storage()` 或 runtime facade 方法访问 storage engine。

为了减少一次性改动面，本轮允许根包通过 `runtime.Engine.Storage()` 获取 storage engine facade，并继续复用现有 public DTO 转换逻辑。后续可逐步把更多组合行为收敛到 runtime 方法。

### 架构门禁

扩展 `internal/archtest`：

- 根包不得导入 `github.com/openmts/mts/internal/user`。
- `internal/engine` 不得导入 `internal/user` 和 `internal/runtime`。
- `internal/runtime` 可以导入 `internal/engine` 和 `internal/user`，作为组合层。

## 验收

- `local_user_adapter.go` 从根包移除。
- 根包没有任何 `internal/user` import。
- `Engine` 根包结构体只持有 runtime facade。
- 默认用户管理、用户 CRUD、认证授权、数据库权限测试保持通过。
- 架构测试覆盖根包禁止导入 `internal/user`。
- `goimports-reviser`、`golangci-lint`、`go test ./...`、`git diff --check` 全部通过。
