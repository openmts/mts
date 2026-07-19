# MTS Dashboard 前端短板与前后端对齐检视报告

- **时间**: 2026-07-19 19:33
- **范围**: `cmd/mts-dashboard` + `cmd/mts-server` HTTP 契约
- **基线提交**: `42f0550`（P3 闭环后）
- **性质**: 只读检视；输出问题分级与 EARS 任务清单

---

## 1. 结论

Dashboard 已覆盖主要运维闭环（登录鉴权、查询/写入、库/用户、运维、降采样、审计、存储），但与内核/HTTP 契约仍有 **关键功能错位** 与一批 **体验/能力缺口**：

1. **P0 契约错误**：范围删除 JSON 字段名与 `mts.DeleteRequest` 无 `json` tag 的解码规则不对齐；降采样 pause/resume 路径名与后端 enable/disable 不对齐。
2. **P1 权限面错位**：查询/写入元数据依赖 **admin** `list databases / retention-policies`，普通用户即使有 data 权限也会失败。
3. **P1 查询能力暴露不足**：内核 `Query` 的 tags/predicates/aggregates/window/group/order/offset/budget 等前端几乎未接入。
4. **P2 产品化短板**：暗色/i18n 覆盖不全、无共享 API 类型层、前端测试/E2E 仍薄、管理 API 有能力未接（api-spec、maintenance errors、admin health、downsample run/reset/repair/dry-run 等）。
5. **P2 嵌入部署**：`VITE_BASE` 与服务端 `dashboardHandler` 固定根路径托管未对齐，子路径部署前端可构建但服务端不会按 base 服务。

---

## 2. 前后端 API 覆盖矩阵

### 2.1 前端已使用（对齐度：基本可用）

| 能力 | 前端路径 | 后端 | 备注 |
|---|---|---|---|
| 健康 | `GET /healthz` | ✅ | 未用 `/readyz`、`/api/v1/admin/health` |
| 登录/登出 | `/api/v1/auth/login|logout` | ✅ | 未显式传 TTL（服务端默认 12h） |
| 改密 | `POST /api/v1/auth/password` | ✅ | body 字段对齐 |
| 写 points/typed/points-typed | `/api/v1/data/write*` | ✅ | TypedBatch UI 已接 typed |
| 查询 rows/columns/explain/stream | `/api/v1/data/query/*` | ✅ | stats 已内嵌；独立 `/query/stats` 未用（合理） |
| 删数 | `POST /api/v1/data/delete` | ⚠️ | **请求字段名错位** |
| 元数据 meas/fields/series | `/api/v1/data/databases/...` | ✅ | series 未暴露 query 过滤 UI |
| 用户 CRUD/授权 | `/api/v1/users...` | ✅ | |
| 库/RP 管理 | `/api/v1/admin/databases...` | ⚠️ | 查询页也依赖 admin list |
| 运维 flush/compact/retention | admin 路径 | ✅ | retention 空 body → 服务端 now |
| 降采样 list/create/delete/toggle | admin downsample | ⚠️ | **toggle action 名错误** |
| 审计 | `/api/v1/admin/audit` + fallback user audit | ✅ | |
| 存储 validate/snapshot/export/list/delete | admin storage | ✅ | |

### 2.2 后端有、前端未接

| 路径/能力 | 影响 |
|---|---|
| `GET /readyz` | 就绪态未展示 |
| `GET /metrics` | 无内嵌指标浏览 |
| `GET /api/v1/admin/health` | 结构化 checks 未展示 |
| `GET /api/v1/admin/maintenance/errors` | 运维错误明细缺失 |
| `GET /api/v1/admin/config/schema`、`/api-spec` | 配置可发现性差 |
| `GET /api/v1/admin/config`（非 effective） | 弱 |
| `POST /api/v1/authz/database/check` | 前端无法预检权限 |
| downsample `run/reset/repair/dry-run/run-range` | 运维只能 pause 风格切换且还切错 |
| pprof `/debug/pprof/*` | 可接受不进 UI |
| 独立 `GET /api/v1/data/query/stats` | 可继续不用（避免串台） |

