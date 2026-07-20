# Dashboard / 运维可商用 EARS 清单（2026-07-20 P58）

## 范围
- Operations 操作卡片标题与维护/压缩统计标签 i18n
- QueryChart max series 标签 i18n
- ActionResultBanner 关闭按钮 title i18n
- Config Token 字段标签 i18n
- 查询历史快捷键 title i18n

## 边界
- Flush/Compact 可保留运维术语英文
- 不改变运维 API 与确认流程
- 不宣称生产验收完成

## EARS
- [x] EARS-FE-P58-01 WHEN 展示运维动作卡 THE SYSTEM SHALL 使用 opsAction* 标题
- [x] EARS-FE-P58-02 WHEN 展示维护/压缩统计 THE SYSTEM SHALL 使用 opsStat* 标签
- [x] EARS-FE-P58-03 WHEN 展示查询图例控件 THE SYSTEM SHALL 使用 chartMaxSeries
- [x] EARS-FE-P58-04 WHEN 展示结果横幅关闭按钮 THE SYSTEM SHALL 使用 dismiss title
- [x] EARS-FE-P58-05 WHEN 展示配置 Token 字段 THE SYSTEM SHALL 使用本地化标签
- [x] EARS-FE-P58-06 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖运维 Flush 入口
- [x] EARS-DOC-P58-07 WHEN 更新基线 THE SYSTEM SHALL 记录 P58 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
