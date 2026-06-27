# MTS IoC 模块化深化设计

## 背景

上一轮已完成架构门禁、共享 storage query 契约、runtime 用户组合层、metadata snapshot 隔离和用户模块细粒度接口。本轮继续落实用户提出的目标：平行模块安全解耦，上下层模块通过依赖注入与控制反转处理依赖问题。

## 目标

- When engine 打开本地实例时，系统应通过 `Deps` 注入 metadata store 和 shard deps；默认依赖由独立构造函数补齐，而不是散落在 `Open` 中。
- When 测试或未来运行时需要替换 engine 下层组件时，系统应能注入 metadata store opener、WAL/MemTable/SSTable/file ops，而不修改 engine 主逻辑。
- When 查询服务执行结构化查询时，系统应始终经过 analyzer、logical planner、optimizer、physical planner，再进入 engine 的 QuerySpec 入口，并有测试防止回退到纯 compat executor。
- When 用户 Manager 管理本地状态时，系统应通过本地 state store 和组件视图组织用户、权限、凭证、token 能力，Manager 作为门面组合这些组件。
- When 根包默认本地用户适配器调用 internal 用户模块时，系统应依赖消费侧窄接口，而不是直接持有 internal 用户 Manager 具体类型。

## 非目标

- 不重写 SSTable/MemTable 存储格式。
- 不一次性替换 queryexec 执行器为 physical plan 驱动实现。
- 不改变公共 API 行为。
- 不修改 `cmd/` 目录。

## 设计

### Engine Deps

新增导出到 `internal/engine` 包内的 `Deps` 类型，包含：

- `OpenMetadataStore func(dir string) (MetadataStore, error)`
- `Shard shardDeps`

`Open(ctx, opts)` 调用 `OpenWithDeps(ctx, opts, Deps{})`。`OpenWithDeps` 负责归一化依赖并注入 shard options。默认 metadata opener 仍使用 `OpenLocalMetadataStore`。

### Query 主链路约束

保留当前 `LayeredExecutor` 执行模型，但新增测试明确它必须调用 analyzer/planner/optimizer/physical 后再调用 `QuerySpecRows` 或 `QuerySpecWithExplain`。这让 queryservice 的默认推荐路径被测试保护，CompatExecutor 只作为显式兼容路径存在。

### User Manager 内部组合

新增 `localStateStore` 管理本地状态加载、克隆和原子替换。Manager 持有 `store *localStateStore`，并提供 `Users()`、`Permissions()`、`Credentials()`、`Tokens()`、`Roles()` 组件视图方法。第一步保持现有方法签名和行为不变，把直接 path/load/replace 的职责下沉到 store。

### 根包用户适配器接口化

根包 `localUserManager` 新增消费侧 `localUserBackend` 接口，组合 internal 用户模块已暴露的用户、权限、凭证、认证和 token 契约。默认 runtime 仍返回本地 concrete Manager，但根包适配器只依赖接口能力，避免 facade 直接绑定 internal 具体实现。

## 验收

- engine 新增 `OpenWithDeps` 并有测试验证自定义 metadata opener 被使用。
- shard deps 仍可注入并通过现有 engine 测试。
- queryservice 有测试验证 layered executor 调用 QuerySpec 路径且输出 logical/physical/pushdown 信息。
- user Manager 有组件视图测试和 store 持久化测试。
- 根包本地用户适配器通过消费侧接口持有 internal 用户后端。
- `go test ./...`、`golangci-lint run ./...`、`git diff --check` 通过。
