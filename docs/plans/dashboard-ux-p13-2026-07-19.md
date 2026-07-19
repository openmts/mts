# Dashboard 体验增强 EARS 清单（2026-07-19 P13）

## 范围（可商用：服务侧冒烟 + 生产清单 + 404/权限收口）
- `TestCommercialDashboardSmoke`：SPA + 安全头 + 登录 + 写/查 + 管理库列表 + flush
- 前端 `productionChecklist` 纯函数与单测
- NotFound / PermissionDenied 与 EmptyState 对齐
- 更新可商用基线文档

## EARS
- [x] EARS-BE-P13-01 WHEN 启动嵌入 Dashboard 的 mts-server THE SYSTEM SHALL 对 `/`、`/query`、`/write`、`/operations` 返回 SPA 且 HTTP 200
- [x] EARS-BE-P13-02 WHEN 冒烟请求核心 HTTP 路径 THE SYSTEM SHALL 校验可商用默认安全头与 `X-Request-ID`
- [x] EARS-BE-P13-03 WHEN bootstrap 管理员登录 THE SYSTEM SHALL 完成 write(sync) + query/rows + admin databases + flush 闭环
- [x] EARS-FE-P13-04 WHEN 导出生产上线清单 THE SYSTEM SHALL 提供 required/recommended 分级与 automated 覆盖率
- [x] EARS-FE-P13-05 WHEN 访问未知路由 THE SYSTEM SHALL 展示 EmptyState 风格 404 并提供返回概览/上一页
- [x] EARS-FE-P13-06 WHEN 非管理员进入管理页 THE SYSTEM SHALL 展示 EmptyState 风格权限拒绝说明

## 实现备注
- `cmd/mts-server/dashboard_commercial_smoke_test.go`
- `cmd/mts-dashboard/src/utils/productionChecklist.ts` (+ test)
- `cmd/mts-dashboard/src/pages/NotFoundPage.vue`
- `cmd/mts-dashboard/src/components/PermissionDenied.vue`
- `docs/plans/dashboard-commercial-baseline-2026-07-19.md`

## 验证
- `go test ./cmd/mts-server -run 'TestCommercialDashboardSmoke|Security' -count=1`
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 浏览器级 Playwright e2e（真实 UI）
- 生产 HTTPS/HSTS/runbook 落地与改密强制策略
- RBAC 能力矩阵可视化
