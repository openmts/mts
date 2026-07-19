# Dashboard 检视修复 — EARS 任务清单

- **来源**: `docs/review/code-review-2026-07-19-1801-mts-dashboard-audit.md`
- **日期**: 2026-07-19
- **约束**: 本轮闭环 P0–P2；无遗留、无弱实现。P3 增强项本轮不做（仅登记）。

## 验收总则

- `cd cmd/mts-dashboard && npm run build` 通过
- 嵌入产物权限：目录 0700、文件 0600
- `make test` / `make e2e` / `make lint` 通过
- 检视报告问题状态更新为 fixed（P3 保持 open/deferred）

---

## P0 鉴权与查询正确性

### EARS-P0-01 登出跳转登录
**WHEN** 用户点击退出 **THE SYSTEM SHALL** 清除会话并 `router.replace` 到登录页，且受保护页面不可继续交互。
- [x] 实现
- **验收**: 登出后 URL 为 `/login`，本地无 token

### EARS-P0-02 403 不踢登录
**WHEN** API 返回 403 **THE SYSTEM SHALL** 保留登录态并展示权限错误；**WHEN** 返回 401/`unauthenticated` **THE SYSTEM SHALL** 清会话并跳转登录。
- [x] 实现
- **验收**: 403 不 `clearAuth`；401 清会话

### EARS-P0-03 EXPLAIN 正确展示
**WHEN** 用户选择 EXPLAIN 查询 **THE SYSTEM SHALL** 展示 `explain` 与 `stats`（及 columns JSON），**不得**将 `ColumnSeries[]` 当作行表渲染。
- [x] 实现
- **验收**: EXPLAIN 无行表误绑

### EARS-P0-04 首访可登录
**WHEN** 密码认证未禁用且尚无管理员 **THE SYSTEM SHALL** 启动时 bootstrap 默认 `admin/admin`；登录页 **SHALL** 提示默认账号策略。
- [x] 服务端 bootstrap 调整
- [x] 登录页文案
- **验收**: 空数据目录启动后可用 admin/admin 登录

---

## P1 会话 / 流式 / 权限 / 危险操作

### EARS-P1-01 Token 过期预检
**WHERE** 存在 `expires_at` **THE SYSTEM SHALL** 在路由守卫与请求前判定过期；过期 **SHALL** 清会话并要求重新登录。
- [x] 实现
- **验收**: 伪造过期 token 无法进入壳子

### EARS-P1-02 真流式 NDJSON
**WHEN** 使用流式查询 **THE SYSTEM SHALL** 通过 `ReadableStream` 逐行解析并增量更新统计/预览，支持 Abort。
- [x] 实现 `apiPostNDJSONStream` 或等价
- **验收**: 不整包 `response.text()` 作为唯一路径

### EARS-P1-03 查询 stats 无全局竞态
**WHEN** 查询完成 **THE SYSTEM SHALL** 优先使用响应内/流 end 的 stats；**不得**无条件覆盖为全局 `/query/stats` 快照（可删除二次拉取）。
- [x] 实现
- **验收**: 行/列/流/explain 均有稳定 stats 来源

### EARS-P1-04 取消与请求代数
**WHEN** 发起新查询或取消 **THE SYSTEM SHALL** 使用 requestId/AbortSignal 忽略过期响应，**不得**仅依赖错误文案字符串。
- [x] 实现
- **验收**: 快速连点/取消无错乱结果

### EARS-P1-05 时间戳精度
**WHERE** 涉及纳秒时间戳 **THE SYSTEM SHALL** 以字符串传递与展示输入；展示时间可用 ms 精度并标注；避免 `Date.now()*1e6` 写入关键路径而不校验。
- [x] Query/Write 输入与 format 修正
- **验收**: 时间字段以 string 构建 query/write 载荷

### EARS-P1-06 角色菜单裁剪
**WHEN** 当前用户非 admin **THE SYSTEM SHALL** 隐藏管理面导航（用户/配置/运维/降采样/审计/存储等），页面访问拒绝时展示权限空态而非踢登录。
- [x] Sidebar + 页面级提示
- **验收**: 普通用户侧栏仅数据相关项

