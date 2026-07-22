# Dashboard/Server P465 — 查询响应 meta 对齐

## 目标
补齐 query/rows|columns|explain 契约中已声明但未落地的 admin_op/path/count，Dashboard 查询台同步消费。

## 验收
- [x] 三查询路径返回 path + count + admin_op 字段
- [x] gRPC 与 HTTP 一致
- [x] Dashboard 刷新 admin-op；columns/explain 导出含 path
- [x] 单元测试 + commercial-smoke + 定向 Go 测试
