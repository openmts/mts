# Dashboard / Storage 快照表虚拟滚动 EARS（2026-07-20 P128）

## 范围
- 配置健康快照列表接入 VirtualTable
- data_dir 快照列表接入 VirtualTable
- 修复 loading EmptyState description 绑定

## 边界
- 不改 snapshot / data-snapshot / restore-drill API 语义
- 删除确认门禁保持不变

## EARS
- [x] EARS-FE-P128-01 WHEN 配置快照非空 THE SYSTEM SHALL 虚拟渲染可视行
- [x] EARS-FE-P128-02 WHEN data 快照非空 THE SYSTEM SHALL 虚拟渲染可视行
- [x] EARS-FE-P128-03 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 storage 虚拟列表 testid
- [x] EARS-DOC-P128-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P128

## 实现备注
- SNAPSHOT_ROW_HEIGHT=44 / DATA_ROW_HEIGHT=48 / LIST_HEIGHT=320
- testid：storage-snapshots-virtual-list / storage-data-virtual-list / storage-*-virtual-hint

## 验证
- npm test && npm run build && npm run test:e2e
- make e2e + go test ./...
