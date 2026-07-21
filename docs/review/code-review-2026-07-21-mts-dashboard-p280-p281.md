# 代码检视：mts-dashboard P280–P281（2026-07-21）

## 范围
- ForceChangePasswordPage 密码可见性
- sessionClockTickMs + useMutationGuard / Account 自适应时钟

## 结论
- 强改密与登录页交互对齐
- 临界态剩余时间从 15s 粒度提升到 1s，减少倒计时跳变感
- 无服务端协议变更

## 状态
| 项 | 状态 |
|----|------|
| P280 | 已实现待验证 |
| P281 | 已实现待验证 |
