# Dashboard / 命令面板更多页内安全动作 EARS（2026-07-20 P109）

## 范围
- 新增页内动作：复制当前页 URL、聚焦主内容、刷新当前页
- 仅安全只读/导航侧动作，不自动执行 Flush/Compact/删除等危险写操作
- 商业冒烟覆盖复制 URL 动作可达

## 边界
- 不自动执行运维写操作
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P109-01 WHEN 用户执行复制当前页 URL 动作 THE SYSTEM SHALL 将当前 href 写入剪贴板并提示
- [x] EARS-FE-P109-02 WHEN 用户执行聚焦主内容动作 THE SYSTEM SHALL 聚焦 `#main-content`
- [x] EARS-FE-P109-03 WHEN 用户执行刷新当前页动作 THE SYSTEM SHALL 重新加载当前路由
- [x] EARS-FE-P109-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖新动作入口可见
- [x] EARS-DOC-P109-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P109

## 实现备注
- `CommandActionId` 扩展 + `COMMAND_ACTION_ITEMS`
- `CommandPalette.runAction` 处理；复制用 `copyText`
- testid：`command-item-action-copy-page-url` 等

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `go test ./...`
