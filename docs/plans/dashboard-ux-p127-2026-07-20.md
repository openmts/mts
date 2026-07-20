# Dashboard / Downsample 虚拟滚动 EARS（2026-07-20 P127）

## 范围
- 降采样策略表接入 VirtualTable
- 状态表接入 VirtualTable
- 保留筛选、多选、批量启用/禁用、导出

## 边界
- 不改 downsample API / 区间修复语义
- 选择/导出基于筛选全集

## EARS
- [x] EARS-FE-P127-01 WHEN 策略列表有匹配项 THE SYSTEM SHALL 虚拟渲染可视策略行
- [x] EARS-FE-P127-02 WHEN 状态列表有匹配项 THE SYSTEM SHALL 虚拟渲染可视状态行
- [x] EARS-FE-P127-03 WHEN 用户全选/导出 THE SYSTEM SHALL 仍基于筛选全集
- [x] EARS-FE-P127-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 downsample-virtual-list
- [x] EARS-DOC-P127-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P127

## 实现备注
- POLICY_ROW_HEIGHT=56 / STATUS_ROW_HEIGHT=44 / LIST_HEIGHT=400
- testid：downsample-policies-table / downsample-virtual-list / downsample-virtual-hint / downsample-status-virtual-list

## 验证
- npm test && npm run build && npm run test:e2e
- make e2e + go test ./...
