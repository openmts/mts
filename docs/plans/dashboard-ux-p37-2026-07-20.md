# Dashboard / 运维可商用 EARS 清单（2026-07-20 P37）

## 范围
- 查询 / 写入 / 配置 / 存储核心页用户可见文案 i18n 收口
- formatMessage 占位符工具
- 文档同步

## 边界
- 校验错误与 toast 同步 i18n；注释可不翻译
- 仍不宣称可商用完成

## EARS
- [x] EARS-FE-P37-01 WHEN 语言切换为 en THE SYSTEM SHALL 展示查询页模式/历史/结果等英文文案
- [x] EARS-FE-P37-02 WHEN 语言切换为 en THE SYSTEM SHALL 展示写入页模式/空态/成功提示等英文文案
- [x] EARS-FE-P37-03 WHEN 语言切换为 en THE SYSTEM SHALL 展示配置与存储关键操作英文文案
- [x] EARS-DOC-P37-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P37 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
