# Dashboard / Query 历史虚拟滚动 EARS（2026-07-20 P134）

## 范围
- 查询历史列表去掉 slice(0,20)，全量 VirtualTable
- 历史上限 30→200；文本筛选
- 商业冒烟覆盖 query-history-virtual-list（有历史时）

## 边界
- 不改历史 push/导出/导入语义
- 导出仍覆盖全部历史（非仅可视行）

## EARS
- [x] EARS-FE-P134-01 WHEN 历史非空 THE SYSTEM SHALL 虚拟渲染可视行
- [x] EARS-FE-P134-02 WHEN 新增历史 THE SYSTEM SHALL 最多保留 200 条
- [x] EARS-FE-P134-03 WHEN 用户输入筛选词 THE SYSTEM SHALL 按名称/模式/库表过滤
- [x] EARS-FE-P134-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 query-history virtual testid（有数据时）
- [x] EARS-DOC-P134-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P134

## 实现备注
- HISTORY_ROW_HEIGHT=56 / HISTORY_LIST_HEIGHT=320
- QUERY_HISTORY_MAX=200
- testid：query-history-virtual-list / query-history-virtual-hint / query-history-filter

## 验证
- npm test && npm run build && npm run test:e2e ✅
- make e2e ✅
- go test ./...：pprof/storage_engine 偶发 flaky，重跑通过 ✅
