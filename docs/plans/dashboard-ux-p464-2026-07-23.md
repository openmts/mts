# Dashboard/Server P464 — 写入 accepted points 对齐

## 目标
写路径成功响应返回服务端 accepted `points` 与 `path`，Dashboard 成功提示不再只靠客户端本地估算。

## 验收
- [x] write / write/typed / write/points-typed 响应含 points/path
- [x] Dashboard 优先展示服务端计数
- [x] 契约 ResponseHint 更新
- [x] 单元测试 + commercial-smoke + 定向 Go 测试
