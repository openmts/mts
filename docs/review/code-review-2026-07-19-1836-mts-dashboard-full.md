# MTS Dashboard 全量复审报告

- **时间**: 2026-07-19 18:36
- **范围**: `cmd/mts-dashboard/**`（对照 `cmd/mts-server` HTTP 契约）
- **前置**: `c246f5c` 已闭环上一轮 P0–P2
- **目的**: 回答「是否还有未完整修复项」并给出新一轮 EARS 清单

---

## 1. 结论

### 1.1 上一轮检视（2026-07-19-1801）完成度

| 批次 | 状态 | 说明 |
|---|---|---|
| P0-01 ~ P0-04 | **已完整修复** | 登出跳转、403≠401、EXPLAIN JSON、bootstrap admin |
| P1-01 / 02 / 04 / 05 / 06 / 07 / 08 | **已完整修复** | 过期预检、真流式、requestId、ms 精度、菜单裁剪、删除确认、服务 Token |
| **P1-03** | **部分完整 / 有回归** | 去掉全局 `/query/stats` 竞态正确，但 **rows/columns 响应本身不含 stats**，当前行/列模式几乎总是无统计卡片 |
| P2-01 ~ P2-11 主体 | **已修复** | meta 封装、错误统一、校验、重试、notify、404、拆分等 |
| **P2-12** | **部分修复** | Modal Esc 有；多处仍用 `window.confirm` / `prompt` |
| P3-01 ~ P3-06 | **未做（deferred）** | 图表、TypedBatch UI、历史、暗色、i18n、子路径 |

**一句话**：上一轮“阻断级”问题已基本清掉；仍有 **查询统计缺口（P1-03 残留）**、**危险确认未完全组件化**、以及若干 **角色/页面能力边界** 与 **工程债**。

### 1.2 本轮复审总评

| 维度 | 评级 | 说明 |
|---|---|---|
| 鉴权会话 | 良 | 登录/登出/过期/403 分流可用；角色推断偏弱 |
| 查询正确性 | 中上 | EXPLAIN/流式正确；行/列无 stats |
| 权限信息架构 | 中 | 管理菜单裁剪 OK；数据库页未区分 admin；概览对非 admin 噪声大 |
| 交互一致性 | 中 | 删除范围强确认；其它危险操作仍 confirm/prompt |
| 工程与质量 | 中下 | 无前端测试/lint；UsersPage 414 行；apiPostText 死代码 |
| 增强能力 | 弱 | 无图表/历史/主题（P3） |

---

## 2. 未完整修复 / 新发现问题清单

### P0（本轮无新增阻断）

无。上一轮 P0 均已闭合。

### P1 — 应优先修

#### DASH-P1-01 行/列查询无 stats（P1-03 残留回归）
- **位置**: `useQueryWorkbench.ts` rows/columns 分支
- **契约**: `queryRowsResponse` / `queryColumnsResponse` **仅** `rows` / `columns`，不含 `stats`
- **现象**: 前端读 `data.stats` 永远为空；仅 explain/stream end 有统计
- **建议（择一，POC 可直接改契约）**:
  1. 服务端 rows/columns 响应附带 `stats`（推荐）；或
  2. 前端对 rows/columns 显式请求 `/query/stats` **且** 用 requestId 绑定，避免竞态
- **状态**: fixed

#### DASH-P1-02 角色推断不可靠
- **位置**: `useAuth.resolveRole`
- **现象**: 能 `GET /users` 且找不到自己 → 当成 `admin`；403/失败 → `user`
- **影响**: 网络抖动或权限边界变化时菜单/页面权限错误
- **建议**: 登录响应扩展 `role`；或 `GET /api/v1/users/me`；禁止“能 list 就当 admin”
- **状态**: fixed

#### DASH-P1-03 非管理员概览页体验差
- **位置**: `OverviewPage.vue`
- **现象**: 并行拉 `storage-memory` / `compaction` / `maintenance`（管理面）；非 admin 403 导致整页 `loadError`，健康状态也可能被 Promise.all 拖死
- **建议**: 非 admin 只拉 `/healthz`；admin 再拉统计；`allSettled` 降级展示
- **状态**: fixed

#### DASH-P1-04 数据库管理页无角色门禁
- **位置**: `DatabasesPage.vue` + Sidebar `adminOnly: false`
- **现象**: 创建/删除库、RP 管理走 admin API；普通用户可进页面但操作失败；列表也走 admin databases
- **建议**: 侧栏/路由按 admin 裁剪，或拆“浏览元数据（data）”与“管理库（admin）”
- **状态**: fixed

### P2 — 体验与稳健性

#### DASH-P2-01 危险确认未统一
- **位置**: Operations flush/compact/retention；Downsample 删除；Users 删除；Databases `prompt` 删库
- **状态**: fixed（P2-12 残留）

#### DASH-P2-02 流式结果“预览即全部复制”
- **位置**: `useQueryWorkbench` stream 仅保留前 200 行到 `rawOutput`
- **现象**: 复制按钮只复制预览，用户以为是全量
- **建议**: 文案标明预览；或提供“下载完整流”（需落盘/二次请求）
- **状态**: fixed

