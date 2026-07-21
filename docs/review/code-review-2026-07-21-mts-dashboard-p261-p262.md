# 代码检视：mts-dashboard P261–P262（2026-07-21）

## 范围

- `cmd/mts-dashboard/src/pages/OverviewPage.vue`
- `cmd/mts-dashboard/src/i18n/messages.ts`
- `cmd/mts-dashboard/e2e/commercial-smoke.spec.ts`

## 结论

- **P261 已实现**：Overview 自动刷新与 Metrics soft-fail 对齐；health 失败保留快照，分项 soft 失败不清空已成功数据。
- **P262 轻量完成**：自动刷新 `aria-pressed`、手动刷新 `aria-busy`/testid。

## 问题清单

| ID | 状态 | 说明 |
|----|------|------|
| R1 | fixed | 自动刷新 hard `loadOverview` 可能整页错误态盖数据 |
| R2 | fixed | 缺 `overview-auto-refresh` / refresh-error testid |
| R3 | open | 全站列表排序按钮仍缺统一 `aria-sort`（非本轮目标） |

## 验证

见提交前 `npm test` / `npm run build` / `npm run test:e2e` 与 `go test` / `make e2e`。
