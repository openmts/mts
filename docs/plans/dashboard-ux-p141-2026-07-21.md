# Dashboard / 子路径部署对齐说明 EARS（2026-07-21 P141）

## 范围
- 明确 `VITE_BASE`（前端构建）与 `http.dashboard_base`（服务端托管）必须一致
- Config schema 已有 `http.dashboard_base` 字段描述；补充 README/基线
- 不改默认根路径 `/` 行为

## EARS
- [x] EARS-DOC-P141-01 WHEN 子路径部署 THE SYSTEM SHALL 文档要求 VITE_BASE 与 dashboard_base 同前缀
- [x] EARS-FE-P141-02 WHEN 查看 Config schema THE SYSTEM SHALL 可见 dashboard_base 说明

## 备注
- 服务端 `dashboardHandler(base)` 已支持子路径 SPA fallback
- 前端 `vite.config.ts` 已读 `VITE_BASE`

## 验证
- 文档与 schema 字段存在（只读确认 + 构建门禁）✅
