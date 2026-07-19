# Dashboard / 运维可商用 EARS 清单（2026-07-20 P27）

## 范围
- 全局通知中心：容量上限、同文案去重、warn 级别
- API 错误码友好映射（纯函数，覆盖服务端 error-codes 主码）
- Overview 摘要条：会话剩余 + 服务版本（admin）+ About 快捷入口
- 文档与冒烟清单同步

## 边界
- 不改服务端错误码契约，只做前端映射与展示
- 不去全局劫持所有 api 调用；提供工具函数供页面使用，并在关键入口接入
- 仍不宣称可商用完成（部署侧验收）

## EARS
- [x] EARS-FE-P27-01 WHEN 连续推送相同 kind+message 通知 THE SYSTEM SHALL 在去重窗口内合并为一条并刷新计数/TTL
- [x] EARS-FE-P27-02 WHEN 通知超过容量上限 THE SYSTEM SHALL 丢弃最旧项
- [x] EARS-FE-P27-03 WHEN 映射 API 错误码 THE SYSTEM SHALL 返回面向运维的中文/英文提示且保留原始 code
- [x] EARS-FE-P27-04 WHEN Overview 加载 THE SYSTEM SHALL 展示会话剩余摘要，管理员额外展示服务 version
- [x] EARS-DOC-P27-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P27 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收
- 目标环境 cron/systemd 与跨主机备份实装
