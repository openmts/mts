# Dashboard / 运维可商用 EARS 清单（2026-07-20 P33）

## 范围
- 降采样页接入 repair 区间修复（对齐服务端 `/repair`）
- 降采样页标题/创建等文案 i18n 收口
- 文档同步

## 边界
- repair 需用户确认时间范围；默认最近 24h 或基于完成水位
- 不新增服务端批量 repair API
- 仍不宣称可商用完成

## EARS
- [x] EARS-FE-P33-01 WHEN 管理员点击策略 repair THE SYSTEM SHALL 弹出时间范围对话框
- [x] EARS-FE-P33-02 WHEN 确认 repair 且范围合法 THE SYSTEM SHALL 调用 enable 路径同源的 `/repair` API 并展示结果
- [x] EARS-FE-P33-03 WHEN 范围非法 THE SYSTEM SHALL 拒绝提交并提示错误
- [x] EARS-DOC-P33-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P33 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
