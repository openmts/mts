# Dashboard UX P221（2026-07-21）

## P221 — 会话 critical 横幅续期/重登快捷入口

### 背景
会话即将过期时写操作只读，用户需主动找账户页续期；横幅仅文案缺少一键动作。

### EARS
- [x] EARS-FE-P221-01 横幅提供「去续期」→ `/account#account-session`
- [x] EARS-FE-P221-02 横幅提供「重新登录」→ 先 logout，再 login + reason=session + redirect 回账户会话
- [x] EARS-FE-P221-03 i18n zh/en
- [x] EARS-E2E-P221-04 商业冒烟：critical banner 动作可见；续期按钮跳转账户页
- [x] EARS-FE-P221-05 Downsample 范围对话框打开后 writeBlocked 时展示阻断提示

### 非目标
- 服务端独立 refresh token
- 宣称可商用完成

### 验证
- [x] npm test / build / test:e2e
- [x] go test ./...
- [x] make e2e
