# gRPC 流式查询 + Dashboard 能力补齐

日期：2026-07-19 16:21  
状态：**已实现**

## 目标

1. 为 `mts-server` 补齐 gRPC **server-streaming** 查询，对齐 HTTP NDJSON stream 的 row/column 能力。
2. Dashboard 接入列式流、按范围删除、维护统计，以及 points-typed 写入入口。

## 实现

### gRPC

- 方法：`QueryStream`（server stream）
- 请求：`queryStreamRequest{query, format|mode}`，`format=row|column`
- 响应消息：`streamRecord{type,row,column,error,stats}`，与 HTTP NDJSON 语义一致
- 实现：`cmd/mts-server/grpc_stream.go`，注册到 `ServiceDesc.Streams`
- 测试：`TestGRPCQueryStreamRowAndColumn`

### Dashboard

| 页面 | 能力 |
|---|---|
| Query | 模式拆为“流式行 / 流式列”；新增“按范围删除”调用 `/api/v1/data/delete` |
| Write | 默认开启 points-typed（`/api/v1/data/write/points-typed`） |
| Operations | 新增维护统计拉取 `/api/v1/admin/stats/maintenance` |

### 构建

- 升级 `@vitejs/plugin-vue` 至 `^6.0.8` 以兼容 vite 7 peer
- 重建并嵌入 `cmd/mts-server/dashboard-dist`（权限 0700/0600）

## 验证

- [x] `go test ./cmd/mts-server`
- [x] `make test`
- [x] `make e2e`
- [x] `make lint`
- [x] dashboard `npm run build`
