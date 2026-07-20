# Dashboard / 命令面板运维深链 EARS（2026-07-20 P99）

## 范围
- 命令面板新增运维动作深链：Flush / Compact / Retention / 动作日志 / 维护错误
- Operations 页支持 hash 锚点滚动（与 Storage/Readiness 一致）
- 商业冒烟覆盖 flush / action-log 深链

## 边界
- 深链仅定位与聚焦，不自动执行危险运维动作
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P99-01 WHEN 管理员在命令面板搜索运维关键词 THE SYSTEM SHALL 展示对应深链项
- [x] EARS-FE-P99-02 WHEN 选择运维深链 THE SYSTEM SHALL 导航至 `/operations#...` 并显示目标区块
- [x] EARS-FE-P99-03 WHEN 非管理员打开命令面板 THE SYSTEM SHALL 隐藏运维深链
- [x] EARS-FE-P99-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 flush 与 action-log 深链
- [x] EARS-DOC-P99-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P99

## 实现备注
- `COMMAND_NAV_ITEMS` 增加 operations-* 条目
- `OperationsPage` 挂载 `scheduleScrollToHash` + section id

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
