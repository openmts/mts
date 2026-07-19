# mts-server API 能力补齐

日期：2026-07-19 16:08  
状态：**已实现**

## 总览：Engine 公开能力 vs mts-server

| Engine 能力 | 补齐前 | 补齐后 |
|---|---|---|
| QueryRowIterator | HTTP NDJSON stream(row only) | 保留 row |
| QueryColumnIterator | **缺失**（常量 streamTypeColumn 未用） | HTTP stream `format=column` |
| QueryColumns / QueryRows / Explain / Stats | 已有 | 已有 |
| Write / WriteTypedBatch | 已有 | 已有 |
| WritePointsAsTypedBatch | **缺失** | HTTP+gRPC |
| Delete | **缺失** | HTTP+gRPC |
| MaintenanceStatsSnapshot | **缺失** | HTTP+gRPC |
| Compact/Flush/Retention/Downsample/Users/... | 已有 | 已有 |

说明：gRPC 仍为 unary JSON codec，大结果优先 HTTP NDJSON 流；未引入 gRPC streaming 以保持现有协议栈简单。

## 新增/增强 API

### 数据面

1. `POST /api/v1/data/query/stream`
   - body: `{ "query": {...}, "format": "row"|"column" }`（`mode` 兼容别名）
   - NDJSON：`type=row|column|error|end`
   - 打开 iterator 失败时仍返回 HTTP 4xx（与历史错误测试一致）
2. `POST /api/v1/data/write/points-typed` + gRPC `WritePointsAsTypedBatch`
3. `POST /api/v1/data/delete` + gRPC `Delete`

### 管理面

4. `GET /api/v1/admin/stats/maintenance` + gRPC `MaintenanceStats`

## 验证

- [x] `go test ./cmd/mts-server`
- [x] registry 覆盖新增 gRPC method / HTTP path
- [x] HTTP 覆盖 column stream / points-typed / delete / maintenance stats

## 后续可选（本轮不做）

- gRPC server-streaming 列/行导出
- Dashboard 前端接入 column stream 与 delete 表单
- HTTP write 路径自动启发式选择 typed 转换
