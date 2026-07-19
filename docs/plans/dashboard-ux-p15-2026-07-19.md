# Dashboard 体验增强 EARS 清单（2026-07-19 P15）

## 范围（可商用：强制修改 bootstrap 默认密码）
- bootstrap `admin` 写入 `must_change_password` metadata
- 登录响应 `must_change_password`
- 已认证用户访问业务 API 时门禁拦截
- 改密/管理员设密后清除标记
- Dashboard 强制改密页与路由守卫

## EARS
- [x] EARS-BE-P15-01 WHEN bootstrap 创建默认 admin THE SYSTEM SHALL 标记 `must_change_password`
- [x] EARS-BE-P15-02 WHEN 使用默认密码登录成功 THE SYSTEM SHALL 在响应中返回 `must_change_password=true`
- [x] EARS-BE-P15-03 WHEN 用户仍标记强制改密 THE SYSTEM SHALL 拒绝除 login/logout/change-password 外的业务 API
- [x] EARS-BE-P15-04 WHEN 用户成功改密或管理员重置密码 THE SYSTEM SHALL 清除强制改密标记
- [x] EARS-FE-P15-05 WHEN 登录返回强制改密 THE SYSTEM SHALL 跳转强制改密页并阻止进入控制台
- [x] EARS-FE-P15-06 WHEN 强制改密成功 THE SYSTEM SHALL 清理会话并引导重新登录

## 验证
- `go test ./cmd/mts-server -run 'MustChange|CommercialDashboardSmoke' -count=1`
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`
