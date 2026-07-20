# Dashboard 全量检视 EARS 任务清单（2026-07-21）

来源：`docs/review/code-review-2026-07-21-0002-mts-dashboard-full.md`  
基线：`docs/plans/dashboard-commercial-baseline-2026-07-19.md`

## 边界
- 不宣称可商用目标完成
- 部署侧边缘证书 / cron / 异地备份告警 **不计入** readiness 评分
- 单机 POC；不引入 SQL parser

## 本会话已完成（P133–P136）
- [x] P133 Query 结果/列摘要 VT
- [x] P134 Query 历史 VT + 筛选 + 上限 200
- [x] P135 命令面板结果密度
- [x] P136 Readiness 签核引导

## P137 — 剩余列表虚拟化与空态
- [x] EARS-FE-P137-01 WHEN ApiSpec 命名空间端点非空 THE SYSTEM SHALL 虚拟渲染可视端点行
- [x] EARS-FE-P137-02 WHEN Overview doctor/health/maintenance 列表非空 THE SYSTEM SHALL 使用虚拟滚动或等价上限策略并暴露 testid
- [x] EARS-FE-P137-03 WHEN Readiness doctor checks 非空 THE SYSTEM SHALL 虚拟渲染 checks
- [x] EARS-FE-P137-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖新增 virtual-list testid（有数据时）
- [x] EARS-DOC-P137-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P137

## P138 — 写入与元数据边界
- [x] EARS-FE-P138-01 WHEN 表单写行数达到上限 THE SYSTEM SHALL 阻止继续新增并提示 Typed/Line
- [x] EARS-FE-P138-02 WHEN 非 admin 无法拉取 RP THE SYSTEM SHALL 保持手填可用并展示明确提示
- [x] EARS-DOC-P138-03 WHEN 更新基线 THE SYSTEM SHALL 记录 P138

## P139 — 查询高级语义
- [x] EARS-FE-P139-01 predicates DSL → Query.predicates
- [x] EARS-FE-P139-02 非法谓词可读错误
- [x] EARS-FE-P139-03 查询页暴露 query-predicates
- [x] EARS-DOC-P139-04 基线记录（expr 树 UI 仍非目标）

## 部署侧（open，不计分）
- [ ] OPS-EDGE-HTTPS 生产证书/HSTS 人工验收
- [ ] OPS-CRON-SYSTEMD 目标环境实装
- [ ] OPS-OFFSITE-ALERT 跨主机备份+告警演练

## P140 — Data 面只读 RP（服务端 + 前端）
- [x] EARS-BE-P140-01 data 面 GET retention-policies（read 权限）
- [x] EARS-FE-P140-03 listRetentionPolicies 优先 data 回退 admin
- [x] EARS-DOC-P140-04 基线记录

## P141 — 子路径部署对齐
- [x] EARS-DOC-P141-01 VITE_BASE 与 dashboard_base 文档对齐
- [x] 服务端/Vite 已支持；默认 `/`

## 部署侧（open，不计分）
- [ ] OPS-EDGE-HTTPS 生产证书/HSTS 人工验收
- [ ] OPS-CRON-SYSTEMD 目标环境实装
- [ ] OPS-OFFSITE-ALERT 跨主机备份+告警演练

## P142 — Query series/fields meta
- [x] EARS-FE-P142-01 选定 measurement 加载 fields/series
- [x] EARS-FE-P142-02 series 超限截断提示
- [x] EARS-FE-P142-03 选择 series 填充 tags
- [x] EARS-FE-P142-04 商业冒烟 series meta testid
- [x] EARS-DOC-P142-05 基线记录

## P143 — Query engine stats
- [x] EARS-FE-P143-01 GET query/stats 入口
- [x] EARS-FE-P143-02 来源区分 query/engine
- [x] EARS-FE-P143-03 空态引导
- [x] EARS-FE-P143-04 商业冒烟
- [x] EARS-DOC-P143-05 基线记录

