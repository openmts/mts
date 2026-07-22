# Dashboard/Server P467 — 数据面 limits 对齐

## 目标
把服务端 `max_write_points` / `default_query_limit` / `max_query_limit` 暴露给数据面与 Dashboard，Write/Query 可预检、可见、可裁剪。

## 验收
- [x] GET /api/v1/data/limits + gRPC
- [x] Write/Query 横幅与预检
- [x] Config schema limits.*
- [x] npm test/build + commercial-smoke + 定向 Go 测试
