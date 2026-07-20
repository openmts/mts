# Dashboard / 命令面板导航动作分组 EARS（2026-07-20 P104）

## 范围
- 命令面板结果按「导航 / 动作」分组展示
- 键盘选择顺序与扁平列表一致：导航在前、动作在后
- 空组不渲染；商业冒烟覆盖分组 testid

## 边界
- 不改变既有跳转/动作语义
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P104-01 WHEN 命令面板打开且无查询 THE SYSTEM SHALL 展示导航与动作分组标题
- [x] EARS-FE-P104-02 WHEN 查询仅匹配动作 THE SYSTEM SHALL 仅展示动作分组
- [x] EARS-FE-P104-03 WHEN 用户键盘上下选择 THE SYSTEM SHALL 按扁平顺序跨组移动
- [x] EARS-FE-P104-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 command-palette-group-*
- [x] EARS-DOC-P104-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P104

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
