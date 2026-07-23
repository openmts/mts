# Dashboard UX P499 — Audit 会话摘要 + Users path/批量对齐

## 目标
Audit / Users 页补齐与 P492–P498 一致的 path/会话扫视与深链；导出 meta 对齐服务端 path。

## 范围
- 前端：`auditSessionSummary`、`usersMetaAlign` 纯函数 + 单测
- Audit：会话摘要卡、Users/Readiness 深链、export v2（path/source/filters）
- Users：列表 path/统计卡、batch 结果摘要、Audit/Grants 深链、inventory v2（list/batch path）
- 清单：`audit-session-summary`、`users-meta-align`
- e2e commercial-smoke 软断言
- Server：既有 `path` 字段已覆盖，本轮无强制 Go 变更

## 验收
- [x] auditSessionSummary / usersMetaAlign 单测
- [x] audit-session-summary / users-meta-align testid
- [x] audit export v2 / users inventory v2 meta
- [x] 清单 + commandPalette
- [x] npm test / build / commercial-smoke
