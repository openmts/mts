# Storage Interface Isolation Design

## 目标

基于 `docs/review/code-review-2026-06-17-0843.md` 的架构检视结果，对 mts 存储层做一次最小侵入的接口化隔离。目标不是把所有实现都抽象成接口，而是在需求变化最容易扩散的 engine/shard 组装层建立消费方端口，让 WAL、MemTable、SSTable Part/Manifest、文件操作、public API DTO 与内部模型之间的边界更清晰。

## EARS 需求清单

- 当 Shard 需要访问 WAL 时，系统应通过 engine 消费方定义的小接口调用 append、replay、checkpoint、close，不应在 Shard 业务逻辑中强依赖 `*wal.Log` 字段类型。
- 当 Shard 需要访问 MemTable 时，系统应通过 engine 消费方定义的小接口调用 apply、query、snapshot/restore，不应在 Shard 业务逻辑中强依赖 `*memtable.MemTable` 字段类型。
- 当 Shard 需要加载 manifest、打开 part、写 part、写 streaming compaction part、提交 manifest 时，系统应通过 `partManager` 适配器完成，不应把 SSTable 具体函数散落在 flush 和 compaction 逻辑中。
- 当 Shard 需要删除目录或清理 orphan part 时，系统应通过最小 `fileOps` 端口调用删除能力，便于后续故障注入或文件系统适配。
- 当默认生产路径启动时，系统应使用包装现有 WAL、MemTable、SSTable、storagefs 的默认实现，保持现有持久化格式和行为不变。
- 当测试需要验证接口隔离时，系统应允许注入 fake `partManager`、fake WAL、fake MemTable 或 fake fileOps，而不需要新增全局 hook。
- 当根包暴露公共类型时，系统应使用独立 DTO 类型，并在 public engine 边界转换为 internal model，避免 public API 直接 alias 内部存储模型。
- 当查询返回 ColumnIterator 时，系统应延迟把 internal `ColumnData` 装饰为 public/internal `ColumnSeries`，不应在创建 iterator 时提前构造完整 `[]ColumnSeries`。
- 如果接口适配或转换过程中遇到错误，系统应保留原有错误返回语义，不应 panic 或静默忽略。
- 如果接口化改造完成，系统应通过定向测试、全量测试、覆盖率、lint、e2e 验证，并清理所有临时产物。

## 设计

在 `internal/engine` 新增 `ports.go`，只在消费方定义接口：`walStore`、`memStore`、`memSnapshot`、`partReader`、`partWriter`、`seriesBatchReader`、`partManager`、`fileOps`。这些接口只包含 Shard 当前真实使用的方法，避免过早抽象。`defaultShardDeps` 包装现有 `wal.Open`、`memtable.New`、`sstable.*`、`storagefs.*`，生产路径不改变持久化格式。

`Shard` 改为持有接口字段：WAL 使用 `walStore`，MemTable 使用 `memStore`，Parts 使用 `[]partReader`，SSTable/Manifest 操作通过 `partManager`，目录删除通过 `fileOps`。`ShardOptions` 增加未导出的 `deps shardDeps`，`OpenShard` 自动补齐默认依赖。测试可通过同包构造 options 注入 fake。

Compaction 保持当前 streaming 与分批输出行为，但 `compactionInput`、`compactionOutput` 改为面向 `partReader`、`partWriter` 和 `partManager`。需要使用 `SeriesBatchReader` 时由 `partManager` 负责创建，避免 engine 直接了解具体 `*sstable.Part`。

根包 `types.go` 将 type alias 改为独立 DTO。新增转换 helper，将 public DTO 转为 `internal/model`，并把内部查询结果转回 public DTO。这样内部 `ResolvedPoint`、`ColumnData`、`VersionedSample` 后续可以独立演进。

`internal/engine/query.go` 保持对内部 model 的查询接口，但 `columnIterator` 改为保存 raw `[]model.ColumnData` 和 catalog snapshot，在 `Column()` 调用时按需 decorate 当前列，避免创建 iterator 时一次性构造所有 `ColumnSeries`。

## 非目标

- 不改变 WAL、SSTable、Catalog 的磁盘格式。
- 不引入独立 DI 框架。
- 不把每个 helper 函数都改成接口。
- 不在本轮重写为真正跨 shard/page 的 lazy streaming 查询执行器；本轮建立 iterator 延迟装饰边界，后续可以在同一接口下替换底层数据来源。

## 验证

- `go test -count=1 ./internal/engine -run 'TestShardUsesInjectedStoragePorts|TestQueryColumnIteratorDecoratesLazily' -timeout 180s`
- `go test -count=1 ./... -coverprofile=coverage.out -timeout 600s`
- `go tool cover -func=coverage.out | tail -1`，总覆盖率不低于 `90.0%`
- `timeout 300s goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .`
- `golangci-lint run --timeout 12m`
- 逐个 build/run `tests/e2e/*` 并删除二进制。

## 自检

- Placeholder scan：无 TBD/TODO/后续增强占位。
- Scope：聚焦 engine 消费方端口、public DTO 解耦和 iterator 装饰边界，不做无关重构。
- Type consistency：接口均定义在 `internal/engine` 消费方，默认实现包装现有具体包。
