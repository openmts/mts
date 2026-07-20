# Dashboard / Access Matrix 虚拟滚动 EARS（2026-07-20 P124）

## 范围
- 权限能力矩阵表接入 VirtualTable 虚拟滚动
- 保留角色/区域/文本筛选、排序、多选、JSON/CSV 导出
- 无匹配时展示空态文案

## 边界
- 不改 RBAC 矩阵数据源与权限语义
- 选择/导出基于筛选全集，非仅可视行

## EARS
- [x] EARS-FE-P124-01 WHEN 矩阵有匹配行 THE SYSTEM SHALL 以虚拟列表渲染可视行
- [x] EARS-FE-P124-02 WHEN 用户全选/导出 THE SYSTEM SHALL 仍基于当前筛选全集
- [x] EARS-FE-P124-03 WHEN 筛选无匹配 THE SYSTEM SHALL 展示空态提示
- [x] EARS-FE-P124-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 virtual-list
- [x] EARS-DOC-P124-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P124

## 实现备注
- `MATRIX_ROW_HEIGHT=48` / `MATRIX_LIST_HEIGHT=448`
- testid：`access-matrix-table` / `access-matrix-table-header` / `access-matrix-virtual-list` / `access-matrix-virtual-hint` / `access-matrix-empty`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 根目录 `make e2e` + `go test ./...`
