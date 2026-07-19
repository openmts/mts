# mts-server P0 协议补齐 + P1 配置透出

日期：2026-07-19 16:47  
状态：**已实现**

## P0 协议

| 能力 | 变更 |
|---|---|
| `ListDatabases` | 新增 gRPC；返回 `{databases:[]}` |
| `DropDownsamplePolicy` | gRPC 请求改为 `grpcDownsampleDropRequest{name,options}`，内部走 `DropDownsamplePolicyWithOptions`；兼容旧 name-only |
| `AdminHealth` | 新增 gRPC，返回 `HealthSnapshot` |

## P1 配置透出

`engine` YAML / `engineOptions()` 新增映射：

- 并发：`max_concurrent_compaction` / `max_concurrent_downsample` / `compaction.max_concurrent`
- 压缩：`value_page_samples` / `omit_write_seq` / `zstd_level`
- WAL：`sync` / `segment_bytes` / `batch_*`
- 查询缓存：`query_page_cache` / `query_block_cache`
- 保护：`query_protection` / `cardinality` / `storage_memory`
- 乱序 flush：`memtable_disorder_flush_ratio` / `min_samples`

示例配置：`configs/mts-server.yaml`  
契约字段：`/admin/config/schema` 同步补充关键项。

## 验证

- [x] `go test ./cmd/mts-server`
- [x] `make test`
- [x] `make e2e`
- [x] `make lint`
