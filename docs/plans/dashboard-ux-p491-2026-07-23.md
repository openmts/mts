# Dashboard UX P491 — 存储导出摘要 + 写入 API path

## 目标
继续压缩原始 JSON 展示：export 结构化摘要；Write 页标明当前写入 API path，便于与契约对齐。

## 范围
- `summarizeStorageExport` 纯函数 + 单测
- Storage 导出摘要卡 / raw 折叠
- Write `write-active-path`
- Server `storage_export` ResponseHint 含 path
- 清单 / 命令面板 / commercial-smoke

## 验收
- [x] summarizeStorageExport 单测（计数 + null）
- [x] storage-export-summary / write-active-path testid
- [x] productionChecklist `storage-export-write-path`
- [x] commandPalette 条目
- [x] npm test / build / commercial-smoke
- [x] 不宣称 dashboard 可商用 goal complete

## 备注
export 摘要在点击导出后出现；e2e 对 summary 做软断言。