---

## 3. 关键错位（按严重度）

### P0 — 功能错误

#### FE-ALIGN-P0-01 范围删除 JSON 契约错误
- **位置**: `QueryPage.vue` `doRangeDelete`；`delete.go` `DeleteRequest`（**无 json tag**）
- **现象**: 前端发送：
  ```json
  {"request":{"database":"...","retention_policy":"...","measurement":"...","start_time":1,"end_time":2}}
  ```
  Go `encoding/json` 对无 tag 字段按 **导出字段名** 解码（`StartTime`/`EndTime`/`Measurement`...），`start_time` **不会绑定**。
- **后果**: 时间范围失效；可能只靠 measurement 等部分字段，或因 measurement 空直接失败；即使用户确认 DELETE，删除语义与 UI 不一致。
- **服务端测试**: `http_test` 用 Go 字面量 `StartTime` 编码，JSON 为 `StartTime`，与前端 snake_case **不一致**。
- **修复方向（二选一，推荐 1）**:
  1. 给 `mts.DeleteRequest` 补齐 json tag（与 `Query` 一致：`start_time` 等）——POC 可直接破兼容。
  2. 或前端改为发送 `StartTime`/`Measurement` 等大写字段（丑且易错）。
- **状态**: open

#### FE-ALIGN-P0-02 降采样启停 action 名错误
- **位置**: `DownsamplePage.vue` `pause`/`resume`
- **后端**: `handleDownsampleAction` 仅识别 `enable`/`disable`/`reset`/`run`/...
- **后果**: 点击暂停/恢复大概率 4xx，策略启停不可用。
- **修复**: 前端改为 `disable`/`enable`；文案可仍显示“暂停/恢复”。
- **状态**: open

### P1 — 权限与核心工作流

#### FE-ALIGN-P1-01 查询/写入元数据走 admin API
- **位置**: `api/meta.ts` `listDatabases` → `/api/v1/admin/databases`；`listRetentionPolicies` → admin RP
- **影响页面**: Query、Write（及任何调用 meta 的路径）
- **现象**: 非 admin 即使用户有某库 read/write，下拉库列表/RP 仍 403。
- **建议**:
  - 提供/使用 data 面可访问的库枚举（若暂无，可让用户手填 database，并对 admin list 失败降级）；
  - RP 手填或 data 面 API。
- **状态**: open

#### FE-ALIGN-P1-02 查询 Builder 与内核 Query 能力落差大
- **内核** `Query` 已支持: `tags`, `predicates`, `expr`, `aggregates`, `window`, `group`, `order`, `offset`, `budget` 等
- **前端** 仅: database/rp/measurement/start/end/fields/limit + 模式切换
- **后果**: Dashboard 无法表达内核已有的过滤/聚合/窗口查询；与“Builder 表达能力清晰”目标仍有差距。
- **建议分阶段**: tags 精确过滤 → order/offset → aggregates+window → budget 高级项。
- **状态**: open

#### FE-ALIGN-P1-03 行结果 FieldValue 展示粗糙
- **后端** `Row.Fields` 为 `map[string]FieldValue`（`{type,float64|int64|string|bool}`）
- **前端** 表格直接 `JSON.stringify(row.fields)`，图表虽能读 float64/int64，表格可读性差。
- **建议**: 统一 `formatFieldValue` 展示标量；复制时可选“展开值/原始 JSON”。
- **状态**: open

#### FE-ALIGN-P1-04 列式查询结果未表格化/未图表化
- **columns/stream-column** 仅塞进 raw JSON；`QueryChart` 只消费 rows。
- **建议**: columns → 转置为时间序列或至少字段级表格。
- **状态**: open

### P2 — 体验、工程、能力缺口

