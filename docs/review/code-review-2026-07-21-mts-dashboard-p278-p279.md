# 代码检视：mts-dashboard P278–P279（2026-07-21）

## 范围
- `cmd/mts-dashboard/src/pages/LoginPage.vue`
- `cmd/mts-dashboard/src/pages/ForceChangePasswordPage.vue`
- `cmd/mts-dashboard/e2e/commercial-smoke.spec.ts`

## 结论
- 会话 critical 剩余时间与写门禁 e2e 已补齐（P278）
- 登录/强改密错误恢复语义正确：本地校验不可 retry，服务端/离线可 retry（P279）
- e2e 曾错误地在空密码路径要求 retry，并在 dismiss 后 assert role，已修复

## 状态
| 项 | 状态 |
|----|------|
| P278 session remaining e2e | 已修复/待验证合入 |
| P279 login/force error recovery UI | 已实现 |
| P279 e2e 语义对齐 | 已修复 |

## 非目标
- refresh token、边缘 HSTS、expr 树 UI 等
