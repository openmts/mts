# MTS Dashboard 前端审计报告

- **时间**: 2026-07-19 18:01
- **范围**: `cmd/mts-dashboard/**`（API client / 路由鉴权 / 全部页面 / 布局组件）
- **对照**: `cmd/mts-server` HTTP 契约与当前嵌入式运行行为
- **状态约定**: `open` 未修 / `partial` 部分改善 / `fixed` 已修

---

## 1. 总览结论

Dashboard 作为 **Vue3 + TS + Vite + Tailwind v4 SPA**，已覆盖查询/写入/运维/用户/配置等主路径，近期补了取消查询、流式统计、概览刷新等体验点。但当前仍存在 **鉴权闭环缺陷、API 契约误用、交互危险操作、流式“假流”、管理面权限菜单未分流** 等问题；其中 **登出不跳转、403 被当 401、EXPLAIN 结果类型错绑** 属于高优先级。

| 维度 | 评级 | 摘要 |
|---|---|---|
| 鉴权与会话 | 差 | 前端强制登录与后端 `require_user=false` 脱节；登出不跳转；403 误清会话；不校验 `expires_at` |
| API 契约 | 中下 | 数据库列表字段语义混乱；EXPLAIN 当行表渲染；stats 全局竞态；无 admin_token 入口 |
| 交互体验 | 中 | 危险操作用 `confirm/prompt`；查询/删除混页；流式整包加载；空态/错误提示不一致 |
| 布局与信息架构 | 中 | 侧栏完整但无角色菜单裁剪；配置/审计偏“JSON 控制台”；缺 404/帮助/默认账号提示 |
| 工程与质量 | 中下 | 无前端测试；`QueryPage` 超 400 行；类型松散；无 a11y/多标签会话同步 |

---

## 2. 问题清单（按优先级）

### P0 — 必须优先修

#### P0-01 登出后仍停留在受保护页面
- **位置**: `TopBar.vue` + `useAuth.ts` + `router/index.ts`
- **现象**: `logout()` 只清 token/`isAuthenticated`，不 `router.push('/login')`；路由守卫仅在导航时运行，用户仍可见当前页 UI。
- **影响**: 看起来像“退出无效”，后续操作 401 才跳转。
- **建议**: 登出成功后强制 `router.replace({ name: 'Login' })`；可选清除页面级敏感状态。
- **状态**: fixed

#### P0-02 403 与 401 一视同仁清会话
- **位置**: `api/client.ts` `request` / `apiPostText`
- **现象**: `response.status === 401 || response.status === 403` 均 `clearAuth` + 跳登录。
- **影响**: 普通用户访问管理 API（无权限）会被踢下线，而不是提示权限不足。
- **建议**: 仅 401/`unauthenticated` 清会话；403 保留登录态并展示业务错误。
- **状态**: fixed

#### P0-03 EXPLAIN 模式把列式结果当行表渲染
- **位置**: `QueryPage.vue` explain 分支
- **契约**: `/api/v1/data/query/explain` 返回 `QueryResult{ columns: []ColumnSeries, explain, stats }`
- **现象**: 代码 `rows.value = data.result.columns as QueryResultRow[]`，模板按 `timestamp/measurement/tags/fields` 渲染。
- **影响**: EXPLAIN 表格内容错误/空白，误导排查。
- **建议**: EXPLAIN 仅展示 `explain` + `stats`（及可选 columns 原始 JSON），不要复用行表。
- **状态**: fixed

#### P0-04 前端强制登录 vs 后端默认可无用户
- **位置**: 路由守卫 + `bootstrapDefaultAdmin`（仅 `require_user=true` 时预置 `admin/admin`）
- **现象**: Dashboard 无 token 必进登录页；演示配置 `require_user:false` 时启动不创建用户，首访无法登录。
- **影响**: 运维/POC 首次部署卡死（本次环境已手动建 admin 才可用）。
- **建议（择一）**:
  1. 演示模式 Dashboard 支持“兼容无登录”或显示 bootstrap 引导；
  2. 服务启动（即便 `require_user=false`）也预置可登录 admin；
  3. 登录页文案明确默认账号策略与创建路径。
