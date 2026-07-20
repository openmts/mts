# Dashboard / 命令面板页内动作 EARS（2026-07-20 P102）

## 范围
- 命令面板支持页内动作：主题、语言、密度、侧栏过滤聚焦、通知历史、快捷键帮助、侧栏折叠
- 动作与导航统一搜索；动作项标注「动作」徽章
- Layout provide 注入 focusSidebarFilter / toggleSidebarCollapse
- 商业冒烟覆盖 action-toggle-theme

## 边界
- 动作不执行危险运维写操作；仅 UI/偏好切换
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P102-01 WHEN 用户搜索 theme/语言/密度等关键词 THE SYSTEM SHALL 展示对应页内动作
- [x] EARS-FE-P102-02 WHEN 用户执行页内动作 THE SYSTEM SHALL 立即生效且不强制跳转路由
- [x] EARS-FE-P102-03 WHEN 执行主题切换 THE SYSTEM SHALL 切换 html.dark 并 toast 反馈
- [x] EARS-FE-P102-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 command-item-action-toggle-theme
- [x] EARS-DOC-P102-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P102

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
