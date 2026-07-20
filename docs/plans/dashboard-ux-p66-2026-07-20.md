# Dashboard / 运维可商用 EARS 清单（2026-07-20 P66）

## 范围
- ErrorBoundary / 会话过期标签 / 查询延迟水线 / 结果列标签 i18n
- API client 与 meta 层用户可见中文硬编码收口（英文界面不泄漏）
- main 鉴权失败 / 多标签登出 toast 走统一 reason 文案
- TopBar 图标按钮 aria-label 与角色展示本地化

## 边界
- 服务端原始 message 仍可在中文 locale 作为补充细节
- 不计分部署验收项不变
- 不宣称生产验收完成

## EARS
- [x] EARS-FE-P66-01 WHEN ErrorBoundary 捕获渲染错误 THE SYSTEM SHALL 使用 i18n 标题/描述/操作文案
- [x] EARS-FE-P66-02 WHEN 会话剩余为 expired THE SYSTEM SHALL 按 locale 显示过期标签
- [x] EARS-FE-P66-03 WHEN 展示查询延迟水线 THE SYSTEM SHALL 按 locale 显示快速/正常/偏慢/很慢
- [x] EARS-FE-P66-04 WHEN 展示查询结果列开关 THE SYSTEM SHALL 按 locale 显示列名
- [x] EARS-FE-P66-05 WHEN API client 抛出会话/取消/解析错误 THE SYSTEM SHALL 以 error code 为主，避免固定中文 message 泄漏到英文 UI
- [x] EARS-FE-P66-06 WHEN 鉴权失败或跨标签登出 THE SYSTEM SHALL 使用 loginReasonMessage 友好 toast
- [x] EARS-FE-P66-07 WHEN TopBar 展示用户角色 THE SYSTEM SHALL 本地化 admin/user
- [x] EARS-DOC-P66-08 WHEN 更新基线 THE SYSTEM SHALL 记录 P66 与仍未完成部署侧项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
