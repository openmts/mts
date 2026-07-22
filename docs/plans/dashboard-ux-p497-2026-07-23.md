# Dashboard UX P497 — 运维动作会话摘要 + Metrics 深链

## 目标
Operations 展示本会话 flush/compact/retention 结构化摘要（含 path），动作日志落 path，并深链 Metrics/Readiness。

## 验收
- [x] summarizeOpsActionLog 单测 + path 持久化
- [x] ops-action-summary / jump-metrics / jump-readiness testid
- [x] 清单 ops-action-summary
- [x] npm test / build / commercial-smoke
