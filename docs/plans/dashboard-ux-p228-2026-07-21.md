# Dashboard UX P228（2026-07-21）

## 目标
P227 后续体验收口：取消态视觉区分、导入错误友好化、query-cancel e2e。

## EARS
- [x] EARS-FE-P228-01 Readiness 导入 catch 使用 `formatCaughtError`
- [x] EARS-FE-P228-02 Query `actionError` 在 `canceled` 时用 `mts-alert-info` + `query-action-error`
- [x] EARS-E2E-P228-03 空闲时 `query-cancel` disabled

## 非目标
- NDJSON idle timeout、服务端 refresh token、宣称可商用完成
