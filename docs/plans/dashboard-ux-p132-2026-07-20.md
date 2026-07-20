# Dashboard / 通知历史与维护错误虚拟滚动 EARS（2026-07-20 P132）

## 范围
- NotifyHistoryPanel 列表接入 VirtualTable
- 通知历史上限 40→200
- Operations 维护错误列表接入 VirtualTable + 文本筛选

## 边界
- 不改 toast 触发与通知语义
- 导出/复制仍基于当前筛选结果

## EARS
- [x] EARS-FE-P132-01 WHEN 通知历史有匹配项 THE SYSTEM SHALL 虚拟渲染可视行
- [x] EARS-FE-P132-02 WHEN 新增通知历史 THE SYSTEM SHALL 最多保留 200 条
- [x] EARS-FE-P132-03 WHEN 维护错误非空 THE SYSTEM SHALL 支持筛选与虚拟滚动
- [x] EARS-FE-P132-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 notify/ops-maint virtual list
- [x] EARS-DOC-P132-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P132

## 实现备注
- NOTIFY_ROW_HEIGHT=72 / MAINT_ROW_HEIGHT=36
- testid：notify-history-virtual-list / ops-maint-errors-virtual-list

## 验证
- npm test && npm run build && npm run test:e2e
- make e2e + go test ./...
