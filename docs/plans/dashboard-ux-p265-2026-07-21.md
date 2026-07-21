# Dashboard UX P265（2026-07-21）

## 目标

Storage 快照 / 数据快照双列表 soft-fail：已有数据时刷新失败保留列表。

## 实现

- `snapshotListError` / `dataListError` 分项
- hard 空列表 → ActionResultBanner；soft 有数据 → PartialErrorBanner
- `storage-refresh` aria-busy + testid
- i18n 与 e2e 健康路径

## 验收

- [x] npm test / build / e2e
