# Dashboard UX P268（2026-07-21）

## 目标

Downsample 策略/状态分项 soft-fail。

## 实现

- `policiesError` / `statusesError`
- `Promise.allSettled` 加载；单侧失败保留对侧与本侧快照
- 双侧皆失败且无快照 → 兼容 `downsample-load-error`
- PartialErrorBanner 分项可重试

## 附带

- measurement 详情失败仅就地 banner，避免双重 toast
