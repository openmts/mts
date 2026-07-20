# Dashboard / Databases 虚拟滚动 EARS（2026-07-20 P125）

## 范围
- Databases 顶层库列表接入 VirtualTable 虚拟滚动
- 折叠详情改为单展开面板（固定行高，避免可变行高破坏虚拟窗口）
- 保留筛选、排序、多选、JSON/CSV 导出

## 边界
- 不改库/RP/measurement 管理 API 语义
- 选择/导出基于筛选全集，非仅可视行
- measurement 与 RP 详情仍懒加载

## EARS
- [x] EARS-FE-P125-01 WHEN 数据库列表有匹配项 THE SYSTEM SHALL 以虚拟列表渲染可视行
- [x] EARS-FE-P125-02 WHEN 用户展开某一库 THE SYSTEM SHALL 仅保留单库详情面板并懒加载元数据
- [x] EARS-FE-P125-03 WHEN 用户全选/导出 THE SYSTEM SHALL 仍基于当前筛选全集
- [x] EARS-FE-P125-04 WHEN 筛选无匹配 THE SYSTEM SHALL 展示既有空态
- [x] EARS-FE-P125-05 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 databases-virtual-list
- [x] EARS-DOC-P125-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P125

## 实现备注
- `DB_ROW_HEIGHT=52` / `DB_LIST_HEIGHT=416`
- testid：`databases-virtual-list` / `databases-virtual-hint` / `databases-detail-panel` / `databases-row-*`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 根目录 `make e2e` + `go test ./...`