- **状态**: fixed

### P1 — 高优先级

#### P1-01 Token 只看“有没有”，不看是否过期
- **位置**: `useAuth.ts` `isAuthenticated = !!getBearerToken()`；`LoginResponse.expires_at` 未使用
- **影响**: 过期 token 仍可进壳子，首个 API 401 才跳转；刷新后短暂“假登录”。
- **建议**: 解析/存储 `expires_at`，前端预检；临近过期提示。
- **状态**: fixed

#### P1-02 流式查询并非真流式
- **位置**: `apiPostText` + Query 流式模式
- **现象**: `response.text()` 一次性读完全部 NDJSON；大结果占内存、无进度、无法边下边看。
- **影响**: 与“流式”产品承诺不符；大查询易卡死浏览器。
- **建议**: `ReadableStream` + 分行解析 + 增量表格/计数；保留取消。
- **状态**: fixed

#### P1-03 查询 stats 全局竞态 / 覆盖
- **位置**: `QueryPage.vue` 查询结束后无条件再 GET `/api/v1/data/query/stats`
- **现象**: stats 是服务端“最近一次”全局快照；并行查询/多标签会串；流式已解析 `type=end.stats` 仍可能被覆盖。
- **建议**: 行/列模式用响应内 stats；流式仅用 end 记录；去掉无条件二次拉取或改为 opt-in。
- **状态**: fixed

#### P1-04 取消判定脆弱 + finally 清空 AbortController
- **位置**: `QueryPage.vue`
- **现象**: 靠 `actionError === '查询已取消'` 字符串判断；`finally` 置 `queryAbort=null` 后无法区分竞态请求。
- **建议**: 用请求代数 `requestId` / `AbortSignal.aborted`；忽略过期响应。
- **状态**: fixed

#### P1-05 纳秒时间戳在 JS Number 中精度风险
- **位置**: Write/Query 的 `Date.now()*1e6`、`parseInt`、`formatTimestamp(ns/1e6)`
- **证据**: `ms*1e6` 经 IEEE754 可能偏移数十 ns（例：`...0123ms` → delta 64）。
- **影响**: 展示不准；写入时间戳可能非预期对齐；>2^53 整型不可精确表示。
- **建议**: 时间戳全程字符串/BigInt；UI 用 ms/RFC3339 输入；展示避免假装 ns 精确。
- **状态**: fixed

#### P1-06 管理菜单未按角色裁剪
- **位置**: `SidebarNav.vue` 固定 10 项；普通用户也能点进 Users/Config/Operations 等
- **影响**: 一堆 403（再叠加 P0-02 会被踢登录）。
- **建议**: 按 `role` 过滤导航；页面级权限空态。
- **状态**: fixed

#### P1-07 查询页承载删除（危险操作混入）
- **位置**: `QueryPage.vue` “按范围删除”
- **现象**: 与查询同表单；仅 `confirm`；无 tag filter/ dry-run/影响预估。
- **影响**: 误删风险高。
- **建议**: 独立“数据治理”区或二次确认+必须输入 measurement；展示将删范围摘要。
- **状态**: fixed

#### P1-08 无 `admin_token` / Data Token 配置入口
- **位置**: 全站仅 Bearer user token
- **影响**: 生产若启用 `auth.admin_token` 且无用户体系时，Dashboard 无法配置服务级 token；`data_tokens` 同理。
- **建议**: 设置页支持可选 Admin Token / Data Token（sessionStorage），client 自动附加。
- **状态**: fixed

### P2 — 体验 / 契约 / 稳健性