#### FE-ALIGN-P2-01 暗色主题覆盖不全
- TopBar/Sidebar/部分 Query/Write 有 `dark:`；Overview/Databases/Users/Config/Operations/Login 等大量页面仍偏浅色硬编码。
- **状态**: open

#### FE-ALIGN-P2-02 i18n 覆盖浅
- 仅导航/顶栏/少量按钮；页面主体中文硬编码；`useI18n` 无参数插值/复数。
- **状态**: open

#### FE-ALIGN-P2-03 子路径 base 与嵌入服务未闭环
- 前端: `VITE_BASE` + `createWebHistory(BASE_URL)` + `API_BASE`
- 服务端: `dashboardHandler` 始终从 `/` 与 `/assets/` 服务 embed FS，**不感知 base**
- **后果**: `VITE_BASE=/mts/` 构建后挂到默认 mts-server 会资源 404 / 路由错乱。
- **状态**: open

#### FE-ALIGN-P2-04 无共享 API 类型层
- 各页面重复定义 `User`/`AuditEvent`/`CompactionStats` 等，易与后端漂移。
- **建议**: 从 `api-spec` 生成或手写 `src/api/types.ts` 单一来源。
- **状态**: open

#### FE-ALIGN-P2-05 前端质量门禁仍偏弱
- 仅有 node:test 工具函数单测；无组件测、无 Playwright/e2e、无 eslint。
- **状态**: open

#### FE-ALIGN-P2-06 降采样运维动作缺失
- 后端有 run/reset/repair/dry-run/run-range；UI 无入口。
- **状态**: open

#### FE-ALIGN-P2-07 运维可观测缺口
- 未接 maintenance/errors、admin health checks、metrics 摘要。
- Overview 非 admin 仅 healthz（合理），但 admin 也未展示 `checks[]`。
- **状态**: open

#### FE-ALIGN-P2-08 审计仅内存环 + 过滤能力有限
- 服务端 list 来自内存 ring（limit 256），虽可 filter；持久化在 `_internal.audit_log` 但 UI 未查询存储。
- 无 action 枚举提示、无分页。
- **状态**: open

#### FE-ALIGN-P2-09 快照管理能力有限
- 仅 list/delete/create；无预览内容、下载、校验哈希、批量清理。
- **状态**: open

#### FE-ALIGN-P2-10 登录/会话产品细节
- 登录未允许配置 TTL；过期仅前端 `expires_at` 判断。
- 多标签同步有，但角色/主题/语言分 key 一致性文档不足。
- **状态**: open

#### FE-ALIGN-P2-11 图表能力初级
- 单字段折线 SVG；无多 series、图例、缩放、空值处理、导出图片。
- **状态**: open

#### FE-ALIGN-P2-12 写入 TypedBatch UI 仍窄
- 仅单 tag 列 + 单 field 列；内核 TypedBatch 可多 tags/多 fields。
- 无 string/bool 列、无从 points 预览转换。
- **状态**: open

#### FE-ALIGN-P2-13 a11y / 交互残留
- ConfirmDialog/UserModals 无完整 focus trap；部分表格无键盘操作。
- 查询历史无收藏/命名。
- **状态**: open

#### FE-ALIGN-P2-14 list databases 响应字段历史兼容
- 后端 admin list 仍返回 `measurements` 承载库名；前端兼容 `databases|measurements`（已处理）但仍是契约噪音。
- **建议**: 服务端正式返回 `databases` 字段（可双写）。
- **状态**: open

### P3 — 增强（可排期）

- 配置页可视化编辑 schema 驱动表单
- API Spec 浏览器
- 内嵌简易 metrics 面板
- 查询结果导出 CSV
- 权限预检（authz check）向导
- 响应式大屏/移动端专项

---

## 4. 前端工程短板摘要

