# Dashboard / Config 表虚拟滚动 EARS（2026-07-20 P129）

## 范围
- Config schema 表接入 VirtualTable（保留筛选与导出）
- Error codes 表接入 VirtualTable（保留导出）
- 空态保留既有文案

## 边界
- 不改 config/effective、validate、reload、error-codes API 语义
- 导出仍基于当前筛选 schema / 全量 error codes

## EARS
- [x] EARS-FE-P129-01 WHEN schema 有匹配项 THE SYSTEM SHALL 虚拟渲染可视行
- [x] EARS-FE-P129-02 WHEN error codes 非空 THE SYSTEM SHALL 虚拟渲染可视行
- [x] EARS-FE-P129-03 WHEN 筛选无匹配 THE SYSTEM SHALL 展示 schema 空态
- [x] EARS-FE-P129-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 virtual-list testid
- [x] EARS-DOC-P129-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P129

## 实现备注
- SCHEMA_ROW_HEIGHT=40 / ERROR_ROW_HEIGHT=44 / LIST_HEIGHT=320
- testid：config-schema-virtual-list / config-error-codes-virtual-list / *-virtual-hint
- 表头文案仍可被 config-schema-table / config-error-codes-table 命中

## 验证
- npm test && npm run build && npm run test:e2e
- make e2e + go test ./...
