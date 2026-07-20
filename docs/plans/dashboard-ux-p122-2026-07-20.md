# Dashboard / Users 虚拟滚动 EARS（2026-07-20 P122）

## 范围
- 用户列表接入 VirtualTable 虚拟滚动
- 保留过滤、排序、多选、批量启用/禁用、导出
- 空结果仍走 EmptyState

## 边界
- 不改用户 API
- 选择/导出基于筛选全集，非仅可视行

## EARS
- [x] EARS-FE-P122-01 WHEN 用户列表有数据 THE SYSTEM SHALL 以虚拟列表渲染可视行
- [x] EARS-FE-P122-02 WHEN 用户全选/导出 THE SYSTEM SHALL 仍基于当前筛选全集
- [x] EARS-FE-P122-03 WHEN 列表为空 THE SYSTEM SHALL 展示 EmptyState
- [x] EARS-FE-P122-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 users-table/virtual-list
- [x] EARS-DOC-P122-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P122

## 实现备注
- `USERS_ROW_HEIGHT=52` / `USERS_LIST_HEIGHT=448`
- testid：`users-table` / `users-table-header` / `users-virtual-list` / `users-virtual-hint` / `users-row-*`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 根目录 `make e2e` + `go test ./...`
