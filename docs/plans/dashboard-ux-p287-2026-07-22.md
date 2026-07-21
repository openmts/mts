# Dashboard UX P287（2026-07-22）

## 目标
- 导出失败后在 ExportJobBanner 提供一键重试
- useExportJob 记忆上次导出参数，全页统一接入

## 验收
- [x] `canRetryExportJob` / banner retry 按钮
- [x] `retryLastExport` + e2e fail 注入 `__MTS_E2E_FAIL_EXPORT`
- [x] 全站 ExportJobBanner 支持 retryable
- [x] npm test / build / e2e 通过后合入
