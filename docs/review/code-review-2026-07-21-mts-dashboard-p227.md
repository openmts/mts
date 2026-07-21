# Code Review — mts-dashboard P227（2026-07-21）

## 范围
超时/取消可见反馈：`useExportJob`、`apiError`、`useQueryWorkbench`、Query/Write 页。

## 问题与处理

| ID | 问题 | 状态 |
|----|------|------|
| R-P227-01 | 导出失败裸 `Error.message` | **已修复** `formatCaughtError` |
| R-P227-02 | `AbortError` 被 `formatCaughtError` 当成 timeout | **已修复** → canceled |
| R-P227-03 | `cancelQuery` 推进 `requestSeq` 导致 finally 不清 loading | **已修复** 用户取消仅 abort |
| R-P227-04 | Query 取消走 error toast，与 Write 不一致 | **已修复** `queryCancelled` + success toast |
| R-P227-05 | Write 取消判定分散 | **已修复** `isCanceledError` |

## 验证计划
- `npm test` / `npm run build` / `npm run test:e2e`
- `go test ./...` / `make e2e`
