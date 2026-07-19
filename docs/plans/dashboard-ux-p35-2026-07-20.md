# Dashboard / 运维可商用 EARS 清单（2026-07-20 P35）

## 范围
- 文档标题随路由与语言更新
- 降采样创建/区间对话框焦点陷阱与 body scroll lock
- 脏表单离开确认文案 i18n
- 文档同步

## 边界
- 标题映射覆盖主要路由；未知路由回退 app 名
- 仍不宣称可商用完成

## EARS
- [x] EARS-FE-P35-01 WHEN 路由或语言变化 THE SYSTEM SHALL 更新 document.title 为「页面 · 应用名」
- [x] EARS-FE-P35-02 WHEN 打开降采样创建或区间对话框 THE SYSTEM SHALL 陷阱焦点并在关闭时恢复
- [x] EARS-FE-P35-03 WHEN 脏表单离开确认 THE SYSTEM SHALL 使用当前语言文案
- [x] EARS-DOC-P35-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P35 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
