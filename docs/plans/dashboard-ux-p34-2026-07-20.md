# Dashboard / 运维可商用 EARS 清单（2026-07-20 P34）

## 范围
- 降采样统一区间操作：repair / run-range / dry-run（含 advance_watermark）
- 区间校验纯函数 + 契约测试
- 策略运行状态明细表
- 降采样页 i18n 深度收口
- 文档同步

## 边界
- run 仍为单次推进；区间操作用统一对话框
- 区间上限 90 天防止误操作
- 仍不宣称可商用完成

## EARS
- [x] EARS-FE-P34-01 WHEN 管理员点击 run-range/repair/dry-run THE SYSTEM SHALL 打开统一区间对话框
- [x] EARS-FE-P34-02 WHEN 提交 run-range THE SYSTEM SHALL 发送 start_unix/end_unix 与 options.advance_watermark
- [x] EARS-FE-P34-03 WHEN 区间非法 THE SYSTEM SHALL 拒绝并给出可理解错误
- [x] EARS-FE-P34-04 WHEN 策略有 statuses THE SYSTEM SHALL 展示水位/错误明细表
- [x] EARS-DOC-P34-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P34 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
