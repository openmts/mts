# Dashboard / 运维可商用 EARS 清单（2026-07-20 P26）

## 范围
- 会话剩余时间常显 + warn/critical 一次性 toast 预警
- 会话到期自动登出并跳转登录（带 session 原因）
- 账户页自愿修改密码（复用改密 API 与策略校验）
- 改密策略纯函数可单测；文档与冒烟清单同步

## 边界
- 不实现服务端 refresh token / 滑动续期
- 不改鉴权协议；到期仍 clearAuth + 登录页
- 强制改密页逻辑保持，自愿改密走 `/account`

## EARS
- [x] EARS-FE-P26-01 WHEN token 有 expires_at THE SYSTEM SHALL 在顶栏展示会话剩余时间（含 ok 态）
- [x] EARS-FE-P26-02 WHEN 剩余时间进入 warn/critical THE SYSTEM SHALL 各弹出一次 toast 预警且不重复刷屏
- [x] EARS-FE-P26-03 WHEN 会话已过期 THE SYSTEM SHALL 自动登出并跳转登录页（reason=session）
- [x] EARS-FE-P26-04 WHEN 用户打开账户页 THE SYSTEM SHALL 可自愿修改密码；成功后要求重新登录
- [x] EARS-FE-P26-05 WHEN 校验新密码 THE SYSTEM SHALL 拒绝过短/与旧密码相同/默认 admin
- [x] EARS-DOC-P26-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P26 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`（按门禁）

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收
- 目标环境 cron/systemd 与跨主机备份实装
