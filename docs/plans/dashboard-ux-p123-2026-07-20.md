# Dashboard / Access Grants 虚拟滚动 EARS（2026-07-20 P123）

## 范围
- 实时授权表接入 VirtualTable 虚拟滚动
- 保留过滤、排序、多选、导出
- 空结果仍走 EmptyState

## 边界
- 不改授权 API
- 选择/导出基于筛选全集，非仅可视行

## EARS
- [x] EARS-FE-P123-01 WHEN 授权列表有数据 THE SYSTEM SHALL 以虚拟列表渲染可视行
- [x] EARS-FE-P123-02 WHEN 用户全选/导出 THE SYSTEM SHALL 仍基于当前筛选全集
- [x] EARS-FE-P123-03 WHEN 列表为空 THE SYSTEM SHALL 展示 EmptyState
- [x] EARS-FE-P123-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 在有数据时覆盖 virtual-list
- [x] EARS-DOC-P123-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P123

## 实现备注
- `GRANTS_ROW_HEIGHT=48` / `GRANTS_LIST_HEIGHT=448`
- testid：`access-grants-table` / `access-grants-table-header` / `access-grants-virtual-list` / `access-grants-virtual-hint`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 根目录 `make e2e` + `go test ./...`
