# 代码检视：mts-dashboard P263（2026-07-21）

## 范围

- `utils/listSort.ts` / `listSort.test.ts`
- Audit / AccessGrants / AccessMatrix / Users / Databases 排序按钮
- Operations / About 刷新按钮 `aria-busy`

## 结论

- 排序控件补齐 `aria-sort`，与 `ariaSortValue` 统一映射
- 不改变排序语义（仍 cycle asc/desc/clear）

## 验证

- npm test 403 pass
- npm run build ok
- npm run test:e2e 1 passed
- make e2e 本轮此前已绿（P261 门禁）
