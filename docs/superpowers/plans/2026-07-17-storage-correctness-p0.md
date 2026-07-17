# Storage Correctness P0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** 闭合检视报告中的 P0 正确性缺口（flush 提交状态机、WAL/Mem 失败契约、查询生命周期、Shard 写维护互斥），并在每项修复后通过单元测试、全量 e2e 与性能基准门禁，确认无正确性回归与无显著性能劣化。

**Architecture:** 以 `Shard` 为一致性边界：Manifest 提交后数据权威落在 SSTable part；WAL checkpoint 失败只记维护问题、不回灌 MemTable；写入与 flush 在 `lifecycleMu` 下协调；查询通过读引用计数与关闭语义协作。`Engine.mu` 继续保护 shard map 与全局写序号，但不替代 Shard 级互斥。

**Tech Stack:** Go 1.26、现有 `internal/engine` 测试 fakes、`make test` / `make e2e`、`scripts/storage_benchmark_gate.sh`、golangci-lint / goimports-reviser。

## Global Constraints

- 语言与注释：简体中文
- 单机边界：不引入分布式能力
- 覆盖率：生产包 `>=90%`
- 错误必须接收处理；目录 `0700`、文件 `0600`
- 禁止 for 循环 defer
- 函数 ≤50 行、文件 ≤300 行（本轮优先正确性；若必须改大文件则局部最小改动，后续再拆）
- 每完成一个 task：跑相关单测 → 全量 `make test` → `make e2e` → 性能门禁，并更新本 plan 勾选与备注
- 性能门禁：`scripts/storage_benchmark_gate.sh`；若无 baseline 则先建立本轮 baseline 快照到 `/tmp` 并在修复前后对比；劣化阈值默认 **同 bench 平均 ns/op 恶化 >10% 需解释并优化**

## 文件改动地图

| 文件 | 职责 |
| --- | --- |
| `internal/engine/shard.go` | flush 提交状态机；WriteBatch 生命周期锁；读引用 |
| `internal/engine/shard_scan.go` | 查询 acquire/release 读引用 |
| `internal/engine/metadata.go` / `lifecycle.go` | Drop/retention 与读引用协作 |
| `internal/engine/ports.go` | 如需扩展 mem/wal 接口契约注释 |
| `internal/engine/flush_commit_test.go`（新） | P0-01 测试 |
| `internal/engine/write_atomicity_test.go`（新） | P0-03 测试 |
| `internal/engine/shard_lifecycle_test.go`（新） | P0-02/04 测试 |
| `docs/review/code-review-2026-07-17-2325.md` | 更新问题状态 |
| `docs/storage/recovery-protocol.md` | 补充 Manifest 已提交后 checkpoint 失败语义 |

---

## Task 1: P0-01 Flush checkpoint 失败不回灌 MemTable

**超时预算：** 实现 30m；单测 3m；全量 test 10m；e2e 15m；bench 10m

**EARS：** 当 Manifest 已提交且 WAL Checkpoint 失败时，系统应当保留已提交 part，且不得把 snapshot 回灌 MemTable。

### Steps

- [x] 1.1 写失败测试 `TestFlushCheckpointFailureKeepsCommittedPartWithoutRestoringMem`
  - 使用真实或 fake：Manifest 成功、Checkpoint 返回 error
  - 断言：Flush 返回 error
  - 断言：查询 sample 数 = 写入数（不翻倍）
  - 断言：MemTable sample count 不包含已提交 snapshot（为 0 或仅后续新写）
  - 断言：manifest parts 仍包含新 part
- [x] 1.2 运行该测试，确认当前实现失败（RED）
- [x] 1.3 修改 `flushLocked`：Manifest 提交成功后的失败路径调用 `snapshot.Release()`，**禁止** `Restore(snapshot)`；记录 recovery/maintenance issue
- [x] 1.4 同步修正 `afterManifestBeforeWALTrunc` 失败路径相同语义
- [x] 1.5 运行 Task1 单测至 GREEN
- [x] 1.6 更新 `docs/storage/recovery-protocol.md` 对应段落
- [x] 1.7 验证：
  ```bash
  go test ./internal/engine -count=1 -timeout 180s -run 'TestFlushCheckpoint|TestFlushManifest|TestCompactionManifest'
  make test
  make e2e
  scripts/storage_benchmark_gate.sh /tmp/mts-bench-before.txt /tmp/mts-bench-after-t1.txt || true
  ```
  注：首轮先跑 baseline 到 `/tmp/mts-bench-baseline.txt`，后续 task 对比。
- [x] 1.8 在本文件勾选并写实现备注

---

## Task 2: P0-03 WAL 成功 MemTable 失败契约

**超时预算：** 实现 25m；验证同 Task1

**EARS：** 当 WAL append 成功但 MemTable apply 失败时，系统应当保证不丢 durable 数据，并返回明确错误；进程内应尽力恢复 mem 可见性。

### Steps

- [x] 2.1 写测试 `TestWriteBatchMemApplyFailureAfterWALRemainsDurableAndVisibleOrRecoverable`
  - fake WAL append 成功；fake mem 第一次 ApplyBatch 失败、第二次成功（或可注入）
  - 期望：实现采用“失败后立即重试 apply 一次 / 或标记需恢复”
  - 最小契约：返回 error 时数据仍在 WAL；成功路径 mem 可见