## P144 — Account landing EmptyState
- [x] EARS-FE-P144-01 列表+筛选
- [x] EARS-FE-P144-02 无匹配 EmptyState
- [x] EARS-FE-P144-03 列表选择保存
- [x] EARS-FE-P144-04 商业冒烟
- [x] EARS-DOC-P144-05 基线记录

## P145 — Data 面库列表
- [x] EARS-BE-P145-01 data 用户仅见可读库
- [x] EARS-BE-P145-02 admin 路径仍拒绝非 admin
- [x] EARS-FE-P145-03 listDatabases 优先 data
- [x] EARS-DOC-P145-04 基线记录

## P146 — Query series 筛选
- [x] EARS-FE-P146-01 series 下拉客户端筛选
- [x] EARS-FE-P146-02 无匹配提示
- [x] EARS-FE-P146-03 商业冒烟 filter testid
- [x] EARS-DOC-P146-04 基线记录

## P147 — Write measurement/fields meta
- [x] EARS-FE-P147-01 选定库加载 measurement
- [x] EARS-FE-P147-02 measurement 加载 field
- [x] EARS-FE-P147-03 芯片填充 form/Typed
- [x] EARS-FE-P147-04 商业冒烟 write-meta-panel
- [x] EARS-DOC-P147-05 基线记录

## P148 — Downsample create meta
- [x] EARS-FE-P148-01 打开创建面板加载库建议
- [x] EARS-FE-P148-02 source db/measurement 驱动 measurement/field 建议
- [x] EARS-FE-P148-03 商业冒烟 create meta
- [x] EARS-DOC-P148-04 基线记录

## P149 — Query 范围删除确认修复
- [x] EARS-FE-P149-01 DELETE 输入可提交
- [x] EARS-FE-P149-02 确认框范围摘要
- [x] EARS-FE-P149-03 无时间警告
- [x] EARS-FE-P149-04 商业冒烟
- [x] EARS-DOC-P149-05 基线记录

## P150 — UserGrantPanel 体验
- [x] EARS-FE-P150-01 库筛选
- [x] EARS-FE-P150-02 空态
- [x] EARS-FE-P150-03 授权按钮 disabled
- [x] EARS-DOC-P150-04 基线记录

## P151 — 非 admin 只读库浏览器
- [x] EARS-FE-P151-01 非 admin data 面浏览库
- [x] EARS-FE-P151-02 隐藏写操作
- [x] EARS-FE-P151-03 admin 完整能力
- [x] EARS-FE-P151-04 商业冒烟
- [x] EARS-DOC-P151-05 基线

## P152 — Query series 服务端 tag 过滤
- [x] EARS-FE-P152-01/02/03 tags 刷新 + 非法提示
- [x] EARS-DOC-P152-04 基线

## P153 — 库页 series 截断
- [x] EARS-FE-P153-01/02 截断提示
- [x] EARS-DOC-P153-03 基线

## P154 — Users 授权 e2e
- [x] EARS-FE-P154-01/02 testid + 冒烟
- [x] EARS-DOC-P154-03 基线

## P155 — series limit/total API
- [x] EARS-BE-P155-01/02 reserved qs + total
- [x] EARS-FE-P155-03 listSeriesDetailed limit
- [x] EARS-DOC-P155-04 基线

## P156 — 导航开放 databases
- [x] EARS-FE-P156 侧栏/命令面板/路由

## P157 — 落地页与能力矩阵
- [x] EARS-FE-P157 落地/RBAC/冒烟路径

## P158 — measurement 筛选
- [x] EARS-FE-P158 筛选/计数/空态

## P159 — 库页深链
- [x] EARS-FE-P159 Query/Write 预填

## P160 — 非 admin e2e
- [x] EARS-FE-P160 reader 冒烟

## P161 — 矩阵 browse
- [x] EARS-FE-P161 只读浏览行
