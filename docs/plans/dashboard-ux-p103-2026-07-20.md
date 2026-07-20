# Dashboard / 返回顶部 EARS（2026-07-20 P103）

## 范围
- 主内容区滚动超过阈值后显示「返回顶部」浮动按钮
- 点击平滑滚动至顶部；命令面板支持「返回主内容顶部」动作
- 商业冒烟覆盖 back-to-top

## 边界
- 仅控制 `#main-content` 滚动容器，不操作 window
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P103-01 WHEN 主内容 scrollTop ≥ 阈值 THE SYSTEM SHALL 显示返回顶部按钮
- [x] EARS-FE-P103-02 WHEN 用户点击返回顶部 THE SYSTEM SHALL 将主内容滚至顶部并隐藏按钮
- [x] EARS-FE-P103-03 WHEN 用户从命令面板执行返回顶部动作 THE SYSTEM SHALL 调用同一滚动逻辑
- [x] EARS-FE-P103-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 back-to-top
- [x] EARS-DOC-P103-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P103

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
