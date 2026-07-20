# Dashboard / 路由切换主内容回顶 EARS（2026-07-20 P106）

## 范围
- 路由 path 变化时主内容 `#main-content` 自动回顶
- 同页仅 hash 变化时不回顶，避免破坏深链锚点
- 商业冒烟覆盖跨页 scrollTop 归零

## 边界
- 使用 auto 行为避免与 hash 平滑滚动竞态
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P106-01 WHEN 用户从 path A 导航到 path B THE SYSTEM SHALL 将主内容滚至顶部
- [x] EARS-FE-P106-02 WHEN 用户仅改变 hash THE SYSTEM SHALL 不强制回顶
- [x] EARS-FE-P106-03 WHEN 回顶发生 THE SYSTEM SHALL 隐藏返回顶部按钮
- [x] EARS-FE-P106-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖跨页 scrollTop=0
- [x] EARS-DOC-P106-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P106

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
