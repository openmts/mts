# Dashboard UX P478（2026-07-23）

## 目标
元数据列表响应与数据面 path/scope 对齐，Query/Databases 可观测服务端路径。

## 范围
- Server：HTTP+gRPC meta list path/scope；契约 meta_list_path
- Dashboard：meta 客户端、Query series path 徽章、清单/e2e
- 不做：SQL parser、分布式、对象存储

## 验收
- [x] series/fields/measurements/databases 含 path
- [x] 契约含 meta_list_path
- [x] Query series 展示 path
- [x] npm test / build / commercial-smoke / Go 定向
