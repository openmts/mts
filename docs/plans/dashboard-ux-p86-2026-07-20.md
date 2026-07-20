# Dashboard / 登录与面包屑体验 EARS（2026-07-20 P86）

## 范围
- 登录：密码显隐切换、记住用户名（不存密码）
- 面包屑：复制当前路径
- 商业冒烟覆盖相关 testid

## 边界
- 不持久化密码或 Token
- 不改登录 API 契约
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P86-01 WHEN 用户在登录页输入密码 THE SYSTEM SHALL 支持显示/隐藏密码
- [x] EARS-FE-P86-02 WHEN 用户勾选记住用户名并登录成功 THE SYSTEM SHALL 在本机记忆用户名
- [x] EARS-FE-P86-03 WHEN 用户查看面包屑 THE SYSTEM SHALL 支持一键复制当前路径
- [x] EARS-FE-P86-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 login-toggle/remember 与 breadcrumb-copy testid
- [x] EARS-DOC-P86-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P86

## 实现备注
- `loginUsernamePrefs` 纯函数；密码永不写入 storage
- testid：`login-toggle-password`、`login-remember-user`、`breadcrumb-copy-path`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
