# Dashboard UX P263（2026-07-21）

## 目标

- 列表排序按钮补齐 `aria-sort`
- 抽取 `ariaSortValue` 到 `listSort`
- 补齐 Ops/About 刷新按钮 `aria-busy`

## 实现

- `utils/listSort.ts`：`ariaSortValue(state, key)`
- Audit / AccessGrants / AccessMatrix / Users / Databases 排序按钮 `:aria-sort`
- Operations `ops-refresh` / `ops-status-refresh-stats`、About `about-server-refresh` 加 `aria-busy`

## 验收

- [x] unit 覆盖 `ariaSortValue`
- [x] build / test / e2e / make e2e
