# Dashboard 体验增强 EARS 清单（2026-07-19 P16）

## 范围（可商用：实时 grants + 指标浏览）
- `/access/grants`：聚合全部用户库级授权
- `/metrics`：Prometheus 文本只读浏览与过滤
- 纯函数解析/汇总 + 单测
- 更新生产清单与可商用基线

## EARS
- [x] EARS-FE-P16-01 WHEN 管理员打开实时授权页 THE SYSTEM SHALL 拉取用户列表及各自 database-permissions 并表格展示
- [x] EARS-FE-P16-02 WHEN 过滤用户/库/权限/关键字 THE SYSTEM SHALL 更新覆盖统计与可见行
- [x] EARS-FE-P16-03 WHEN 管理员打开指标页 THE SYSTEM SHALL 读取后端 `/metrics` 并解析为指标族/样本
- [x] EARS-FE-P16-04 WHEN 过滤指标关键字 THE SYSTEM SHALL 按名称/help/标签缩小结果
- [x] EARS-FE-P16-05 WHEN 非管理员访问上述页面 THE SYSTEM SHALL 展示 PermissionDenied

## 实现备注
- `utils/grantsSummary.ts` / `utils/prometheus.ts`
- `pages/AccessGrantsPage.vue` / `pages/MetricsPage.vue`（路由 `/observability/metrics`，避免与 Prometheus `/metrics` 冲突）
- `apiGetText` 支持非 JSON 文本
- 导航：实时授权、指标

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 浏览器级 Playwright e2e
- 边缘 HTTPS/HSTS 落地（部署侧）
