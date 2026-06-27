# MTS 模块化与工程设计闭环优化设计

## 背景

本设计承接 `docs/review/code-review-2026-06-27-0913.md`。目标是在不改变当前单机边界、不引入 SQL/PromQL/InfluxQL parser、不触碰 `cmd/` 目录实现的前提下，对报告中 P0/P1/P2 问题做可验证的结构优化。

设计依据包括 Go 官方模块布局建议：Go 项目应保持简单，`internal` 用于隐藏不希望被外部直接依赖的实现细节；本项目当前以根包作为外部 API，以 `internal/` 承载实现，符合该方向。

## 目标

- When 项目新增或修改模块依赖时，系统应通过架构约束测试阻止明显的反向依赖和跨层穿透。
- When MemTable 和 SSTable 执行扫描时，系统应使用同一个 storage scan contract，避免重复查询结构漂移。
- When 根包需要打开默认用户模块时，系统应通过内部 runtime 组合层创建本地用户运行时，根包只保留公共接口适配。
- When 用户模块需要对接第三方权限系统时，系统应提供用户仓储、凭证、token、授权和角色策略等细粒度接口，而不是只能整体替换本地 Manager。
- When 嵌入式 Options 与内部 model Options 转换时，系统应有测试保证核心默认值和显式配置不漂移。
- When 后续继续拆分 engine/query/model 时，工程师应能从文档和任务状态看到已完成边界与剩余演进方向。

## 非目标

- 不实施分布式存储、分布式查询、副本、故障转移、一致性协议。
- 不新增 SQL、InfluxQL、PromQL parser。
- 不修改 `cmd/` 目录实现。
- 不一次性重写查询执行器或大规模搬迁 `internal/engine`，避免破坏当前商业路径。

## 设计方案

### 架构约束

新增 `internal/archtest` 包，用 Go 测试执行 `go list -json ./...` 并检查依赖方向。规则聚焦当前报告中的风险：

- storage 基础包不得依赖 `internal/engine`。
- query 基础包不得依赖 `internal/engine`。
- `internal/model` 不得依赖其他 MTS 内部业务包。
- 根包不得直接导入 `cmd/`。

这些规则不替代人工设计评审，但能阻止最危险的方向性回归。

### Runtime 用户组合层

新增 `internal/runtime`，承载默认本地用户模块的打开逻辑和路径策略。根包仍暴露 `UserManager` 接口，但默认实现的创建由 runtime 完成，根包只负责把内部 Manager 适配成公共接口。

### Storage Query Contract

新增 `internal/storagequery.Query`，作为 MemTable 与 SSTable 的共享扫描契约。`memtable.Query` 和 `sstable.Query` 先以类型别名方式兼容迁移，降低修改面；后续可逐步让 engine 直接依赖 `storagequery.Query`。

### User 组件接口

在 `internal/user` 内补齐细粒度接口：

- `UserStore`
- `PermissionStore`
- `CredentialStore`
- `TokenStore`
- `Authenticator`
- `Authorizer`
- `RolePolicy`

本地 `Manager` 继续作为默认组合门面，并通过编译期断言确保实现这些接口。新增默认角色策略，明确管理员、普通用户的管理边界。

### 配置一致性

新增 Options 映射测试，覆盖默认值、用户配置不进入 storage model、WAL/compaction/compression/storage memory 显式字段完整转换。

## 验收

- 架构约束测试存在且可执行。
- MemTable 和 SSTable 查询结构统一来源于 `internal/storagequery.Query`。
- 根包默认本地用户模块打开逻辑下沉到 `internal/runtime`。
- 用户模块有细粒度接口和角色策略测试。
- Options 转换有回归测试覆盖。
- 所有新增任务在计划文档中标记完成并写明验证命令。
