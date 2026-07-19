# Dashboard 体验增强 EARS 清单（2026-07-19 P14）

## 范围（可商用：生产 Runbook + RBAC 能力矩阵可视化）
- `docs/ops/dashboard-production-runbook.md` 部署/上线/应急
- 前端 RBAC 能力矩阵纯数据 + 单测
- Dashboard「权限矩阵」页 + 导航入口
- 更新可商用基线与 review 状态

## EARS
- [x] EARS-DOC-P14-01 WHEN 运维准备上线 THE SYSTEM SHALL 提供生产 Runbook（拓扑、清单、代理、日常、应急）
- [x] EARS-FE-P14-02 WHEN 导出权限能力矩阵 THE SYSTEM SHALL 以纯数据描述 admin/user 对各控制台能力的访问级别
- [x] EARS-FE-P14-03 WHEN 用户打开权限矩阵页 THE SYSTEM SHALL 展示可过滤的角色×能力表格与分布摘要
- [x] EARS-FE-P14-04 WHEN 导航渲染 THE SYSTEM SHALL 对所有已登录角色暴露「权限矩阵」入口（非 adminOnly）
- [x] EARS-FE-P14-05 WHEN 单测校验矩阵 THE SYSTEM SHALL 覆盖核心商业面与 admin/user 差异

## 实现备注
- `docs/ops/dashboard-production-runbook.md`
- `cmd/mts-dashboard/src/utils/rbacMatrix.ts` (+ test)
- `cmd/mts-dashboard/src/pages/AccessMatrixPage.vue`
- router `/access`、SidebarNav、i18n `accessMatrix`

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 浏览器级 Playwright e2e
- 生产强制改密策略（服务端强制）
- 更细粒度 per-database UI 权限可视化（实时拉取 grants）
