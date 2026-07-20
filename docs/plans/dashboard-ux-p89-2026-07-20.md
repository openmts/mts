# Dashboard / 落地页与命令面板最近访问 EARS（2026-07-20 P89）

## 范围
- 账户页：默认登录落地页偏好（localStorage）
- 登录成功 / 已登录访问 Login：无 redirect 时走落地偏好
- 命令面板空查询展示最近访问
- 商业冒烟覆盖相关 testid

## 边界
- 显式 `redirect` query 始终优先于偏好
- 非管理员选择 admin 落地页时回落 `/`
- 不持久化密码/Token
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P89-01 WHEN 用户在账户页选择落地路径 THE SYSTEM SHALL 本机持久化该路径
- [x] EARS-FE-P89-02 WHEN 登录成功且无合法 redirect THE SYSTEM SHALL 导航到落地偏好（或 `/`）
- [x] EARS-FE-P89-03 WHEN 已登录用户打开 Login 且无 redirect THE SYSTEM SHALL 按落地偏好跳转
- [x] EARS-FE-P89-04 WHEN 命令面板打开且查询为空 THE SYSTEM SHALL 展示最近访问列表
- [x] EARS-FE-P89-05 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 `account-landing-select` 与 `command-palette-recent-label`
- [x] EARS-DOC-P89-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P89

## 实现备注
- `landingPrefs` / `recentCommandItems` 纯函数 + 单测
- testid：`account-landing-select`、`account-landing`、`command-palette-recent-label`、`command-recent-*`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
