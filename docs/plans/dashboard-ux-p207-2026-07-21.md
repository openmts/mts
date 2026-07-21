# Dashboard UX P207（2026-07-21）

## P207 — 剩余小导出统一 ExportJob

- [x] EARS-FE-P207-01 Query 历史导出走 `runJSONExport`，导出中禁用并展示 banner
- [x] EARS-FE-P207-02 Overview / About 快照导出接入 ExportJob
- [x] EARS-FE-P207-03 Operations stats / maintenance-errors / action-log 导出接入 ExportJob
- [x] EARS-FE-P207-04 Storage 配置下载接入 ExportJob
- [x] EARS-FE-P207-05 Account 账户快照与本机偏好导出接入 ExportJob
- [x] EARS-FE-P207-06 Write 结果/草稿导出接入 ExportJob
- [x] EARS-FE-P207-07 NotifyHistory JSON/CSV 导出接入 ExportJob
- [x] EARS-FE-P207-08 页面直连 `downloadJSON`/`downloadText` 仅保留在 `useExportJob` 与 `utils/download`

## 非目标
- 服务端独立 refresh token
- 对象存储冷层 / 部署证书 / cron
- 宣称可商用完成

## 验证
- [x] `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- [x] `go test ./...`
- [x] `make e2e`
