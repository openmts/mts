# Dashboard UX P224（2026-07-21）

## P224 — API 超时可配置与运维文档

### EARS
- [x] EARS-FE-P224-01 `resolveApiTimeoutMs` 解析/夹逼
- [x] EARS-FE-P224-02 构建期 `VITE_API_TIMEOUT_MS` 覆盖默认 30s
- [x] EARS-DOC-P224-03 `docs/ops/dashboard-production-runbook.md` 说明超时策略

### 非目标
- 宣称可商用完成

### 验证
- [x] npm test / build / test:e2e
- [x] go test ./...
- [x] make e2e
