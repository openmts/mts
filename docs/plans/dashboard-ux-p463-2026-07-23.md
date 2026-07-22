# Dashboard/Server P463 — 错误码可操作契约对齐

## 目标
拉齐 mts-server 与 mts-dashboard 的 error-codes 商用运维面：契约含可重试/处置建议/深链，错误响应透传，Query/Write/Config 可操作。

## 范围
- Server：`errorCodeSpec`、`errorResponse`、`apiErrorResponse`
- Dashboard：Config 表、api client、apiError、Query/Write 横幅、清单/命令面板/e2e

## 非目标
- refresh token、跨主机备份、SQL parser
- 宣称可商用完成

## 验收
- [x] error-codes 含 retryable/category/remediation/dashboard_path
- [x] 错误 JSON 附带 remediation（主码映射）
- [x] Config 表展示并可筛选
- [x] Query/Write 深链对照
- [x] npm test / build / commercial-smoke + Go 单测

## 备注
- POC 阶段无兼容包袱，直接最终契约字段
