# Metadata Store Interface Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将存储层元数据系统接口化，并把当前本地 catalog 实现命名为 `LocalMetadataStore`。

**Architecture:** Engine 定义并消费 `MetadataStore` 接口。`LocalMetadataStore` 包装现有 `catalog.Catalog`，保留本地二进制持久化行为。查询计划、写入解析、metadata 管理 API 均通过接口访问。

**Tech Stack:** Go、`internal/catalog`、`internal/engine`、二进制 catalog WAL/snapshot/metadata。

---

### Task 1: 规格与调用面确认

**Files:**
- Create: `docs/superpowers/specs/2026-06-18-metadata-store-interface-design.md`
- Create: `docs/superpowers/plans/2026-06-18-metadata-store-interface.md`
- Read: `internal/engine/engine.go`
- Read: `internal/engine/query_plan.go`
- Read: `internal/engine/query.go`
- Read: `internal/engine/metadata.go`
- Read: `internal/catalog/catalog.go`

- [x] **Step 1: 写入 EARS 规格**

实现备注：已明确本次只做接口化和本地实现命名，不改变落盘格式、不引入远程依赖。

- [x] **Step 2: 枚举 Engine 对 catalog 的直接依赖**

实现备注：直接依赖集中在 Engine 打开/关闭、写入解析、查询计划、查询装饰、metadata API。

### Task 2: 测试先行

**Files:**
- Modify: `internal/engine/metadata_store_test.go`

- [x] **Step 1: 新增本地元数据实现持久化测试**

测试应覆盖：通过 Engine 写入数据、关闭、重启后仍可通过查询读出，并且 Engine 使用的是 `LocalMetadataStore`。

实现备注：新增 `TestEngineUsesLocalMetadataStoreAcrossRestart`，覆盖本地实现类型、写入、关闭、重启和查询。

- [x] **Step 2: 运行红灯测试**

Run: `timeout 180s go test ./internal/engine -run TestEngineUsesLocalMetadataStoreAcrossRestart -count=1`

Expected: 初次运行因 `LocalMetadataStore` 或接口断言缺失失败。

实现备注：红灯失败原因为 `eng.metadata undefined`，符合接口化测试预期。

### Task 3: 定义接口与本地适配器

**Files:**
- Create: `internal/engine/metadata_store.go`

- [x] **Step 1: 定义 `MetadataStore` 接口**

接口包含写入解析、查询匹配、快照、database/rp/measurement/field/series 管理、关闭方法。

实现备注：定义 `MetadataResolver`、`MetadataQuerier`、`MetadataManager` 小接口，并组合为 `MetadataStore`。

- [x] **Step 2: 实现 `LocalMetadataStore`**

`LocalMetadataStore` 包装 `*catalog.Catalog`，方法转发到现有本地实现。

实现备注：`OpenLocalMetadataStore` 保持原 catalog 目录和二进制持久化行为不变，方法转发前检查 `context.Context`。

- [x] **Step 3: 增加编译期断言**

`var _ MetadataStore = (*LocalMetadataStore)(nil)`

实现备注：已增加编译期断言。

### Task 4: Engine 依赖倒置

**Files:**
- Modify: `internal/engine/engine.go`
- Modify: `internal/engine/query_plan.go`
- Modify: `internal/engine/query.go`
- Modify: `internal/engine/metadata.go`

- [x] **Step 1: 将 Engine 字段改为接口**

`catalog *catalog.Catalog` 改为 `metadata MetadataStore`。

实现备注：`Engine` 当前持有 `metadata MetadataStore`。

- [x] **Step 2: Open 默认创建 `LocalMetadataStore`**

保持 `catalogDir(opts.Path)` 和本地落盘行为不变。

实现备注：`Open` 默认调用 `OpenLocalMetadataStore(catalogDir(opts.Path))`。

- [x] **Step 3: 替换所有 `e.catalog` 调用**

写入、查询计划、查询装饰和 metadata API 全部改为 `e.metadata`。

实现备注：`rg` 未发现剩余 `e.catalog` 直接调用。

### Task 5: 验证与收尾

**Files:**
- Modify: `docs/superpowers/plans/2026-06-18-metadata-store-interface.md`

- [x] **Step 1: gofmt/goimports-reviser**

Run: `timeout 300s goimports-reviser -rm-unused -set-alias -format ./...`

Result: 通过。

- [x] **Step 2: 定向测试**

Run: `timeout 180s go test ./internal/catalog ./internal/engine -count=1`

Result: 通过。

- [x] **Step 3: lint**

Run: `timeout 720s golangci-lint run ./...`

Result: 通过，输出 `0 issues.`。

- [x] **Step 4: e2e 验证并清理产物**

Run: `find tests/e2e -mindepth 1 -maxdepth 1 -type d | sort` 后逐目录执行 `go build` 和二进制运行。

Result: 当前 12 个 e2e 目录全部通过，构建产物已清理。

- [x] **Step 5: 更新计划状态**

记录已完成验证结果和无法执行的验证原因。

实现备注：已记录定向测试、全量 Go 测试、lint 和 e2e 验证结果。
