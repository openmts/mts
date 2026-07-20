# Dashboard / 命令面板结果密度 EARS（2026-07-20 P135）

## 范围
- 命令面板列表更紧凑（跟随 UI density）
- sticky 分组标题；结果计数
- 视口自适应 max-height

## 边界
- 不改命令过滤/动作语义与深链折叠逻辑
- 不引入 VirtualTable（列表规模可控）

## EARS
- [x] EARS-FE-P135-01 WHEN 打开命令面板 THE SYSTEM SHALL 展示匹配结果计数
- [x] EARS-FE-P135-02 WHEN UI density 为 compact THE SYSTEM SHALL 使用更紧凑行高
- [x] EARS-FE-P135-03 WHEN 列表滚动 THE SYSTEM SHALL sticky 保留分组标题
- [x] EARS-FE-P135-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 result-count 与 density 属性
- [x] EARS-DOC-P135-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P135

## 实现备注
- testid：command-palette-panel / command-palette-result-count
- listbox data-density 透传 comfortable|compact

## 验证
- npm test && npm run build && npm run test:e2e ✅
- make e2e + go test ./... ✅
