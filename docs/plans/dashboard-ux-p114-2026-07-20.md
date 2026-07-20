# Dashboard / Access Matrix 清单能力对齐 EARS（2026-07-20 P114）

## 范围
- Access Matrix：文本搜索、列排序（area/capability/admin/user/route）、多选与导出（JSON+CSV）
- 导出优先已选行；无选择导出当前过滤结果
- sticky 表头与选择工具条，与 Users/Audit 对齐

## 边界
- 矩阵为只读前端 catalog，不写后端
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P114-01 WHEN 用户输入矩阵搜索词 THE SYSTEM SHALL 按区域/能力/路由/备注过滤
- [x] EARS-FE-P114-02 WHEN 用户点击矩阵表头排序 THE SYSTEM SHALL 循环排序并本机记忆
- [x] EARS-FE-P114-03 WHEN 用户多选后导出 THE SYSTEM SHALL 仅导出已选行
- [x] EARS-FE-P114-04 WHEN 用户导出 CSV THE SYSTEM SHALL 下载本地化矩阵清单
- [x] EARS-FE-P114-05 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖搜索/排序/选择/CSV
- [x] EARS-DOC-P114-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P114

## 实现备注
- `accessMatrixToCSV` + 单测
- 复用 `listSelection` / `listSort` / `useListSelection`
- key `mts.dashboard.access-matrix-sort.prefs.v1`
- testid：`access-matrix-search` / `access-matrix-select-*` / `access-matrix-sort-*` / `access-matrix-export-csv`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
