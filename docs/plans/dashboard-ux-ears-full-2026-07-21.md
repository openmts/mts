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