| 维度 | 现状 | 短板 |
|---|---|---|
| 架构 | 页面 + composable | 缺 `api/types`、缺 domain 层 |
| 状态 | localStorage 多 key | 无统一 store；主题/语言/鉴权分散 |
| 样式 | Tailwind v4 | dark 不完整；设计 token 未系统化 |
| 测试 | utils node:test | 无页面契约测、无 e2e |
| 国际化 | 轻量字典 | 覆盖率低 |
| 性能 | 可接受 | 大结果表格无虚拟滚动；流式预览上限固定 200 |
| 安全 | sanitizeRedirect 有 | delete 契约错比 XSS 更致命；CSP/依赖审计无 |

---

## 5. EARS 任务清单（建议实施顺序）

### P0

#### EARS-FE-P0-01 删除契约对齐
**WHEN** 用户在查询页执行范围删除 **THE SYSTEM SHALL** 按 UI 填写的 measurement/时间范围正确删除，且请求字段与后端解码一致。
- 验收: 写入样例 → 用 UI 删除该时间点 → 再查为空；HTTP 单测覆盖 snake_case JSON。

#### EARS-FE-P0-02 降采样启停对齐
**WHEN** 用户点击暂停/恢复策略 **THE SYSTEM SHALL** 调用后端 `disable`/`enable` 并刷新状态。
- 验收: 策略 enabled 字段切换成功；错误路径有 notify。

### P1

#### EARS-FE-P1-01 非 admin 元数据可用
**WHEN** 普通用户进入查询/写入 **THE SYSTEM SHALL** 不强制依赖 admin databases API；失败时允许手填 database/RP 并提示。
- 验收: 仅 data 权限用户可完成一次查询/写入。

#### EARS-FE-P1-02 查询 Builder 扩展（tags + order/offset）
**WHEN** 用户构造查询 **THE SYSTEM SHALL** 支持 tag 精确过滤与 order/offset（与 `Query` JSON 对齐）。
- 验收: 带 tags 查询结果仅匹配 series。

#### EARS-FE-P1-03 FieldValue 展示规范化
**WHEN** 渲染行结果 **THE SYSTEM SHALL** 将 FieldValue 展开为可读标量（可切换 raw）。
- 验收: 表格显示 `0.7` 而非 `{"type":1,"float64":0.7}`。

#### EARS-FE-P1-04 列式结果可视化/表格
**WHEN** 查询模式为 columns **THE SYSTEM SHALL** 提供基础表格或转为图表数据。
- 验收: columns 模式不只显示整段 JSON。

### P2

#### EARS-FE-P2-01 暗色主题全站
**WHEN** 启用 dark **THE SYSTEM SHALL** 主要页面背景/边框/文本对比度可用。
- 验收: 抽查 10 页面无白底刺眼块。

#### EARS-FE-P2-02 i18n 关键路径覆盖
**WHEN** 切换 en **THE SYSTEM SHALL** 覆盖登录/查询/写入/侧栏/错误通知关键文案。
- 验收: en 下上述页面无大段中文硬编码（专有名词除外）。

#### EARS-FE-P2-03 嵌入子路径闭环
**WHEN** 以 `VITE_BASE=/mts/` 构建并配置服务端 base **THE SYSTEM SHALL** 正确加载 JS/CSS 与路由。
- 验收: 子路径打开 dashboard 无 404 asset。

#### EARS-FE-P2-04 共享 API 类型
**THE SYSTEM SHALL** 将高频 DTO 收敛到 `src/api/types.ts`（或生成物），页面停止复制粘贴。
- 验收: User/QueryStats/DownsamplePolicy 等单点定义。

#### EARS-FE-P2-05 降采样高级动作
**WHEN** admin 运维降采样 **THE SYSTEM SHALL** 提供 run/reset/dry-run（至少）入口。
- 验收: dry-run 返回窗口估算可展示。

#### EARS-FE-P2-06 运维错误与 health checks
**WHEN** admin 查看概览/运维 **THE SYSTEM SHALL** 展示 maintenance errors 与 health checks。
- 验收: 注入失败后 errors 列表可见。

