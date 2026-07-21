# Dashboard UX P245（2026-07-21）

## 目标
Write 元数据失败可恢复，对齐 Query meta 体验。

## EARS
- [x] EARS-FE-P245-01 `write-meta-hint` 提供 `write-meta-reload`
- [x] EARS-FE-P245-02 `write-meta-error` 提供重试 `reloadWriteMeta` + alert live region
- [x] EARS-FE-P245-03 RP 列表失败写入 `writeMetaError`（不再仅 hint）
- [x] EARS-E2E-P245-04 健康路径 `write-meta-error` 不出现

## 非目标
- 宣称可商用完成
