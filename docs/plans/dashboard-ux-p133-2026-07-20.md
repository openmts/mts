# Dashboard / Query 列摘要虚拟滚动 EARS（2026-07-20 P133）

## 范围
- Query 行结果 VirtualTable 补齐 testid/hint
- Query 列模式摘要表接入 VirtualTable
- 保留列显隐、CSV 导出、图表

## 边界
- 不改查询 API / workbench 语义
- 导出 CSV 仍基于全量 rows

## EARS
- [x] EARS-FE-P133-01 WHEN 行结果非空 THE SYSTEM SHALL 暴露 query-results-virtual-list
- [x] EARS-FE-P133-02 WHEN 列摘要非空 THE SYSTEM SHALL 虚拟渲染可视序列行
- [x] EARS-FE-P133-03 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 query virtual testid（有结果时）
- [x] EARS-DOC-P133-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P133

## 实现备注
- COLUMN_ROW_HEIGHT=40 / COLUMN_LIST_HEIGHT=320
- testid：query-results-virtual-list / query-results-virtual-hint / query-columns-virtual-list

## 验证
- npm test && npm run build && npm run test:e2e ✅
- make e2e + go test ./... ✅
- e2e 等待改为 query-run enabled + 元数据预填，避免固定 waitForTimeout
