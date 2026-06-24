# 用户管理与 DB 级权限设计

## 背景

当前 MTS 已有本地 metadata、database/retention policy 管理和 `internal/queryservice` 的 tenant allowlist 授权器，但没有公开的用户管理模块。现有授权器只能按 tenant 粗粒度放行查询，不提供用户增删改查，不维护用户到 database 的权限，也不适合作为外部用户可商用的权限接口。

本设计新增 public package 用户管理接口，默认提供本地二进制持久化实现，并在 Engine 上暴露用户 CRUD 和 DB 级权限管理方法。当前阶段只做授权数据管理和权限校验，不实现密码认证、token 签发、HTTP 登录或第三方系统适配器。

## 范围

本次包含：

- public `UserManager` 接口，便于后续对接 LDAP、OIDC、IAM、自研权限系统。
- 默认 `LocalUserManager`，随 `Engine` 默认打开本地用户元数据文件。
- 用户 CRUD：创建、更新、查询、列表、删除。
- DB 级权限：授予、撤销、列出、校验。
- 权限粒度：database + `read/write/admin`。
- `Options.UserManager` 支持注入第三方实现；未注入时使用本地默认实现。
- 二进制 envelope 持久化，避免 JSON 存储格式。

本次不包含：

- 密码、登录、session、token、API key、mTLS 用户认证。
- measurement、retention policy、series、tag、field、row 级权限。
- 默认拦截现有 `Write`、`QueryRows`、`QueryColumns` 等无用户上下文 API。调用方可通过 `CheckUserDatabasePermission` 在入口层显式校验；后续如新增带用户上下文的读写 API，可复用同一 `UserManager`。
- 分布式权限同步或外部一致性元数据系统。

## 核心语义

用户由 `User{Name, DisplayName, Disabled, Metadata}` 表示。`Name` 是稳定唯一标识，创建后不可通过 `UpdateUser` 改名。`Metadata` 仅存放非敏感扩展字段，默认实现不会存储密码、token 或密钥。

权限由 `DatabasePermission` 表示：

- `DatabasePermissionRead`
- `DatabasePermissionWrite`
- `DatabasePermissionAdmin`

`admin` 隐含 `read` 和 `write`。用户 `Disabled=true` 时所有权限校验失败。不存在的用户、空用户名、空 database 或非法 permission 均返回明确错误。

本地实现持久化到 `users.bin`，使用内部 `codec.Envelope`，文件权限由 `storagefs.WriteFileAtomic` 保证为 `0600`，目录为 `0700`。

## EARS 清单

- When 打开 Engine 且未配置 `Options.UserManager` 时，系统应创建默认本地用户管理器。
- When 打开 Engine 且配置了 `Options.UserManager` 时，系统应使用调用方注入的用户管理器，不创建默认本地用户文件。
- When 调用方创建用户时，系统应校验用户名非空、用户不存在，并持久化用户。
- When 调用方更新用户时，系统应校验用户已存在，并只更新显示名、禁用状态和 metadata。
- When 调用方查询用户时，系统应返回用户快照和是否存在，不暴露内部 map 引用。
- When 调用方列出用户时，系统应按用户名稳定排序返回用户快照。
- When 调用方删除用户时，系统应删除用户及其所有 database 权限。
- When 调用方授予 DB 权限时，系统应校验用户存在、database 非空、permission 合法，并持久化权限。
- When 调用方撤销 DB 权限时，系统应校验用户存在，并从该用户指定 database 中删除对应权限。
- When 调用方列出 DB 权限时，系统应按 database 和 permission 稳定排序返回。
- When 用户拥有 admin 权限时，系统应允许 read/write/admin 权限校验。
- When 用户被禁用或不存在时，系统应拒绝权限校验。
- When 本地用户管理器重启后，系统应恢复用户和 DB 权限数据。
- When 本地用户元数据文件损坏时，系统应在打开默认用户管理器时返回错误，避免静默丢权限。
- When 上下文已取消时，用户管理方法应返回 context 错误，不应继续修改状态。

## 验收标准

- public API 测试覆盖用户 CRUD、metadata clone、权限 grant/revoke/list/check、disabled 用户、admin 隐含权限、错误路径、持久化重启。
- custom `UserManager` 注入测试证明 Engine 使用外部实现。
- e2e public API workflow 覆盖默认用户管理器的创建、权限授予、重启后校验。
- no-json e2e 保持通过，用户 metadata 不写 JSON。
- `make fmt`、定向测试、`make ci`、`make e2e-public-api`、`make e2e-no-json`、`git diff --check` 通过。
