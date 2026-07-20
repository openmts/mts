# Dashboard / 运维可商用 EARS 清单（2026-07-20 P68）

## 范围
- 登录 / 强制改密表单无障碍（错误 live 区、aria-invalid）
- prefers-reduced-motion：减弱进度条与非必要动画
- 浏览器离线/在线状态条（可商用运维可感知性）
- 商业冒烟覆盖登录错误 live 与离线条 testid 存在性（在线时隐藏）

## 边界
- 离线条仅反映浏览器 navigator.onLine，不探测服务端可用性
- 不改变鉴权/API 契约
- 不计分部署验收项不变；不宣称生产验收完成

## EARS
- [x] EARS-FE-P68-01 WHEN 登录失败 THE SYSTEM SHALL 以 role=alert 展示错误，并对用户名/密码设置 aria-invalid
- [x] EARS-FE-P68-02 WHEN 强制改密校验失败 THE SYSTEM SHALL 以 role=alert 展示错误
- [x] EARS-FE-P68-03 WHEN 用户启用 prefers-reduced-motion THE SYSTEM SHALL 禁用全局进度脉冲动画
- [x] EARS-FE-P68-04 WHEN 浏览器离线 THE SYSTEM SHALL 在布局顶栏展示离线提示条
- [x] EARS-FE-P68-05 WHEN 浏览器恢复在线 THE SYSTEM SHALL 隐藏离线条
- [x] EARS-FE-P68-06 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖登录表单 aria 与离线条 testid
- [x] EARS-DOC-P68-07 WHEN 更新基线 THE SYSTEM SHALL 记录 P68 与仍未完成部署侧项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
