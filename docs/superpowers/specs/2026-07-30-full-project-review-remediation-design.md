# MTS 全项目检视整改设计

## 1. 目标

在不降低主库写入/查询性能、不增加 Dashboard 生产 bundle 风险的前提下，闭环 2026-07-30 总检视发现的鉴权、凭据生命周期、失败原子性、并发治理、协议一致性、覆盖率和性能门禁问题。

## 2. EARS 需求

1. 当项目执行 Go 全量测试时，主库、`mts-server` 和全部 Go E2E 必须通过。
2. 当执行 Go/前端漏洞扫描时，项目不得存在 high/critical 或 Go 实际可达漏洞。
3. 当执行生产包覆盖率门禁时，每个列入门禁的 Go 包覆盖率必须不低于 90%。
4. 当执行 benchmark gate 时，若版本化基线不存在，gate 必须失败。
5. 当任何整改修改完成时，系统必须使用同机前后样本证明 `ns/op` median 未劣化超过 10%，并检查 `B/op`、`allocs/op`。
6. 当 Dashboard 依赖或代码变化时，单元测试、typecheck、构建和 Playwright 商业冒烟必须通过，主入口及总 bundle gzip 增长不得超过 5%。
7. 当拆分超大文件时，系统行为、HTTP/gRPC 契约和前端路由/交互必须保持不变。
8. 当调用 Registry 标记为 `authAdmin` 的 gRPC 操作时，匿名和普通用户必须被统一拒绝。
9. 当服务首次启动或已有管理员被禁用/降权后重启时，系统不得创建公开固定凭据或恢复已处置账号权限。
10. 当 Retention 或 Downsample 元数据持久化失败时，运行态不得暴露半提交状态，重试和重启后状态必须一致。
11. 当配置热重载与请求并发发生时，系统不得产生 data race、旧新 limiter 叠加或“响应成功但运行态未生效”。
12. 当 HTTP/gRPC 提供等价能力时，鉴权、TTL、timeout、request ID、metrics、access log、错误映射和审计语义必须一致。
13. 当用户登出、会话过期或改密时，Dashboard 必须清理全部会话关联凭据；远端撤销必须使用统一 API base，并明确报告撤销失败。
14. 当导出 CSV 时，任何用户可控字段不得被电子表格解释为公式。
15. 当 Dashboard 快速切换元数据上下文时，旧异步响应不得覆盖当前页面状态。
16. 当 Dashboard 部署在子路径时，分享链接、路由和 API 请求必须保留部署前缀。
17. 当 Access Grants 展示大量用户时，网络请求数不得随总用户数线性增长。
18. 当使用虚拟表格和图标按钮时，界面必须提供合法的表格/列表语义和可访问名称。

## 3. 方案选择

### 方案 A：先恢复可信门禁，再修安全和覆盖率（推荐）

先修两个测试 fixture，随后立即封堵鉴权和固定凭据风险；再修失败原子性与并发问题，建立 benchmark gate，升级依赖，最后补覆盖率和分批拆文件。优点是先恢复可信基线，同时不延迟高危安全修复。

### 方案 B：先升级全部依赖

可最快消除已知漏洞，但当前全量测试和性能门禁不可信，升级后难以区分依赖变化与既有漂移，风险较高。

### 方案 C：一次性全量重构和门禁升级

理论上可快速清债，但修改面过大，无法归因性能变化，也不符合最小修改和逐项闭环要求。

采用方案 A。

## 4. 架构与数据流

整改不改变存储格式或 Dashboard 信息架构。除 2026-07-30 经用户单独批准的 Access Grants 聚合读取契约外，现有公共 Go API 与 HTTP/gRPC schema 保持兼容；该新增契约不得改变既有用户和单用户授权接口。

验证流为：

```text
可信测试基线
  -> 鉴权/固定凭据封堵
  -> 存储失败原子性与 server 并发治理
  -> Dashboard 凭据/导出安全
  -> 版本化 benchmark 基线
  -> 安全依赖升级
  -> Dashboard 正确性/可扩展性/无障碍
  -> 覆盖率补齐
  -> 工程门禁补齐
  -> 行为保持型文件拆分
  -> 全量 test/lint/race/e2e/audit/bench
```

## 5. 错误处理

- benchmark 基线缺失返回非零退出码，不再软成功。
- benchmark 比较超过阈值返回非零退出码，不只打印统计结果。
- 漏洞扫描不可用时门禁失败并输出数据源错误，不得当作无漏洞。
- 测试 fixture 使用生产策略常量或公共策略接口，避免复制易漂移的字面值。
- 依赖升级失败时只回退该依赖任务，不混入结构重构。
- Retention/Downsample 采用“持久化成功后提交内存”或等价事务语义，错误时保持调用前状态。
- 热重载只返回实际生效字段；不支持热更新的字段应明确拒绝并要求重启。
- 对外错误按稳定 code/message 返回，内部路径和底层错误仅写服务端日志。
- Dashboard 登出区分本地清理与服务端撤销结果，但无论远端是否成功都不得保留本地敏感凭据。

## 6. 测试策略

