# Dashboard / Audit 虚拟滚动 EARS（2026-07-20 P121）

## 范围
- 审计事件表接入 VirtualTable 虚拟滚动
- 保留多选、排序、sticky 表头、导出（覆盖筛选全集）
- 空结果仍走 EmptyState

## 边界
- 不改服务端 audit API / limit 语义
- 不自动导出；虚拟化仅影响渲染行数

## EARS
- [x] EARS-FE-P121-01 WHEN 审计列表有数据 THE SYSTEM SHALL 以虚拟列表渲染可视行
- [x] EARS-FE-P121-02 WHEN 用户全选/导出 THE SYSTEM SHALL 仍基于当前筛选全集而非仅可视行
- [x] EARS-FE-P121-03 WHEN 列表为空 THE SYSTEM SHALL 展示 EmptyState 而非空虚拟列表
- [x] EARS-FE-P121-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 audit-table/header 与可选虚拟列表
- [x] EARS-DOC-P121-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P121

## 实现备注
- `displayedRows` 稳定 id+idx；`AUDIT_ROW_HEIGHT=44` / `AUDIT_LIST_HEIGHT=448`
- testid：`audit-table` / `audit-table-header` / `audit-virtual-list` / `audit-virtual-hint` / `audit-row-*`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 根目录 `make e2e` + `go test ./...`
