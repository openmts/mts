# Dashboard / 壳层体验 EARS（2026-07-20 P85）

## 范围
- 侧栏桌面折叠记忆（localStorage）
- 主内容区面包屑导航
- 强制改密 / 账户改密：密码策略分项实时提示
- 商业冒烟覆盖相关 testid

## 边界
- 不改认证/改密服务端契约
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P85-01 WHEN 管理员在桌面宽度使用控制台 THE SYSTEM SHALL 支持折叠侧栏并将偏好持久化
- [x] EARS-FE-P85-02 WHEN 用户进入非概览页面 THE SYSTEM SHALL 展示可导航面包屑
- [x] EARS-FE-P85-03 WHEN 用户在改密表单输入密码 THE SYSTEM SHALL 实时展示策略分项满足状态
- [x] EARS-FE-P85-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 sidebar / breadcrumb / password-hints testid
- [x] EARS-DOC-P85-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P85

## 实现备注
- `sidebarPrefs` / `breadcrumbs` / `passwordHints` 纯函数
- 组件：`BreadcrumbBar`、`PasswordHints`；`SidebarNav` 支持 collapsed

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
