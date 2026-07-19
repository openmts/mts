# Dashboard / 运维可商用 EARS 清单（2026-07-20 P49）

## 范围
- Overview 展示签核备注完整性与跳转入口
- 验收包 JSON/Markdown 纳入 signoff_completeness 摘要
- 就绪中心 #signoff-notes 锚点

## 边界
- 完整性展示不计分，不宣称生产验收完成
- 不自动执行主机侧证书/备份/告警

## EARS
- [x] EARS-FE-P49-01 WHEN 管理员打开 Overview THE SYSTEM SHALL 展示签核备注完整性摘要
- [x] EARS-FE-P49-02 WHEN 用户点击打开签核备注 THE SYSTEM SHALL 进入就绪中心签核区
- [x] EARS-FE-P49-03 WHEN 导出验收包 THE SYSTEM SHALL 包含 signoff_completeness
- [x] EARS-FE-P49-04 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖 Overview 签核入口
- [x] EARS-DOC-P49-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P49 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
