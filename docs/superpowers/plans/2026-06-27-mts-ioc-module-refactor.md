# MTS IoC Module Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 深化模块 IoC 设计，让 engine、queryservice、user 模块具备更明确的可注入边界。

**Architecture:** Engine 使用 `OpenWithDeps` 注入 metadata/shard 依赖；queryservice 用测试锁定 layered 主链路；user Manager 下沉本地状态存储并暴露组件视图。

**Tech Stack:** Go、TDD、现有 internal 模块。

---

## EARS 任务清单

### Task 1：Engine 顶层 Deps 注入

**EARS:** When engine 打开实例时，系统应通过 Deps 注入 metadata store 和 shard deps；When 未传 Deps 时，系统应使用默认本地依赖。

- [x] 写 `OpenWithDeps` 自定义 metadata opener 测试。
- [x] 确认红灯。
- [x] 实现 `Deps`、`OpenWithDeps`、默认依赖归一化。
- [x] 运行 `timeout 240s go test ./internal/engine -run 'TestOpenWithDeps|TestEngineLifecycle' -count=1 -timeout 240s`。

**实现备注:** 已新增 `engine.Deps` 和 `OpenWithDeps`，`Open` 委托默认 deps；Engine 保存 shard deps 并传入新建/加载 shard。红灯为 `OpenWithDeps`/`Deps` 未定义；绿灯验证通过。

### Task 2：QueryService layered 主链路防回退

**EARS:** When queryservice 使用 LayeredExecutor 查询时，系统应先构建 logical/optimized/physical 信息，再调用 QuerySpec 入口执行，且结果中包含 physical operators 和 pushdowns。

- [x] 补充 layered executor 主链路测试。
- [x] 确认测试红灯或补强现有断言。
- [x] 调整 LayeredExecutor 让策略/estimate 与 QuerySpec 请求更清晰。
- [x] 运行 `timeout 180s go test ./internal/queryservice -run 'TestLayeredExecutor' -count=1 -timeout 180s`。

**实现备注:** 已强化 `TestLayeredExecutorRunsAnalyzerAndQuerySpecRows`，断言 logical root、physical operators、pushdowns、limit pushdown 和 limit physical operator，防止回退到纯兼容执行路径。现有实现满足增强断言。

### Task 3：User Manager 本地状态 Store

**EARS:** When 本地用户模块读写 users/grants/password/tokens 时，系统应通过 localStateStore 完成加载、克隆和原子替换，Manager 只作为组合门面。

- [x] 写 localStateStore 加载和替换测试。
- [x] 确认红灯。
- [x] 实现 localStateStore 并迁移 Manager load/replace/clonedState。
- [x] 运行 `timeout 180s go test ./internal/user -run 'TestLocalStateStore|TestManager' -count=1 -timeout 180s`。

**实现备注:** 已新增 `localStateStore`，负责用户模块本地状态 load/replace；Manager 改为持有 store 并委托加载与原子替换。红灯为 store 未定义；绿灯验证通过。

### Task 4：User Manager 组件视图

**EARS:** When 第三方系统需要复用本地用户模块能力时，系统应能从 Manager 获取用户、权限、凭证、token 和角色策略组件视图。

- [x] 写组件视图接口测试。
- [x] 确认红灯。
- [x] 实现 `Users`、`Permissions`、`Credentials`、`Tokens`、`Roles` 方法。
- [x] 运行 `timeout 180s go test ./internal/user -run 'TestManager.*Components|TestDefaultRolePolicy' -count=1 -timeout 180s`。

**实现备注:** 已新增 Manager 组件视图方法，返回用户、权限、凭证、token 和角色策略组件接口。红灯为方法未定义；绿灯验证通过。

### Task 5：验证与文档闭环

**EARS:** When IoC 模块化深化完成时，系统应更新计划状态并通过格式化、lint、全量测试。

- [x] 更新计划任务状态和实现备注。
- [x] 运行 `timeout 300s /root/go/bin/goimports-reviser -rm-unused -format ./...`。
- [x] 运行 `timeout 600s /root/go/bin/golangci-lint run ./...`。
- [x] 运行 `timeout 600s go test ./... -count=1 -timeout 10m`。
- [x] 运行 `timeout 120s git diff --check`。

**实现备注:** 最终验证命令在本轮交付总结中记录。

## 追加接口化评估与处理结果

### 已处理：去除 engine 到 SSTable 具体 Part 的类型断言

**EARS:** When engine 需要创建 series batch reader 时，系统应通过 `partReader` 能力接口获取 batch reader，而不是在默认 part manager 中断言 `*sstable.Part`。

**处理结果:** 已将 `NewSeriesBatchReader(query)` 提升到 `partReader` 接口，`sstable.Part` 直接实现该能力，`defaultPartManager` 不再依赖 `reader.(*sstable.Part)`。

### 已处理：User Manager 本地状态所有权下沉

**EARS:** When Manager 管理本地用户状态时，系统应由 `localStateStore` 持有 users、grants、passwords、tokens，Manager 只通过 store 协调状态加载、克隆和替换。

**处理结果:** 已将 `users`、`grants`、`password`、`tokens` 从 Manager 字段移入 `localStateStore`，Manager 不再直接拥有本地状态 map。

### 已处理：根包本地用户适配器后端接口化

**EARS:** When 根包默认本地用户适配器调用 internal 用户模块时，系统应依赖消费侧窄接口，而不是直接持有 internal 用户 Manager 具体类型。

**处理结果:** 已新增 `localUserBackend` 消费侧接口，组合 internal 用户模块的用户、权限、凭证、认证和 token 契约；`localUserManager.inner` 从 `*internal/user.Manager` 改为接口字段，并新增测试锁定该结构。

### 暂不处理：根包 Engine inner 接口化

**EARS:** When 强行接口化会暴露更多内部类型或扩大接口面时，系统应保留当前具体依赖，避免为了接口而接口。

**评估结果:** 根包 `Engine.inner` 若改为接口，需要把 internal engine 的未导出 iterator 返回类型纳入根包接口签名，反而扩大根包对 internal 细节的认知。当前暂不接口化，保留公共 facade 到 internal engine 的具体封装。

### 暂不处理：标准库与模块内部私有实现接口化

**EARS:** When 具体类型只是同模块内部实现细节或标准库基础设施封装时，系统应保留具体类型，避免增加无替换需求的抽象。

**评估结果:** `internal/service.Server.server *http.Server`、`queryservice.Service` 内部 `resultCache/auditLog`、`LocalMetadataStore.catalog *catalog.Catalog`、底层 WAL/SSTable 文件句柄和读写器属于实现细节或标准库基础设施封装；当前接口化收益低，且会扩大测试桩和生命周期管理复杂度，暂不处理。
