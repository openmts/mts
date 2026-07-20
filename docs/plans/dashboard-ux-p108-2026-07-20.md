# Dashboard / 命令面板空查询折叠长导航 EARS（2026-07-20 P108）

## 范围
- 空查询时导航组默认只展示主路由（无 hash 深链），减少长列表噪音
- 提供「展开更多导航」以显示深链项；有查询关键字时不做折叠
- 分组标题展示计数；保留动作组完整列表
- 键盘选中索引仅覆盖当前可见项

## 边界
- 不删除既有深链 catalog（P99–P101）
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P108-01 WHEN 命令面板空查询打开 THE SYSTEM SHALL 默认隐藏导航深链项
- [x] EARS-FE-P108-02 WHEN 用户展开更多导航 THE SYSTEM SHALL 显示全部导航项
- [x] EARS-FE-P108-03 WHEN 用户输入查询关键字 THE SYSTEM SHALL 不对过滤结果做空查询折叠
- [x] EARS-FE-P108-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖默认折叠与展开
- [x] EARS-DOC-P108-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P108

## 实现备注
- `isCommandDeepLink` / `collapseNavItemsForEmptyQuery` / `applyEmptyQueryNavCollapse` 纯函数
- `CommandPalette`：`navExpanded` 状态，打开面板或查询变化时复位
- testid：`command-palette-nav-expand`、`command-palette-group-nav-count`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `go test ./...`
