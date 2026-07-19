# Dashboard / 运维可商用 EARS 清单（2026-07-20 P41）

## 范围
- 共享组件中文硬编码收口：ConfirmDialog / PermissionDenied / UserModals / UserGrantPanel
- 就绪中心「自动覆盖」文案 i18n
- 文档同步

## 边界
- 不改变后端鉴权与业务语义
- 仍不宣称可商用完成（边缘证书/HSTS 验收、cron/systemd、跨主机备份）

## EARS
- [x] EARS-FE-P41-01 WHEN 语言为 en THE SYSTEM SHALL 展示 ConfirmDialog 默认确认/取消/处理中与 type-to-confirm 英文文案
- [x] EARS-FE-P41-02 WHEN 非 admin 访问管理页 THE SYSTEM SHALL 展示 PermissionDenied 双语描述
- [x] EARS-FE-P41-03 WHEN 打开用户创建/设密/改密弹窗 THE SYSTEM SHALL 使用 i18n 文案
- [x] EARS-FE-P41-04 WHEN 打开库级授权面板 THE SYSTEM SHALL 使用 i18n 文案
- [x] EARS-FE-P41-05 WHEN 就绪中心展示自动覆盖计数 THE SYSTEM SHALL 使用 i18n 模板
- [x] EARS-DOC-P41-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P41 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build`
- `make dashboard-test-e2e` 或 `cd cmd/mts-dashboard && npm run test:e2e`
- 仓库根：`make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
- 运维清单/备份演练等数据层双语（productionChecklist 等）
