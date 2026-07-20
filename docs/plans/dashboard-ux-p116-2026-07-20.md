# Dashboard / Audit·Downsample 选择工具条对齐 EARS（2026-07-20 P116）

## 范围
- Audit 接入 `ListSelectionToolbar`（保留 audit-* testid）
- Downsample 接入同一组件；清空按钮兼容 `downsample-clear-select`
- 组件支持可选 clear/select-all/toolbar testid 覆盖

## 边界
- 不改变选择与批量业务语义
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P116-01 WHEN Audit 渲染 THE SYSTEM SHALL 使用 ListSelectionToolbar 且 audit-select-* testid 可用
- [x] EARS-FE-P116-02 WHEN Downsample 渲染 THE SYSTEM SHALL 使用统一工具条并保留 batch/export 动作
- [x] EARS-FE-P116-03 WHEN Downsample 清空选择 THE SYSTEM SHALL 仍可通过 downsample-clear-select 触发
- [x] EARS-FE-P116-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 audit 工具条与 downsample batch 入口
- [x] EARS-DOC-P116-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P116

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `go test ./...`