#### EARS-FE-P2-07 前端契约测试
**THE SYSTEM SHALL** 为 delete/downsample action/meta 降级增加单测或 e2e。
- 验收: CI 可跑；覆盖 P0 回归。

#### EARS-FE-P2-08 databases 响应字段正规化
**WHEN** list databases **THE SYSTEM SHALL** 返回 `databases`（可兼容旧字段）。
- 验收: 契约测试断言 `databases`。

### P3

#### EARS-FE-P3-01 聚合窗口查询 UI
#### EARS-FE-P3-02 结果导出 CSV
#### EARS-FE-P3-03 TypedBatch 多列编辑器
#### EARS-FE-P3-04 图表多 series/缩放
#### EARS-FE-P3-05 API Spec 浏览器
#### EARS-FE-P3-06 审计查持久化 `_internal`

---

## 6. 状态跟踪

| ID | 优先级 | 状态 |
|---|---|---|
| FE-ALIGN-P0-01 | P0 | open |
| FE-ALIGN-P0-02 | P0 | open |
| FE-ALIGN-P1-01 ~ 04 | P1 | open |
| FE-ALIGN-P2-01 ~ 14 | P2 | open |
| FE-ALIGN-P3-* | P3 | deferred |

---

## 7. 推荐下一轮实施顺序

1. **立刻修 P0**（删除契约 + 降采样 enable/disable）——否则核心写删/运维动作不可信  
2. **P1-01 非 admin 元数据**——否则 data 用户工作流断  
3. **P1-03 字段展示 + P1-02 tags**——查询可用性明显提升  
4. **P2 主题/i18n/类型层/契约测**——工程化  
5. 其余增强按产品优先级

---

## 8. 验证建议（修复后）

- 手工: admin + 普通用户各走通 写→查→删→再查  
- 手工: 降采样创建→disable→enable  
- `npm run test && npm run build`  
- `make test && make e2e && make lint`  
- 契约单测: delete snake_case；downsample action 名

## 处理状态（实现后更新）

| ID | 状态 | 备注 |
|----|------|------|
| P0-01 Delete 契约 | 已修复 | `delete.go` JSON tag + 契约测试 |
| P0-02 降采样 enable/disable | 已修复 | DownsamplePage action + 文案 |
| P1-01 非 admin 元数据降级 | 已修复 | listDatabasesDetailed + Write/Query 手填 |
| P1-02 tags/order/offset | 已修复 | useQueryWorkbench buildQuery |
| P1-03 FieldValue 展示 | 已修复 | utils/fieldValue.ts |
| P1-04 列结果表 | 已修复 | QueryPage 列摘要表 |
| P2-01 暗色全站 | 已修复 | 页面/组件 dark + mts-* 样式 |
| P2-02 i18n 关键路径 | 已修复 | messages 扩展 + Login/Overview |
| P2-03 子路径嵌入 | 已修复 | `http.dashboard_base` + SPA handler |
| P2-04 共享 API 类型 | 已修复 | `src/api/types.ts` |
| P2-05 downsample 高级动作 | 已修复 | run/reset/dry-run |
| P2-06 maintenance/health | 已修复 | Overview/Operations |
| P2-07 前端契约测试 | 已修复 | dashboardAlign.contract.test.ts |
| P2-08 databases 字段 | 已修复 | measurementsResponse 双字段 |
| P3-01 聚合窗口 UI | 已修复 | aggregates/window/group_tags |
| P3-02 CSV 导出 | 已修复 | rowsToCSV + Query/Audit 导出 |
| P3-03 TypedBatch 多列 | 已修复 | 多 tag/field 列编辑器 |
| P3-04 多 series 图 | 已修复 | extractMultiSeries + QueryChart |
| P3-05 API Spec 浏览器 | 已修复 | ApiSpecPage |
| P3-06 审计持久化读回 | 已修复 | 合并 `_internal.audit_log` |

验证：dashboard test/build、make test/e2e/lint 通过（2026-07-19 第三轮）。