- 契约漂移：定向测试先失败，再修 fixture 并观察通过。
- gRPC 权限：自动枚举 Registry 权限矩阵，覆盖匿名、普通用户和管理员。
- 固定管理员：覆盖全新目录、已有管理员、禁用/降权管理员和迁移兼容。
- 失败原子性：使用 file operation 故障注入覆盖 write/rename/fsync/remove 失败和重启恢复。
- 并发：`-race` 覆盖 reload + HTTP/unary/stream、audit record + close，并验证限流上限。
- 依赖升级：`govulncheck`、`npm audit`、全量协议 E2E、race。
- 覆盖率：逐包补错误路径和生命周期测试，不在生产代码加测试 hook。
- 性能：升级/拆分前后 `count=10`，用 `benchstat` 比较。
- Dashboard：单测覆盖 token 生命周期、API base、CSV 公式、异步交错、子路径链接和 a11y；再执行 typecheck、production build、Playwright smoke、bundle size 对比。
- Access Grants：记录 10/100/1000 用户下的请求数、首屏耗时与服务端开销。

## 6.1 Access Grants 聚合分页契约（Task 10，已批准）

### 接口与数据流

- 新增管理员专用 HTTP 接口 `GET /api/v1/users/access-grants?limit=100&cursor=<username>`。
- `limit` 缺省时使用 100，允许范围为 1 到 200；超过范围或无法解析时返回稳定的 `bad_request`。
- `cursor` 是上一页最后一个用户名，服务端按用户名升序返回严格位于 cursor 之后的用户；空 cursor 表示第一页。cursor 必须是合法用户名格式，但分页期间用户被删除不应使后续页失败。
- 响应包含 `items`、`total_users`、`next_cursor`、`path` 以及现有 `admin_op_busy/op/started_at_unix/last` 元数据。每个 item 包含完整用户资料与该用户按 database/permission 稳定排序的 `grants`；没有授权的用户仍保留为空 grants item。
- `next_cursor` 仅在还有下一页时返回；Dashboard 维护已访问 cursor 栈，实现上一页/下一页，不猜测反向 cursor。
- 底层用户管理器在一次 `RLock` 内确定排序用户、页边界并复制当前页用户与授权，保证单个响应中的两类数据来自同一快照；HTTP handler 不循环调用单用户授权接口。
- Dashboard 首屏和每次翻页只调用一次聚合接口，不再调用 `/api/v1/users` 或逐用户 `/database-permissions`；筛选、选择、统计与 CSV/JSON 导出只作用于当前页，并在界面中明确当前页语义。
- 既有 `/api/v1/users` 与 `/api/v1/users/{name}/database-permissions` 契约继续可用，不增加写入路径，不改变存储格式。

### EARS 验收

1. 当管理员打开或刷新 Access Grants 页面时，系统应使用一次聚合请求返回当前用户页及其授权，请求数不得随总用户数增长。
2. 当管理员指定合法 `limit` 和 cursor 时，系统应按用户名稳定分页，不重复、不遗漏静态快照中的用户，并为无授权用户返回空 grants。
3. 当 `limit` 缺省时，系统应使用 100；当 `limit` 小于 1、大于 200 或无法解析时，系统应返回 `bad_request`。
4. 当匿名或普通用户调用聚合接口时，系统应分别返回 401 和 403；响应不得泄露用户或授权数据。
5. 当授权或用户变更与一次读取并发发生时，该次响应中的用户与授权应来自同一读锁快照，并通过 race 测试。
6. 当用户从 10 增长到 1000 时，Dashboard 首屏请求数应保持 1；默认响应最多包含 100 个用户，显式响应最多包含 200 个用户。
7. 当用户翻页时，Dashboard 应提供上一页/下一页导航，并将筛选、选择、统计和导出明确限定为当前页。
8. 当实现完成时，应记录 10/100/1000 用户下旧 `U+1` 模型与新接口的请求数、服务端耗时、响应体积和 Dashboard 可交互时间；server benchmark 的 `ns/op`、`B/op`、`allocs/op` 不得无说明劣化，生产 bundle gzip 增长不得超过 5%。

### 响应 schema

```json
{
  "items": [
    {
      "user": { "name": "alice", "role": "user", "disabled": false },
      "grants": [
        { "database": "metrics", "permission": "read" }
      ]
    }
  ],
  "total_users": 1000,
  "next_cursor": "alice",
  "path": "/api/v1/users/access-grants",
  "admin_op_busy": false
}
```

## 7. 非目标

- 不改变存储磁盘格式。
- 不增加分布式、SQL/PromQL parser 或对象存储能力。
- 不借本轮整改重做 Dashboard 视觉设计。
- Access Grants 的 shared contract 授权仅限第 6.1 节定义的 additive HTTP 读取接口；不得借此修改既有接口、增加 gRPC 对等接口或扩展写入 schema。
- 不承诺一次性把所有超过 300 行的历史文件拆完；按可独立验证的边界分批完成。

## 8. 自检

- 无未决占位符。
- 安全、鉴权、失败原子性、并发、正确性、覆盖率、性能和结构债均有验收证据。
- 设计没有把“测试通过”替代为“无性能劣化”，性能需要独立比较。
