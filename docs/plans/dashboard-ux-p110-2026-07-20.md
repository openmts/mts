# Dashboard / Users·Databases 多选批量 UX EARS（2026-07-20 P110）

## 范围
- Users / Databases 列表支持行多选、全选当前过滤结果、清空选择
- 导出 JSON/CSV 优先导出已选行；无选择时导出当前过滤全集（保持既有行为）
- Users 支持批量启用 / 禁用（确认对话框，禁止批量删除）
- Databases 仅多选导出（不提供批量删除库）

## 边界
- 不批量删除数据库或用户
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P110-01 WHEN 用户勾选过滤结果行 THE SYSTEM SHALL 维护选择集并显示已选计数
- [x] EARS-FE-P110-02 WHEN 用户全选当前过滤结果 THE SYSTEM SHALL 仅勾选过滤可见行
- [x] EARS-FE-P110-03 WHEN 有选择且导出 THE SYSTEM SHALL 仅导出已选行
- [x] EARS-FE-P110-04 WHEN 用户确认批量禁用/启用 THE SYSTEM SHALL 对已选用户串行应用状态变更并刷新
- [x] EARS-FE-P110-05 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖选择控件与导出/批量按钮
- [x] EARS-DOC-P110-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P110

## 实现备注
- `listSelection` 纯函数 + 单测
- `useListSelection` 组合式
- testid：`users-select-*` / `databases-select-*` / `users-batch-*`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `go test ./...`