### P4 体验增强（2026-07-19）
| ID | 状态 | 备注 |
|----|------|------|
| P4-01 权限预检 | 已修复 | authz 自检 + Query/Write 按钮 |
| P4-02 配置 Schema | 已修复 | ConfigPage 可过滤浏览 |
| P4-03 虚拟滚动 | 已修复 | VirtualTable 行结果 |


## P5 状态（2026-07-19）
- 查询历史命名/收藏：已完成
- Focus trap：已完成（ConfirmDialog / UserModals）
- 响应式加固：已完成（Query 表单栅格 + TopBar）


## P6 状态（2026-07-19）
- 查询快捷键：已完成
- 历史导入/导出：已完成
- 查询偏好持久化：已完成


## P7 状态（2026-07-19）
- ErrorBoundary：已完成
- EmptyState：查询空结果/历史空列表
- 结果列可见性记忆：已完成


## P8 状态（2026-07-19）
- 表单脏状态：Query/Write
- 查询耗时水线：已完成
- Write/Audit EmptyState：已完成


## P9 状态（2026-07-19）
- ActionResultBanner：已完成
- Operations/Downsample 空状态与结果条：已完成


## P10 状态（2026-07-19）
- Storage/Users/Config/Overview/Databases 结果条与空状态对齐


## P11 状态（2026-07-19）
- 全局 loading / skeleton / 会话提示：已完成


## P12 状态（2026-07-19）
- Databases 空态、安全响应头、冒烟契约：已完成


## P13 状态（2026-07-19）
- 服务侧可商用冒烟 `TestCommercialDashboardSmoke`：已完成
- 生产清单 `productionChecklist`：已完成
- NotFound / PermissionDenied EmptyState 收口：已完成
- 文档：`docs/plans/dashboard-ux-p13-2026-07-19.md`
- **仍不宣称可商用目标完成**：Playwright UI e2e、生产 HTTPS/HSTS/runbook、RBAC 矩阵可视化


## P14 状态（2026-07-19）
- 生产 Runbook：`docs/ops/dashboard-production-runbook.md`
- RBAC 能力矩阵页 `/access` + `rbacMatrix.ts`：已完成
- **仍不宣称可商用目标完成**：Playwright UI e2e、服务端强制改密、实时 grants 可视化


## P15 状态（2026-07-19）
- 强制改密：`must_change_password` metadata + API 门禁 + 前端强制改密页
- **仍不宣称可商用目标完成**：Playwright UI e2e、实时 grants 拉取可视化


## P16 状态（2026-07-20）
- 实时 grants：`/access/grants`
- 指标浏览：`/observability/metrics` + prometheus 解析
- **仍不宣称可商用目标完成**：Playwright UI e2e、边缘 HTTPS 落地


## P17 状态（2026-07-20）
- Playwright 浏览器商业冒烟：`npm run test:e2e` 通过
- 指标页路由修正为 `/observability/metrics`（避免与 Prometheus `/metrics` 冲突）
- **仍不宣称可商用目标完成**：边缘 HTTPS/HSTS 部署落地、CI 浏览器安装约定


## P18 状态（2026-07-20）
- Storage 备份演练清单：已完成
- Makefile dashboard-test / dashboard-test-e2e：已完成
- **仍不宣称可商用目标完成**：边缘 HTTPS/HSTS 部署落地、完整旁路恢复自动化


## P19 状态（2026-07-20）
- data_dir 旁路恢复自动化：`TestDataDirSidePathRestoreDrill`
- TLS 启用时 HSTS + doctor 边缘 HTTPS 提示
- **仍不宣称可商用目标完成**：生产边缘证书/HSTS 人工验收、跨主机备份编排


## P20 状态（2026-07-20）
- `GET /api/v1/admin/doctor` 结构化检查 + Overview 展示
- 边缘 HTTPS/HSTS 人工验收清单（Storage + edgeHttpsAcceptance）
- **仍不宣称可商用目标完成**：生产边缘证书人工验收执行、跨主机备份编排、UI 一键旁路恢复