### EARS-P1-07 删除操作隔离与强化确认
**WHEN** 执行范围删除 **THE SYSTEM SHALL** 使用独立确认区（须回显 database/measurement/时间范围），**不得**与普通查询按钮并列一次 `confirm` 即删。
- [x] 实现
- **验收**: 未填确认文案不可删

### EARS-P1-08 服务级 Token 配置
**WHERE** 部署使用 admin_token/data_tokens **THE SYSTEM SHALL** 允许在设置（配置页）中保存可选 Admin Token / Data Token 至 sessionStorage，并由 client 自动附加请求头。
- [x] client + Config 页
- **验收**: 可设置/清除，请求带头

---

## P2 契约 / 页面 / 工程

### EARS-P2-01 数据库列表封装
**THE SYSTEM SHALL** 提供 `listDatabases()` 等封装，类型命名不再把数据库列表叫 `measurements`（运行时仍兼容服务端字段）。
- [x] 实现
- **验收**: 页面统一调用封装

### EARS-P2-02 API 错误统一
**THE SYSTEM SHALL** 统一错误解析（含 code）、Abort、空 body；GET **不得**强制无意义 JSON Content-Type 导致问题（Authorization 仍附带）。
- [x] 实现
- **验收**: apiGet/apiPost/apiPostText/stream 错误形态一致

### EARS-P2-03 写入校验
**WHEN** 提交写入 **THE SYSTEM SHALL** 拒绝空点集、NaN/非法数字字段，并返回可读错误行信息（表单/解析）。
- [x] 实现
- **验收**: 非法 float/int 被拦截

### EARS-P2-04 数据库展开状态
**WHEN** 加载数据库详情失败 **THE SYSTEM SHALL** 显示错误并可重试，不留下“已展开但未加载”的假成功态。
- [x] 实现
- **验收**: 失败可 retry

### EARS-P2-05 用户权限错误可见
**WHEN** 批量授权/撤销失败 **THE SYSTEM SHALL** 汇总错误；`revokeAll` 带 try/catch。
- [x] 实现
- **验收**: 失败有 actionError

### EARS-P2-06 运维 loading 分项
**THE SYSTEM SHALL** 使用分项 loading 或 map，观测刷新使用 `allSettled`，单点失败不拖垮整页。
- [x] 实现
- **验收**: 刷新与操作 loading 不互斥卡死

### EARS-P2-07 配置热重载回读
**WHEN** 热重载成功 **THE SYSTEM SHALL** 重新拉取 effective config。
- [x] 实现
- **验收**: reload 后 JSON 更新

### EARS-P2-08 审计与存储增强
**THE SYSTEM SHALL** 为审计提供加载状态/空态；存储导出提供下载 JSON 文件。
- [x] 实现
- **验收**: 可下载 export

### EARS-P2-09 概览字段补齐
**THE SYSTEM SHALL** 展示 peak/RSS/rejected_writes 等关键内存字段，并支持可选自动刷新。
- [x] 实现
- **验收**: UI 可见 RSS/peak

### EARS-P2-10 统一通知
**THE SYSTEM SHALL** 提供轻量 `useNotify`（成功/错误），关键操作有成功反馈（改密、授权等）。
- [x] 实现
- **验收**: 改密成功有提示

### EARS-P2-11 路由与拆分
**THE SYSTEM SHALL** 提供 404 路由；login redirect 仅允许站内相对路径；Query 页拆分 composable 使主文件可控。
- [x] 实现
- **验收**: 未知路径进 404；恶意 redirect 无效

### EARS-P2-12 基础 a11y
**THE SYSTEM SHALL** 为模态提供 Esc 关闭与基础焦点；危险确认避免仅依赖无障碍 `confirm`（删除用自定义确认）。
- [x] 实现
- **验收**: UserModals Esc 可关

---

## P3（本轮不做）

- P3-01 查询可视化图表
- P3-02 TypedBatch 高级构造 UI
- P3-03 查询历史
- P3-04 暗色主题
- P3-05 i18n
- P3-06 子路径 base

---

## 实现备注区

- 2026-07-19：P0-P2 全部实现并验证
- 服务端：密码认证开启时 bootstrap admin；bootstrap 使用 context.WithoutCancel
- 前端：Abort/403 分流、真流式 NDJSON、EXPLAIN 正确展示、角色菜单、删除强确认、服务级 token、notify、404、ms 时间精度
- 验证：npm run build / make test / make e2e / make lint 通过
