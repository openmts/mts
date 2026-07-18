# Storage P2-04 Protocol Registry Closure

**Goal:** 将 mts-server HTTP/gRPC 固定操作收敛到统一 operation registry，避免双协议与 API spec 漂移。

## Tasks
- [x] 设计 operation 结构（name/namespace/http/grpc/auth/desc）
- [x] 实现 operationCatalog + mountRegistryHTTP + grpcMethodsFromRegistry + apiSpecFromRegistry
- [x] 迁移 httpHandler / grpcServiceDesc / apiSpec
- [x] 完整性测试
- [x] make test/e2e/lint

## 实现备注
- 文件：`cmd/mts-server/operation_registry.go`、`operation_registry_test.go`
- 复杂 prefix handler（users/downsample/databases）仍复用原 handler，但挂载入口统一登记
- 新增固定 API 的推荐流程：先登记 registry，再实现 handler，再补测试
