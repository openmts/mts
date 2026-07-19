# Dashboard / 运维可商用 EARS 清单（2026-07-20 P38）

## 范围
- 用户 / 数据库 / 运维页用户可见文案 i18n 收口
- 文档同步

## 边界
- 注释可不翻译
- 仍不宣称可商用完成

## EARS
- [x] EARS-FE-P38-01 WHEN 语言为 en THE SYSTEM SHALL 展示用户管理页英文文案
- [x] EARS-FE-P38-02 WHEN 语言为 en THE SYSTEM SHALL 展示数据库管理页英文文案
- [x] EARS-FE-P38-03 WHEN 语言为 en THE SYSTEM SHALL 展示运维确认与结果英文文案
- [x] EARS-DOC-P38-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P38 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
