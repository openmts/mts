# Dashboard / 运维可商用 EARS 清单（2026-07-20 P48）

## 范围
- 签核备注完整性评估（纯函数）
- 导出归档/验收包时合成 note；缺失字段导出前确认，可继续
- UI 展示完整性状态；**不计入评分、不宣称验收完成**

## 边界
- 确认对话框允许用户在缺失备注时仍导出
- 合成 note 仅为交接摘要，不替代部署侧真实验收

## EARS
- [x] EARS-FE-P48-01 WHEN 评估签核备注 THE SYSTEM SHALL 返回 filled/missing 字段
- [x] EARS-FE-P48-02 WHEN 导出归档 THE SYSTEM SHALL 将签核备注合成为 archive.note
- [x] EARS-FE-P48-03 WHEN 签核备注缺失且用户取消确认 THE SYSTEM SHALL 中止导出
- [x] EARS-FE-P48-04 WHEN 就绪中心展示签核区 THE SYSTEM SHALL 显示完整性状态
- [x] EARS-FE-P48-05 WHEN 商业冒烟 e2e 运行 THE SYSTEM SHALL 覆盖完整性入口可见性
- [x] EARS-DOC-P48-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P48 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
