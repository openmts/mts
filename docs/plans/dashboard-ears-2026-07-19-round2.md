# Dashboard 复审 EARS 任务清单（Round 2）

- **来源**: `docs/review/code-review-2026-07-19-1836-mts-dashboard-full.md`
- **日期**: 2026-07-19
- **范围**: 上一轮未完整项 + 全量复审新问题
- **说明**: 上轮 P0–P2 主体已 fixed；本清单只列 **仍 open / partial** 与 P3 deferred

## 验收总则

- `cd cmd/mts-dashboard && npm run build`
- 涉及服务端契约时：`make test` + `make e2e` + `make lint`
- 更新本文件勾选与 review 状态

---

## P1

### EARS-R2-P1-01 行/列查询统计可用
**WHEN** 用户执行 rows 或 columns 查询 **THE SYSTEM SHALL** 展示与该次查询对应的 stats（shards/samples/duration），且 **不得** 与其它并发查询串台。
- [x] 实现（优先服务端响应内嵌 stats；否则 requestId 绑定 `/query/stats`）
- **验收**: 行查询后统计卡片有值；并发两次查询互不覆盖

### EARS-R2-P1-02 可信角色源
**WHEN** 用户登录成功 **THE SYSTEM SHALL** 获得明确 role（登录响应或 `/users/me`），**不得** 仅凭 ListUsers 成败推断 admin。
- [x] API 契约（若需）
- [x] 前端持久化与侧栏/门禁
- **验收**: 普通用户永不误判 admin；admin 刷新后角色稳定

### EARS-R2-P1-03 非管理员概览降级
**WHEN** 当前用户非 admin **THE SYSTEM SHALL** 仅展示可访问的健康信息，管理统计失败 **不得** 导致整页错误。
- [x] Overview 分权加载 + allSettled
- **验收**: 普通用户概览无大红条；admin 仍见内存/compact/维护

### EARS-R2-P1-04 数据库管理权限边界
**WHEN** 用户非 admin **THE SYSTEM SHALL** 无法进入库管理写操作入口（隐藏或 PermissionDenied）；元数据浏览若保留须走 data 面 API。
- [x] 路由/侧栏/页面门禁
- **验收**: 普通用户不能看到“创建数据库”或看到明确无权限

---

## P2

### EARS-R2-P2-01 统一危险确认组件
**WHEN** 执行删除用户/策略/库、Flush/Compact/Retention **THE SYSTEM SHALL** 使用统一确认组件（可要求输入关键字），**不得** 仅依赖 `window.confirm`/`prompt`。
- [x] ConfirmDialog 组件
- [x] 替换现有 confirm/prompt 调用点
- **验收**: 代码中无业务路径 `confirm(`/`prompt(`

### EARS-R2-P2-02 流式预览语义正确
**WHEN** 流式查询超过预览上限 **THE SYSTEM SHALL** 明确标注“仅预览前 N 行”，复制操作语义与文案一致。
- [x] UI 文案 + 可选下载
- **验收**: 大流结果不会被误认为已完整复制

### EARS-R2-P2-03 保留策略 now 时间安全
**WHEN** 触发 apply retention **THE SYSTEM SHALL** 使用服务端默认 now 或不使用不安全 ns 乘法。
- [x] OperationsPage 调整
- **验收**: 请求体无 `Date.now()*1e6` 或经安全转换

### EARS-R2-P2-04 Line protocol 错误可见
**WHEN** 解析 LP 文本 **THE SYSTEM SHALL** 报告非法行号/字段，**不得** 静默丢弃全部坏行而无提示。
- [x] parseLineProtocol 返回 diagnostics
- [x] WritePage 展示
- **验收**: 含坏行文本有明确错误

### EARS-R2-P2-05 关键操作成功反馈
**WHEN** 创建用户、设置密码、删除策略等成功 **THE SYSTEM SHALL** 给出 success notify。
- [x] 补齐 notify 调用
- **验收**: 上述路径有成功提示

### EARS-R2-P2-06 代码卫生
**THE SYSTEM SHALL** 删除未使用 `apiPostText`（或证明有调用）；`UsersPage` 拆分至 ≤300 行。
- [x] 清理 + 拆分
- **验收**: 无死导出；Users 主文件 ≤300

### EARS-R2-P2-07 前端质量门禁
**THE SYSTEM SHALL** 至少具备：`typecheck`/`build` 脚本清晰；可选 eslint + 1 个鉴权/utils 单测。
- [x] package scripts / 最小测试
- **验收**: `npm run build` 与新增 test 可重复执行

### EARS-R2-P2-08 降采样表单可读
**WHEN** 创建降采样策略 **THE SYSTEM SHALL** 提供 interval 单位说明（或 ms/s/m 输入），关键字段有基础校验。
- [x] DownsamplePage UX
- **验收**: 用户无需手写 6e10 ns

### EARS-R2-P2-09 Modal 焦点与遮罩
**WHEN** 打开 UserModals **THE SYSTEM SHALL** 支持 Esc、遮罩点击关闭、基础初始焦点。
- [x] UserModals 增强
- **验收**: 键盘与鼠标均可关闭

### EARS-R2-P2-10 服务级 Token 生命周期说明
**WHEN** 用户登出 **THE SYSTEM SHALL** 明确是否保留 Admin/Data Token（推荐：登出可选清理或设置页一键清理已有；UI 标注 session 级）。
- [x] 产品默认：登出保留 + Config 已有清除；补充文案即可 或 登出时询问
- **验收**: 登录页/配置页无歧义说明

---

## P3（本轮默认可不做）

- [ ] EARS-R2-P3-01 查询可视化
- [ ] EARS-R2-P3-02 TypedBatch UI
- [ ] EARS-R2-P3-03 查询历史
- [ ] EARS-R2-P3-04 暗色主题
- [ ] EARS-R2-P3-05 i18n
- [ ] EARS-R2-P3-06 子路径 base
- [ ] EARS-R2-P3-07 审计增强
- [ ] EARS-R2-P3-08 快照列表管理

---

## 实现备注区

- 2026-07-19 R2 闭环完成（P3 deferred）
- 后端：`AuthToken.Role` 透传（user→runtime→engine→login JSON）；`queryRows/columns` HTTP+gRPC 响应内嵌 `stats`
- 前端：登录使用 `token.role`；移除 ListUsers 角色启发式；Overview 非 admin 仅 healthz；Databases admin 门禁
- ConfirmDialog 替换所有业务 confirm/prompt；流式预览语义；LP diagnostics；成功 notify；retention 不传不安全 now；downsample 人类 interval
- 删除 `apiPostText`；UsersPage 拆分 `UserGrantPanel`（≤300）；`npm test` + typecheck/build 门禁
- 验证：`npm run test`、`npm run build`、`make test`、`make e2e`、`make lint` 均通过