#### DASH-P2-03 运维保留策略时间戳
- **位置**: `OperationsPage` `Date.now() * 1e6` 作为 `now_unix_nanos`
- **现象**: 可能产生非精确 ns（虽通常可接受）
- **建议**: 不传 now（服务端 now）或用安全整数字符串路径
- **状态**: fixed

#### DASH-P2-04 Line protocol 校验弱于表单
- **位置**: `usePointParsers.parseLineProtocol` 仍 `parseInt` + `isNaN continue`，静默丢字段
- **建议**: 汇总非法行号；与表单一致的安全整数检查
- **状态**: fixed

#### DASH-P2-05 通知/成功反馈覆盖不全
- **位置**: 创建用户/设密/删策略等仍只有 actionError 红条
- **状态**: fixed

#### DASH-P2-06 死代码与文件体量
- `apiPostText` 无引用
- `UsersPage.vue` ~414 行（超 300）
- Databases/Write/Query ~305 临界
- **状态**: fixed

#### DASH-P2-07 前端工程门禁缺失
- 无 `vitest`/`playwright`；`package.json` 无 lint；CI 仅 `vue-tsc`+vite build
- **状态**: fixed

#### DASH-P2-08 降采样表单 UX 粗糙
- interval 默认 `60000000000`（ns）无单位解释；缺少 DB/measurement 下拉联动
- **状态**: fixed

#### DASH-P2-09 Modal a11y 仍弱
- Esc 有；无 focus trap / 初始焦点 / 遮罩点击关闭不完全统一
- **状态**: fixed

#### DASH-P2-10 clearAuth 不清理服务级 Token
- admin/data token 在 sessionStorage 跨登出保留
- 可能期望（运维会话）也可能误用；需产品明确并在 UI 标注
- **状态**: fixed（文档/产品决策）

### P3 — 增强（仍 deferred）

| ID | 项 |
|---|---|
| DASH-P3-01 | 查询时序图 / 字段选择器 |
| DASH-P3-02 | TypedBatch 构造 UI（非 points-typed） |
| DASH-P3-03 | 查询历史与收藏 |
| DASH-P3-04 | 暗色主题 |
| DASH-P3-05 | i18n |
| DASH-P3-06 | 子路径 `base` 部署 |
| DASH-P3-07 | 审计全局流 / 时间过滤 / 导出 |
| DASH-P3-08 | 存储快照列表与清理 |

---

## 3. 页面健康度速览

| 页面 | 行数 | 状态 | 主要残留 |
|---|---|---|---|
| Login | 87 | 良 | — |
| Overview | 208 | 中 | 非 admin 管理 API 噪声 |
| Query | 305 | 中上 | 行/列无 stats；流式预览误解 |
| Write | 305 | 中上 | LP 静默丢点 |
| Databases | 307 | 中 | 无 admin 门禁；prompt 删除 |
| Users | 414 | 中 | 体积；confirm 删除 |
| Config | 158 | 良 | Token 与登出关系需说明 |
| Operations | 181 | 中 | confirm；ns 时间 |
| Downsample | 202 | 中 | confirm；interval UX |
| Audit | 95 | 中 | 能力仍窄 |
| Storage | 105 | 中上 | 已有下载 |
| NotFound | 15 | 良 | — |

---

## 4. 已验证的“修对了”的能力（避免回退）

- 登出 → `/login`
- 401 清会话；403 保留会话
- EXPLAIN 输出 plan/stats/columns JSON，不误绑行表
- 空库启动 bootstrap `admin/admin`（密码认证开启）
- `apiPostNDJSONStream` 真流式 + Abort
- 范围删除独立确认文案
- 侧栏 admin 菜单裁剪（配置/运维/降采样/审计/存储）
- Config 页 Admin/Data Token + reload 回读
- sanitizeRedirect 防开放重定向
- 多标签 `storage` 同步

---

## 5. 推荐下一轮优先级

1. **DASH-P1-01** 行/列 stats（契约或绑定请求）  
2. **DASH-P1-02/03/04** 角色与页面边界（概览降级、数据库门禁、role 可信源）  
3. **DASH-P2-01/02/04** 危险确认组件化、流式预览语义、LP 校验  
4. **DASH-P2-06/07** 拆 UsersPage、删死代码、前端 lint/test  
5. P3 按产品需要排期  

---

## 6. 状态跟踪

| ID | 优先级 | 状态 |
|---|---|---|
| 上轮 P0 | P0 | fixed |
| 上轮 P1（除 03 残留） | P1 | fixed |
| DASH-P1-01 ~ 04 | P1 | fixed |
| DASH-P2-01 ~ 10 | P2 | fixed |
| DASH-P3-01 ~ 08 | P3 | deferred |



---

## 7. R2 实现闭环（2026-07-19）

- 全部 DASH-P1-01~04、DASH-P2-01~10 已实现并验证
- P3 仍 deferred（图表 / TypedBatch UI / 历史 / 暗色 / i18n / base path / 审计增强 / 快照管理）
- 验证：`npm run test` + `npm run build` + `make test` + `make e2e` + `make lint`
