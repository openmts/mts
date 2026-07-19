# Dashboard 体验增强 EARS 清单（2026-07-19 P12）

## 范围（可商用：空态 + 安全基线 + 冒烟契约）
- Databases 空库 EmptyState 与页头
- HTTP 安全响应头增强（CSP / Referrer / Permissions-Policy / COOP）
- 前端安全头契约与核心路径冒烟清单
- index.html referrer / color-scheme

## EARS
- [x] EARS-FE-P12-01 WHEN 数据库列表为空 THE SYSTEM SHALL 展示 EmptyState 并提供创建入口
- [x] EARS-FE-P12-02 WHEN 展开数据库且无 measurement THE SYSTEM SHALL 使用统一 EmptyState 文案
- [x] EARS-BE-P12-03 WHEN HTTP 响应写出 THE SYSTEM SHALL 附带可商用默认安全头（nosniff / DENY / CSP / no-referrer 等）
- [x] EARS-FE-P12-04 WHEN 校验安全头契约 THE SYSTEM SHALL 提供与后端一致的期望常量与单测
- [x] EARS-FE-P12-05 WHEN 审查可商用冒烟范围 THE SYSTEM SHALL 覆盖 login/overview/query/write/databases/operations

## 实现备注
- `cmd/mts-server/middleware.go`：`applySecurityHeaders`
- `cmd/mts-server/middleware_security_test.go`
- `cmd/mts-dashboard/src/utils/securityHeaders.ts`
- DatabasesPage 空态

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `go test ./cmd/mts-server -run Security -count=1`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 浏览器级 Playwright e2e 自动跑通
- 生产部署 runbook / HTTPS 终止与 Cookie 策略说明
- 更细粒度 RBAC 页面能力矩阵可视化