- [x] 2.2 RED
- [x] 2.3 实现 `WriteBatch`/`WriteTypedBatch`：WAL 成功后 mem 失败则重试 apply；仍失败则记录 issue 并返回 `fmt.Errorf("memtable apply after wal: %w", err)`，**不**回滚已写 WAL（durable 优先）
- [x] 2.4 GREEN + 回归 atomicity 测试
- [x] 2.5 全量 `make test` + `make e2e` + bench 对比
- [x] 2.6 勾选备注

---

## Task 3: P0-04 Write 与 flush/scan 的 Shard 互斥

**超时预算：** 实现 40m；验证 30m

**EARS：** 当写、flush、scan 并发时，Shard 应有明确 `lifecycleMu` 语义，不依赖“永远持有 Engine.mu”作为唯一正确性来源。

### Steps

- [x] 3.1 写测试 `TestShardWriteFlushAndScanDoNotRaceOnMemSnapshot`
  - 直接对 Shard：并发 WriteBatch + ScanColumns + 触发 Flush
  - 在 `-race` 下运行
- [x] 3.2 RED/或不稳定
- [x] 3.3 `WriteBatch`/`WriteTypedBatch`/`DeleteRange` 进入时获取与 flush 兼容的锁：
  - 方案：写路径 `lifecycleMu.Lock()` 覆盖 wal+mem+maybeFlush（与 Flush 一致）
  - Scan 继续 `RLock`；写与 flush 互斥，多读并行
- [x] 3.4 确认 `Flush` 外层锁与 `WriteBatch` 内层 `Flush()` 不重入死锁：
  - `WriteBatch` 达阈值时调用 `flushLocked()` 而非 `Flush()`（已持锁）
- [x] 3.5 `go test -race ./internal/engine -count=1 -timeout 300s -run 'TestShardWrite|TestEngineConcurrent|TestFlush'`
- [x] 3.6 全量 test/e2e/bench
- [x] 3.7 勾选备注

---

## Task 4: P0-02 查询读引用与关闭/删除安全

**超时预算：** 实现 40m；验证 30m

**EARS：** 当 iterator 仍持有 shard 读引用时，Close/Drop/retention 应等待或返回明确错误，不得 use-after-close。

### Steps

- [x] 4.1 写测试 `TestQueryIteratorBlocksDropDatabaseUntilClosed` 与 `TestDropDatabaseAfterIteratorCloseSucceeds`
- [x] 4.2 在 `Shard` 增加 `readRefs int`（由 `lifecycleMu` 保护）
  - `acquireRead` / `releaseRead`
  - `ScanColumns` acquire；stream Close release
  - `closeLocked` 要求 `readRefs==0`，否则返回 `ErrShardBusy`（或等待短超时——优先明确错误避免挂死）
- [x] 4.3 `DropDatabase`/`ApplyRetention` 在 Close 失败时不删目录，返回错误
- [x] 4.4 GREEN + race
- [x] 4.5 全量 test/e2e/bench
- [x] 4.6 勾选备注

---

## Task 5: 收尾

- [x] 5.1 更新 `docs/review/code-review-2026-07-17-2325.md` 中 P0 状态为已处理
- [x] 5.2 `make test` + `make e2e` + bench 最终对比报告写入 review 备注
- [x] 5.3 `goimports-reviser` + `golangci-lint` 定向包
- [x] 5.4 清理临时二进制/coverprofile

---

## 性能观测方法

```bash
# baseline（实现前或 Task1 前）
scripts/storage_benchmark_gate.sh --update-baseline /tmp/mts-bench-baseline.txt /tmp/mts-bench-run.txt

# 每 task 后
scripts/storage_benchmark_gate.sh /tmp/mts-bench-baseline.txt /tmp/mts-bench-taskN.txt
```

关注：
- `BenchmarkEngineWriteBatch/points=10000`
- `BenchmarkEngineWriteWideBatch/points=10000`
- `BenchmarkEngineQueryRowIterator/points=10000`
- `BenchmarkEngineQueryColumnIterator/points=10000`

若 benchstat 显示平均恶化 >10%，先分析是否锁粒度过粗，再优化（例如写路径分段锁：wal+mem 用更细锁，仅 flush 升级）。

---

## 实现备注区



## 实现备注区

### 2026-07-18 执行结果

- Task1 P0-01：`completeCommittedFlush`，Manifest 提交后不回灌 MemTable；新增 `RecoveryIssueWALCheckpointFailed`。
- Task2 P0-03：`applyMemAfterWAL` / `applyTypedMemAfterWAL` 重试 1 次；失败记 `RecoveryIssueMemApplyFailed`，不回滚 WAL。
- Task3/4 P0-04/02：写路径 `lifecycleMu.RLock`；扫描 `readRefs`；`Close`/`DropDatabase`/`ApplyRetention` 在活跃读时返回 `ErrShardBusy`；Engine 关闭走 `closeForced`；活跃读时跳过会删 part 的 compact。
- 附带修复：`cmd/mts-server/production_hardening_test.go` 变量 `runtime` 遮蔽标准库导致编译失败（`goruntime` 别名）。
- 验证：
  - `go test ./internal/engine`（含新增用例）通过
  - `go test -race` 关键生命周期/写路径通过
  - `make test` 通过（含 e2e/fault/scale 包）
  - `make e2e` 通过
  - bench 对比见 `/tmp/mts-bench-compare.log`（相对 Task1 后 baseline）

