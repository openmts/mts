# Storage Interface Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 基于架构检视报告完成存储层接口化隔离，降低 Shard 对 WAL/MemTable/SSTable/storagefs 具体实现和 public API 对 internal model 的耦合。

**Architecture:** 在 `internal/engine` 消费方定义小接口和默认适配器，Shard 通过端口依赖调用存储组件。根包 public DTO 与 internal model 分离，并在边界做显式转换。Query iterator 改为延迟装饰当前列，保留现有查询语义。

**Tech Stack:** Go、现有 `internal/engine`、`internal/wal`、`internal/memtable`、`internal/sstable`、`internal/storagefs`、`internal/model`、`go test`、`goimports-reviser`、`golangci-lint`。

---

### Task 1: Engine Storage Ports Tests

**Files:**
- Modify: `internal/engine/engine_test.go`

- [x] **Step 1: 增加 Shard 端口注入测试**
  - 添加 fake `partManager`、fake WAL、fake MemTable、fake fileOps`，验证 `OpenShard`、`Flush`、`Compact` 通过注入端口完成关键操作。
- [x] **Step 2: 运行红测**
  - Run: `go test -count=1 ./internal/engine -run 'TestShardUsesInjectedStoragePorts' -timeout 180s`
  - Expected: 编译失败，缺少 `shardDeps`、`partManager` 等端口类型。
  - Result: 按预期失败，缺少端口类型。

### Task 2: Shard Ports And Default Adapters

**Files:**
- Create: `internal/engine/ports.go`
- Modify: `internal/engine/shard.go`
- Modify: `internal/engine/lifecycle.go`
- Modify: `internal/engine/metadata.go`

- [x] **Step 1: 定义 engine 消费方端口**
  - 新增 `walStore`、`memStore`、`memSnapshot`、`partReader`、`partWriter`、`seriesBatchReader`、`partManager`、`fileOps`、`shardDeps`。
- [x] **Step 2: 实现默认适配器**
  - 默认依赖包装 `wal.Open`、`memtable.New`、`sstable.LoadManifest/OpenPart/WritePartWithOptions/NewPartWriter/WriteManifest`、`storagefs.RemoveAll`。
- [x] **Step 3: 改造 Shard 字段和构造**
  - `Shard` 改持有接口字段；`OpenShard` 使用 `normalizeShardDeps` 补齐默认依赖。
- [x] **Step 4: 改造 flush/compaction/retention 删除**
  - `flushLocked`、`openParts`、`cleanupOrphanParts`、`compactPartsLocked`、`compactionOutput` 全部通过端口调用。
- [x] **Step 5: 运行定向测试**
  - Run: `go test -count=1 ./internal/engine -run 'TestShardUsesInjectedStoragePorts' -timeout 180s`
  - Expected: PASS。
  - Result: 通过。

### Task 3: Public DTO Boundary Tests

**Files:**
- Modify: `types_test.go`
- Modify: `engine_test.go`

- [x] **Step 1: 增加 public DTO 非 alias 测试**
  - 验证 `Point`、`Options`、`ColumnSeries` 等 public 类型不再是 internal model 的 alias，并验证 public write/query 行为不变。
- [x] **Step 2: 运行红测**
  - Run: `go test -count=1 . -run 'TestPublicTypesAreIndependentDTOs|TestPublicEngineConvertsDTOs' -timeout 180s`
  - Expected: 失败，因为当前 public 类型仍为 alias。
  - Result: 按预期失败，public 类型仍为 alias。

### Task 4: Public DTO Conversion

**Files:**
- Modify: `types.go`
- Modify: `engine.go`

- [x] **Step 1: 将 public type alias 改为独立 DTO**
  - 定义 public `FieldType`、`FieldValue`、`Point`、`Query`、`Options`、`WALOptions`、`CompactionOptions`、`RetentionPolicy`、`FieldSchema`、`Series`、`CompressionOptions`、`WriteOptions`、`ColumnSeries`、`Row`、iterator 接口。
- [x] **Step 2: 增加转换 helper**
  - 实现 public DTO 到 internal model 的双向转换。
- [x] **Step 3: 改造 public Engine wrapper**
  - `Open`、`Write`、`QueryColumns`、`QueryRows`、metadata API、iterator wrapper 全部使用转换 helper。
- [x] **Step 4: 运行根包测试**
  - Run: `go test -count=1 . -timeout 180s`
  - Expected: PASS。
  - Result: 通过。

### Task 5: Lazy Query Decoration

**Files:**
- Modify: `internal/engine/query.go`
- Modify: `internal/engine/engine_test.go`

- [x] **Step 1: 增加 iterator 延迟装饰测试**
  - 验证 `QueryColumnIterator` 创建 iterator 时不立即调用 decorate hook，只有 `Column()` 时才装饰当前列。
- [x] **Step 2: 运行红测**
  - Run: `go test -count=1 ./internal/engine -run 'TestQueryColumnIteratorDecoratesLazily' -timeout 180s`
  - Expected: 失败，因为当前 iterator 创建时已经构造 `[]ColumnSeries`。
  - Result: 按预期失败，缺少 lazy decoration hook。
- [x] **Step 3: 改造 columnIterator**
  - 保存 raw `[]model.ColumnData` 和 `catalog.Snapshot`，`Column()` 按需调用 `decorateColumnData`。
- [x] **Step 4: 运行定向测试**
  - Run: `go test -count=1 ./internal/engine -run 'TestQueryColumnIteratorDecoratesLazily|TestQuery' -timeout 180s`
  - Expected: PASS。
  - Result: 通过。

### Task 6: Integration Verification

**Files:**
- Modify: `docs/superpowers/plans/2026-06-17-storage-interface-isolation.md`

- [x] **Step 1: goimports-reviser**
  - Run: `timeout 300s goimports-reviser -project-name codeberg.org/mts/mts -recursive -format -rm-unused .`
  - Result: 通过。
- [x] **Step 2: 定向包测试**
  - Run: `go test -count=1 ./internal/engine ./internal/sstable ./internal/wal ./internal/catalog ./internal/memtable . -timeout 180s`
  - Result: 通过。
- [x] **Step 3: 全量测试与覆盖率**
  - Run: `go test -count=1 ./... -coverprofile=coverage.out -timeout 600s`
  - Run: `go tool cover -func=coverage.out | tail -1`
  - Result: 通过；总覆盖率 `90.0%`。
- [x] **Step 4: lint**
  - Run: `golangci-lint run --timeout 12m`
  - Result: 通过；`0 issues`。
- [x] **Step 5: e2e**
  - 逐个 build/run `tests/e2e/*`，每个用例运行后删除二进制。
  - Result: 通过；实际 7 个 e2e 目录全部 build/run 通过。
- [x] **Step 6: 清理产物**
  - 删除 `coverage.out`、profile、e2e/test binary。
  - Result: 完成。

## 自检

- Spec coverage：覆盖 Shard 存储端口、默认适配器、public DTO 解耦、lazy query decoration、验证闭环。
- Placeholder scan：无 TBD/TODO/后续增强占位。
- Type consistency：端口类型集中在 `internal/engine/ports.go`，public DTO 转换集中在根包边界。
