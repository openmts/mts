# Code Review — Dashboard P218 Users 会话文案（2026-07-21）

## 范围
- `UserModals.vue` / `UserGrantPanel.vue`：writeBlocked + blockReason
- `UsersPage.vue` 传参
- commercial-smoke session-critical 断言

## 结论
- 修正 critical 误显示「离线」文案
- 保留 `offline` prop 兼容

## 处理状态
| 项 | 状态 |
|----|------|
| 组件门禁文案 | 已处理 |
| e2e | 已处理 |

## 残余
- ConfirmDialog 批量/删除仍靠父级 writeBlocked 拦截打开，未单独传 session title（打开前已拦截）