#### P2-01 数据库列表响应字段语义误导
- **契约**: `GET /api/v1/admin/databases` → `{ measurements: string[] }`（实际是数据库名）
- **前端**: 多处 `DatabaseListResponse { measurements }` 表示 databases
- **影响**: 新人易在 data plane measurements API 与 admin databases 间写错。
- **建议**: 服务端改为 `databases`（POC 可直接改）或前端类型命名 `databaseNames` 并集中封装 `listDatabases()`。
- **状态**: fixed

#### P2-02 API Client 错误处理不一致
- **位置**: `request` vs `apiPostText`
- **现象**: 后者失败 code 恒为 `bad_request`；JSON 解析失败抛原生异常；GET 也带 `Content-Type: application/json`。
- **建议**: 统一 `parseAPIError`；非 JSON body 安全处理；GET 不强制 Content-Type。
- **状态**: fixed

#### P2-03 写入路径校验不足
- **位置**: `WritePage.vue` / `usePointParsers.ts`
- **现象**: `parseInt/parseFloat` 失败可产生 NaN 字段；line protocol 不支持转义/引号空格完整语义；无预览校验计数差异；未暴露真正列式 `write/typed` 构造器（仅 points-typed）。
- **建议**: 提交前过滤非法点；解析错误行号报告；可选 typed batch 高级模式。
- **状态**: fixed

#### P2-04 数据库详情加载状态机缺陷
- **位置**: `DatabasesPage.vue` `toggleExpand`
- **现象**: 失败时 `loaded=false` 但 `expanded` 仍 true；无手动刷新；series 全量拉取可能巨大。
- **建议**: 失败折叠或显示 retry；series 分页/limit。
- **状态**: fixed

#### P2-05 用户权限操作错误吞没
- **位置**: `UsersPage.vue` `revokeAllPermissions` 无 try/catch；批量 grant 只报失败数
- **建议**: 逐条错误汇总；按钮 loading；成功 toast。
- **状态**: fixed

#### P2-06 运维页 `actionLoading` 单槽位互斥
- **位置**: `OperationsPage.vue`
- **现象**: 一个字符串状态，刷新/统计/错误按钮互相覆盖；`refreshObservability` 内 quiet 加载失败会 reject 整链。
- **建议**: 分项 loading 或 boolean map；Promise.allSettled。
- **状态**: fixed

#### P2-07 配置页只读 JSON + 热重载不回读
- **位置**: `ConfigPage.vue`
- **现象**: 验证用当前内存 config，reload 成功后不重新 GET effective；用户看不到最新配置。
- **建议**: reload 后刷新；关键字段表单化（至少 log.level / limits）。
- **状态**: fixed

#### P2-08 审计/存储页能力偏薄
- **Audit**: 必须先选用户，无全局事件流、无时间过滤、无导出。
- **Storage**: 导出仅页面内 JSON，无“下载文件”；快照无列表/清理。
- **状态**: fixed

#### P2-09 概览信息未用满后端字段
- **位置**: `OverviewPage.vue`
- **现象**: `StorageMemorySnapshot` 含 `peak_bytes/runtime_rss_bytes/rejected_writes...` 未展示；compaction 详情字段很多但 UI 只 4 项；无自动刷新间隔。
- **建议**: 增加 RSS/peak/拒绝写入；可选 5s/30s 自动刷新。
- **状态**: fixed

#### P2-10 空态 / 成功反馈不一致
- **现象**: 部分页静默 catch（Query 库列表失败无提示）；有的只用红条；几乎无成功 toast（写成功有，改密成功无）。
- **建议**: 统一 `useNotify`：error/success/info。
- **状态**: fixed

#### P2-11 路由与工程缺项
- - 无 404 路由
- - 无前端单测 / 组件测 / e2e（Playwright 等）
- - `QueryPage.vue` ~468 行（超出项目“文件 ≤300 行”约定）
- - 无多标签 token 同步（`storage` 事件）
- - 登录 `redirect` 未校验仅允许站内相对路径
- **状态**: fixed