## P21 状态（2026-07-20）
- `POST /api/v1/admin/storage/data-snapshot` / `restore-drill` / `GET data-snapshots`
- Storage 页一键旁路恢复编排 + 演练清单联动
- **仍不宣称可商用目标完成**：生产边缘证书人工验收、跨主机定时备份编排


## P22 状态（2026-07-20）
- `/ops/readiness` 可商用就绪中心：生产清单 + HTTPS 验收 + 备份编排 + doctor
- 勾选状态 localStorage 持久化，Storage 边缘 HTTPS 清单共用
- Playwright 冒烟覆盖 storage/readiness
- **仍不宣称可商用目标完成**：真实边缘证书验收执行、跨主机定时备份在生产环境的实际部署


## P23 状态（2026-07-20）
- `scripts/mts-backup.sh`：data-snapshot / rsync / restore-drill / 保留清理
- `make backup-script-check` 语法与 dry-run 自检
- 就绪中心快捷动作跳转 Storage 锚点 + 脚本提示
- **仍不宣称可商用目标完成**：真实边缘证书验收执行、目标环境 cron/systemd 安装与演练归档


## P24 状态（2026-07-20）
- 就绪评分纳入 doctor warn / TLS / 加载失败（`readinessScore.ts`）
- Overview 管理员入口跳转 `/ops/readiness`
- CI `scripts/ci_gate.sh` 纳入 `mts-backup-selfcheck`
- 备份文档补充 login 取 Token 示例
- **仍不宣称可商用目标完成**：真实边缘证书验收执行、目标环境 cron/systemd 安装与演练归档


## P25 状态（2026-07-20）
- 就绪中心导出/导入/演练归档
- About + `GET /api/v1/admin/version`
- Playwright 加深：勾选持久化、data-snapshot、About
- 状态：已实现；**仍不宣称可商用目标完成**


## P26 状态（2026-07-20）
- 会话治理：常显徽章 + 预警 toast + 到期自动登出
- 账户改密页 `/account`；强制改密共享 passwordPolicy
- **仍不宣称可商用目标完成**


## P27 状态（2026-07-20）
- 通知中心容量/去重/warn
- 错误码友好映射
- Overview 摘要条（会话+版本）
- **仍不宣称可商用目标完成**


## P28 状态（2026-07-20）
- 全局命令面板 + 审计页筛选/导出体验
- **仍不宣称可商用目标完成**


## P29 状态（2026-07-20）
- a11y 跳过链接 + 运维操作历史导出
- **仍不宣称可商用目标完成**


## P30 状态（2026-07-20）
- 路由脏态离开确认；用户/数据库筛选空态
- **仍不宣称可商用目标完成**

## P31（2026-07-20）
- 验收导出包：`acceptancePack.ts` + 就绪中心「导出验收包」
- 降采样：`filterDownsamplePolicies` + 筛选栏/空态/批量 enable|disable
- 状态：已实现；仍不宣称可商用完成（部署侧边缘证书/cron/异地备份）

## P32（2026-07-20）
- Overview 本地就绪评分卡片 + 跳转就绪中心
- Playwright 商业冒烟加深：验收包、降采样筛选、Overview 评分
- 状态：已实现；仍不宣称可商用完成

## P33（2026-07-20）
- 降采样 repair 区间修复 UI（确认对话框 + start/end unix）
- 降采样页文案 i18n 收口
- 状态：已实现；仍不宣称可商用完成

## P34（2026-07-20）
- 降采样 run-range/repair/dry-run 统一区间对话框
- statuses 明细；downsampleRange 纯函数与契约测试
- 状态：已实现；仍不宣称可商用完成

## P35（2026-07-20）
- 路由文档标题（pageTitle + useDocumentTitle）
- 降采样对话框 focus trap
- 脏表单离开确认 i18n
- 状态：已实现；仍不宣称可商用完成
