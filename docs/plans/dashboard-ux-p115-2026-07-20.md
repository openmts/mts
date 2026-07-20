# Dashboard / 统一列表选择工具条 EARS（2026-07-20 P115）

## 范围
- 抽取 `ListSelectionToolbar` 组件：已选计数、全选当前、清空选择、插槽扩展（批量/排序按钮）
- Users / Databases / Access Matrix / Access Grants 接入统一组件，减少重复 DOM 与样式漂移
- 保持既有 testid 前缀行为（通过 props 传入）

## 边界
- 不改变选择语义与导出逻辑
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P115-01 WHEN 页面使用 ListSelectionToolbar THE SYSTEM SHALL 展示已选计数（count>0）
- [x] EARS-FE-P115-02 WHEN 用户点击全选/清空 THE SYSTEM SHALL 触发对应回调
- [x] EARS-FE-P115-03 WHEN Users/Databases/AccessMatrix 渲染 THE SYSTEM SHALL 使用统一工具条且 testid 兼容
- [x] EARS-FE-P115-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 既有 selection 断言仍通过
- [x] EARS-DOC-P115-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P115

## 实现备注
- `components/ListSelectionToolbar.vue`
- props：`prefix`（testid 前缀）、`selectedCount`、`hasVisible`、`disabled`
- slots：`actions` 放批量/排序按钮

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
