# Dashboard / 命令面板键盘与索引增强 EARS（2026-07-20 P105）

## 范围
- 列表选中索引 O(1) map，避免模板重复 findIndex
- Home/End 跳到首/末项；Arrow 导航后 scrollIntoView nearest
- 纯函数单测 + 商业冒烟覆盖 Home/End

## 边界
- 不改变 Enter 执行语义与分组顺序
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P105-01 WHEN 用户按 End THE SYSTEM SHALL 选中扁平列表最后一项
- [x] EARS-FE-P105-02 WHEN 用户按 Home THE SYSTEM SHALL 选中扁平列表第一项
- [x] EARS-FE-P105-03 WHEN 选中项变化 THE SYSTEM SHALL 将选项滚入可视区域
- [x] EARS-FE-P105-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 Home/End 选中态
- [x] EARS-DOC-P105-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P105

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