#### P2-12 可访问性与危险确认
- - Modal（UserModals）无 focus trap / Esc
- - 删除/flush/compact 用 `window.confirm`，移动端与 a11y 差
- - 按钮 loading 时缺少 `aria-busy`
- **状态**: fixed

### P3 — 增强项（非阻塞）

| ID | 项 | 说明 | 状态 |
|---|---|---|---|
| P3-01 | 查询结果可视化 | 时序图/字段选择，而不只是表/原始 JSON | open |
| P3-02 | 写入支持真正 TypedBatch UI | 对宽表列式入口，而不只 points-typed 转换 | open |
| P3-03 | 查询历史 / 收藏 | localStorage 保存最近查询 | open |
| P3-04 | 暗色主题 | 现有 tailwind 易扩展 | open |
| P3-05 | i18n | 当前中文 hardcode，可接受 | open |
| P3-06 | 子路径部署 `base` | 若将来挂到 `/mts/` 需 vite base + router base | open |

---

## 3. 页面级速览

| 页面 | 主要问题 |
|---|---|
| Login | 无默认账号/引导；redirect 未白名单 |
| Overview | 字段利用率低；无自动刷新；依赖 public healthz（可用） |
| Query | P0 EXPLAIN；假流式；stats 竞态；删除混入；ns 精度 |
| Write | 校验弱；解析器简化；精度；DB 加载失败静默 |
| Databases | 字段语义；展开状态机；series 无上限 |
| Users | 菜单权限；revoke 错误；isAuthenticated 未用 |
| Config | 只读；reload 不刷新 |
| Operations | loading 互斥；确认简陋 |
| Downsample | 基本可用；缺策略校验提示统一化 |
| Audit | 能力过窄 |
| Storage | 无下载/列表 |
| Layout/Nav | 无角色裁剪；logout 不跳转 |

---

## 4. 推荐修复顺序（实施时）

1. **鉴权闭环**: P0-01/02/04 + P1-01/06/08  
2. **查询正确性**: P0-03 + P1-03/04/05 + P1-02  
3. **危险操作**: P1-07 + 统一 confirm 组件  
4. **API 封装**: listDatabases/listMeasurements、错误码统一（P2-01/02）  
5. **运维可观测**: Overview/Operations 字段补齐与 allSettled（P2-06/09）  
6. **工程债**: 拆分 QueryPage、补关键单测、404 路由（P2-11）

---

## 5. 验收建议（修后）

- 登录/登出/过期 token/无权限 403 四条路径手工验收  
- EXPLAIN / 行查询 / 列查询 / 流式取消 契约对照 curl  
- 普通用户角色：侧栏仅见允许项，访问管理 API 不掉登录  
- 写入非法数字/空 fields 被前端拦截  
- `npm run build` + 嵌入后 `make test`/`e2e`/`lint`（Go 侧）  

---

## 6. 处理状态跟踪

| ID | 优先级 | 状态 | 备注 |
|---|---|---|---|
| P0-01 | P0 | fixed | 登出跳转 |
| P0-02 | P0 | fixed | 403≠401 |
| P0-03 | P0 | fixed | EXPLAIN 渲染 |
| P0-04 | P0 | fixed | 首访账号 |
| P1-01 ~ P1-08 | P1 | fixed | 见上 |
| P2-01 ~ P2-12 | P2 | fixed | 见上 |
| P3-01 ~ P3-06 | P3 | deferred | 本轮不做 |

---

## 7. 技能与方法

- 代码通读：`src/api`、`router`、`composables`、全部 `pages`、布局组件  
- 契约对照：`protocol_types.go` / `result_types.go` / `operation_registry` / 实机 API  
- 运行时验证：登录 token 结构、compaction/memory stats 真实字段、healthz  


## 8. 本轮闭环

- 状态：P0-P2 已全部 fixed；P3 deferred
- EARS 清单：docs/plans/dashboard-ears-2026-07-19.md
- 验证：npm run build + make test + make e2e + make lint 通过
