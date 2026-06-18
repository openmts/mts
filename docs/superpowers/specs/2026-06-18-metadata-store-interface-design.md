# 元数据存储接口化设计

## 目标

将当前 `internal/catalog.Catalog` 从 Engine 的直接依赖中抽离出来，作为本地元数据实现 `LocalMetadataStore` 使用。Engine 后续只依赖元数据接口，便于扩展 etcd、ZooKeeper 等外部元数据后端。

## EARS 需求

- 当 Engine 打开本地存储路径时，系统应创建 `LocalMetadataStore` 并保持现有 catalog 目录、WAL、snapshot、metadata 二进制落盘行为不变。
- 当 Engine 写入数据点时，系统应通过 `MetadataStore.ResolvePoints` 解析 seriesID 和 fieldID，而不是直接依赖 `catalog.Catalog`。
- 当 Engine 构建查询计划时，系统应通过元数据接口匹配 seriesID 与 fieldID，保持当前查询过滤语义不变。
- 当 Engine 装饰查询结果时，系统应通过元数据接口获取快照，保持 measurement、tags、field name 还原语义不变。
- 当用户调用 database、retention policy、measurement、field、series 管理 API 时，系统应通过元数据接口转发，并保持现有错误和返回值行为不变。
- 如果后续接入远程元数据后端，Engine 不应需要感知具体后端类型，只需要接收满足接口的实现。

## 架构设计

- 在 `internal/engine` 消费侧定义小而明确的 `MetadataStore` 接口，遵循 Go 的“接口由消费者定义”原则。
- 新增 `LocalMetadataStore`，包装现有 `catalog.Catalog`，作为当前唯一实现。
- Engine 字段从 `*catalog.Catalog` 改为 `MetadataStore`，默认构造逻辑仍创建本地实现。
- `catalog.Catalog` 的二进制持久化格式、目录结构和原有测试不变。

## 边界

- 本次不引入 etcd、ZooKeeper 客户端依赖。
- 本次不改变 seriesID、fieldID 分配算法。
- 本次不迁移已有 `catalog` 包测试，只补充 Engine 侧接口化测试。
- 本次不改变 public API。

## 验证

- 新增测试覆盖 Engine 通过 `LocalMetadataStore` 完成写入、关闭、重启、查询。
- 新增编译期断言，确保 `LocalMetadataStore` 实现 `MetadataStore`。
- 执行 `go test` 定向覆盖 `internal/engine` 和 `internal/catalog`。
- 执行格式化和 lint 检查；若全量 e2e 因耗时无法完整执行，需要明确记录。
