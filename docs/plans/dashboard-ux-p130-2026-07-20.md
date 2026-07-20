# Dashboard / Metrics family 虚拟滚动 EARS（2026-07-20 P130）

## 范围
- Metrics family 列表接入 VirtualTable（固定行高）
- 样本详情改为单展开面板，避免可变行高破坏虚拟窗口
- 保留筛选、导出 raw/json、自动刷新

## 边界
- 不改 /metrics 抓取与 Prometheus 解析语义
- 导出 JSON 仍基于当前筛选全集

## EARS
- [x] EARS-FE-P130-01 WHEN metrics family 有匹配项 THE SYSTEM SHALL 虚拟渲染可视 family 行
- [x] EARS-FE-P130-02 WHEN 用户展开某一 family THE SYSTEM SHALL 在详情面板展示 samples
- [x] EARS-FE-P130-03 WHEN 导出 JSON THE SYSTEM SHALL 覆盖当前筛选全集
- [x] EARS-FE-P130-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 metrics-virtual-list
- [x] EARS-DOC-P130-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P130

## 实现备注
- FAMILY_ROW_HEIGHT=64 / LIST_HEIGHT=480
- testid：metrics-virtual-list / metrics-virtual-hint / metrics-detail-panel

## 验证
- npm test && npm run build && npm run test:e2e
- make e2e + go test ./...
