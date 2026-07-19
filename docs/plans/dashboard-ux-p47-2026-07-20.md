# Dashboard / 运维可商用 EARS 清单（2026-07-20 P47）

## 范围
- 就绪中心增加部署侧签核证据备注（边缘证书 / 异地备份 / 告警），本地持久化并纳入归档
- 部署材料包补充异地 rsync 与备份失败告警钩子样例
- 备份编排清单文案对齐样例；**不计入评分、不宣称验收完成**

## 边界
- 备注与样例仅为交接材料，不代表目标环境已完成人工签核
- 不自动触发主机侧 cron/rsync/告警

## EARS
- [x] EARS-FE-P47-01 WHEN 用户填写签核证据备注 THE SYSTEM SHALL 持久化到就绪状态
- [x] EARS-FE-P47-02 WHEN 导出就绪归档 THE SYSTEM SHALL 包含 signoff_notes
- [x] EARS-FE-P47-03 WHEN 下载部署材料包 THE SYSTEM SHALL 包含 rsync-offsite 与 backup-alert-hook 样例
- [x] EARS-FE-P47-04 WHEN 计算就绪评分 THE SYSTEM SHALL 不将 signoff_notes 计入总分
- [x] EARS-FE-P47-05 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖签核备注与新样例入口可见性
- [x] EARS-DOC-P47-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P47 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
