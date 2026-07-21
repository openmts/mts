# Dashboard UX P231（2026-07-21）

## 目标
查询取消路径 e2e：延迟 mock + 取消按钮 + 文案/样式/loading 恢复。

## EARS
- [x] EARS-E2E-P231-01 查询进行中 `query-cancel` 可点
- [x] EARS-E2E-P231-02 取消后 `query-action-error` 为 `queryCancelled` 且 `mts-alert-info`
- [x] EARS-E2E-P231-03 取消后 `query-run` 恢复可点、`query-cancel` disabled
- [x] EARS-E2E-P231-04 mock 必须 `unroute`，不污染后续用例

## 非目标
- 真实 API 超时 e2e（超时依赖构建期 `VITE_API_TIMEOUT_MS`）
- 宣称可商用完成
