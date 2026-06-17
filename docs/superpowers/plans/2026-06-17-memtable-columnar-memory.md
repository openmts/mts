# MemTable Columnar Memory Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 隔离 pprof 输入造数内存与引擎内存，并把 MemTable 改为按列分类型驻留结构。

**Architecture:** pprof 工具增加阶段性内存快照日志；MemTable 内部 `columnBuffer` 由 `first + []VersionedSample` 改为 timestamp/writeSeq/value typed slices。Engine、SSTable、public API 继续使用现有 `ColumnData` 契约，降低改动面。

**Tech Stack:** Go 1.26.2、标准库 runtime/memstats、现有 mts engine/memtable/sstable。

---

### Task 1: pprof 阶段指标隔离

**Files:**
- Modify: `tests/pprof/storage_engine/main.go`
- Modify: `tests/pprof/storage_engine/main_test.go`

- [x] **Step 1: 增加 stage metrics helper**

新增 `logStageMetrics(stage string, dir string) error`，在 prebuild 后、workload 前、workload 后、mem profile 后输出 heap/RSS/data bytes。

实现备注：已新增 `after_prebuild`、`before_workload`、`after_workload`、`after_profile` 阶段日志；`collectRunMetrics("")` 支持只采 runtime/RSS，用于隔离造数内存基线。

- [x] **Step 2: 覆盖 prebuild 和 read mode 测试**

扩展 pprof 测试，确认 prebuild/read/compression 参数均可运行。

实现备注：已覆盖 `-prebuild-points`、`-compression-algorithm` 和只读 `read` 模式。

**验收:** `go test -count=1 ./tests/pprof/storage_engine -timeout 180s` 通过。

### Task 2: MemTable typed columnar buffer

**Files:**
- Modify: `internal/memtable/memtable.go`
- Modify: `internal/memtable/memtable_test.go`
- Modify: `internal/memtable/internal_test.go`

- [x] **Step 1: 替换 columnBuffer 字段**

把 `first` 和 `samples` 替换为 timestamps/writeSeqs/floats/ints/strings/bools/count，并实现 typed append。

实现备注：`columnBuffer` 已改为 `times/writeSeqs/floats/ints/strings/boolBits/count`，bool 使用 bitset，string 保留字符串引用，数值列使用紧凑 typed slice。

- [x] **Step 2: 实现 reserve 和 restore**

按字段类型预扩容 timestamp/writeSeq/value slices；Snapshot restore 按 typed arrays 追加。

实现备注：`ApplyBatch` 按列统计 reservation，restore 复用 typed append，Snapshot release 清空 typed slices，避免保留大 backing array。

- [x] **Step 3: 实现 materialize**

query/flush 时按 query 范围生成 `[]VersionedSample`，保持排序、去重和 writeSeq 语义。

实现备注：flush/query 边界继续输出 `ColumnData`；有序唯一列使用二分范围生成紧容量样本切片，乱序/重复列按匹配数量预分配并保持 LWW 语义。

**验收:** `go test -count=1 ./internal/memtable -timeout 180s` 通过。

### Task 3: 集成验证

**Files:**
- Modify as needed: `internal/engine/*`

- [x] **Step 1: 跑 engine/sstable 相关测试**

运行 `go test -count=1 ./internal/engine ./internal/sstable -timeout 240s`。

实现备注：已包含在全量 `go test -count=1 ./... -coverprofile=coverage.out -timeout 600s` 中，`internal/engine` 与 `internal/sstable` 均通过。

- [x] **Step 2: 跑全量测试、lint、e2e**

运行 goimports-reviser、`go test ./...`、golangci-lint、逐个 e2e build/run。

实现备注：`goimports-reviser` 退出码 0；全量测试通过，总覆盖率 90.0%；`golangci-lint run --timeout 12m` 输出 `0 issues.`；`tests/e2e/*` 逐个 build/run 通过。

### Task 4: 性能复测

**Files:**
- No repository output; use `/tmp`.

- [x] **Step 1: 复测 no-prebuild 100K wide10**

运行无 prebuild、`memtable-max-samples=1200000` 和 `100000` 两组写入，记录 RSS peak。

实现备注：
- `memtable-max-samples=1200000`、无 prebuild：RSS peak 134,914,048 bytes，写入 536 ms，1 个 SSTable。
- `memtable-max-samples=100000`、无 prebuild：RSS peak 67,817,472 bytes，写入 720 ms，9 个 SSTable。
- `memtable-max-samples=1200000`、prebuild：`after_prebuild` RSS 180,629,504 bytes，workload 后 RSS peak 400,183,296 bytes，写入 487 ms，1 个 SSTable。

- [x] **Step 2: 清理临时产物**

删除 `/tmp/mts-storage-pprof`、`/tmp/mts-rss-*`、coverage/profile/e2e testbin。

实现备注：最终收尾阶段清理 `/tmp/mts-storage-pprof`、`/tmp/mts-rss-columnar`、`coverage.out` 和 e2e `testbin`。
