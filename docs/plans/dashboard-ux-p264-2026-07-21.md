# Dashboard UX P264（2026-07-21）

## 目标

Databases 详情加载失败 soft-fail：保留展开态与库列表，分项错误可重试。

## 实现

- `DatabaseEntry.detailError`
- `loadDatabaseDetails` 失败不再折叠展开；写 `detailError`
- 详情面板：loading / PartialErrorBanner / loaded 三态
- i18n：`databasesDetailFailed`
- e2e：健康路径 `databases-detail-error` count 0

## 验收

- [x] npm test / build / test:e2e
