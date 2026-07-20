# Dashboard / 侧栏分组与界面密度 EARS（2026-07-20 P90）

## 范围
- 侧栏导航固定分组：工作区 / 访问控制 / 运维管理 / 系统
- 界面密度：舒适 / 紧凑（`html[data-density]` + localStorage）
- 账户页设置入口；商业冒烟覆盖

## 边界
- 分组不影响权限过滤与 `/` 聚焦过滤
- 折叠态隐藏分组标题，仅保留图标项
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P90-01 WHEN 侧栏展开 THE SYSTEM SHALL 按固定分组展示导航并显示分组标题
- [x] EARS-FE-P90-02 WHEN 侧栏折叠 THE SYSTEM SHALL 隐藏分组标题仍保留导航入口
- [x] EARS-FE-P90-03 WHEN 用户在账户页切换界面密度 THE SYSTEM SHALL 本机记忆并应用到文档根
- [x] EARS-FE-P90-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 `sidebar-section-*` 与 `account-density-select`
- [x] EARS-DOC-P90-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P90

## 实现备注
- `navSections` / `densityPrefs` 纯函数 + 单测；`useDensity` 启动应用
- testid：`sidebar-section-{id}`、`sidebar-section-label-{id}`、`account-density-select`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
