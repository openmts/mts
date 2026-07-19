# Dashboard 体验增强 EARS 清单（2026-07-19 P11）

## 范围（可商用：全局反馈与会话）
- 全局请求/路由 loading 进度条
- 路由懒加载 Skeleton
- 401/会话失效通知 + 登录原因提示
- 会话剩余时间角标
- 纯逻辑单测（loading / expiry / auth reason）

## EARS
- [x] EARS-FE-P11-01 WHEN 任意 API 请求进行中 THE SYSTEM SHALL 显示顶部全局进度条
- [x] EARS-FE-P11-02 WHEN 路由懒加载组件 THE SYSTEM SHALL 展示 PageSkeleton 作为 Suspense fallback
- [x] EARS-FE-P11-03 WHEN 认证失败或 token 过期 THE SYSTEM SHALL toast 提示并跳转登录，附带 reason
- [x] EARS-FE-P11-04 WHEN 登录页 query.reason 存在 THE SYSTEM SHALL 展示对应会话提示
- [x] EARS-FE-P11-05 WHEN 会话剩余时间进入 warn/critical THE SYSTEM SHALL 在顶栏显示剩余时间角标
- [x] EARS-FE-P11-06 WHEN 其他标签页清除会话 THE SYSTEM SHALL 提示并跳转登录

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`
