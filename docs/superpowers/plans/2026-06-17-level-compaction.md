# Level Compaction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 mts 存储层实现可配置的 L0/L1/L2 leveled compaction，并保持 streaming、原子 manifest 替换和逐层压缩配置。

**Architecture:** 在 public/model options 中增加逐层配置；在 engine 中新增归一化后的 level options 和 planner；executor 继续复用 per-series streaming writer，但按输出 level 选择 roll 大小与 compression。测试覆盖配置、planner、级联、重启和 pprof 参数。

**Tech Stack:** Go、现有 `internal/engine`、`internal/sstable`、`tests/e2e`、`tests/pprof/storage_engine`。

---

### Task 1: 配置模型与归一化

**Files:**
- Modify: `types.go`
- Modify: `internal/model/types.go`
- Modify: `internal/engine/paths.go`
- Modify: `internal/model/types_test.go`

- [x] 增加 `CompactionLevelOptions` public/model 类型，字段为 `Level`、`PartLimit`、`SizeLimit`、`MaxOutputPartBytes`、`Compression`。
- [x] 在 `CompactionOptions` 中增加 `Levels []CompactionLevelOptions` 和 `MaxCascadeSteps int`。
- [x] 更新 `toModelCompactionOptions`，完整拷贝逐层配置和压缩配置。
- [x] 在 `normalizeOptions` 中构建默认 L0 配置，并对显式 levels 排序、补默认值。
- [x] 增加测试：旧字段能生成 L0 配置；显式 levels 能排序并继承全局输出大小和 compression。

实现备注：`types.go`、`internal/model/types.go`、`internal/engine/paths.go` 已完成配置扩展与默认归一化，`TestNormalizeCompactionLevelsSortsAndInheritsDefaults` 覆盖显式 level 排序、输出大小继承和 compression 继承。

### Task 2: Planner 抽象

**Files:**
- Create: `internal/engine/compaction_planner.go`
- Modify: `internal/engine/lifecycle.go`
- Modify: `internal/engine/engine_test.go`

- [x] 新增 `compactionPlan`，包含 `Level`、`OutputLevel`、`Candidates`、`OutputOptions`。
- [x] 新增 `nextCompactionPlan(parts []sstable.PartMeta, tombstones []model.Tombstone, opts model.CompactionOptions)`。
- [x] 从低到高扫描归一化 level 配置，按 part limit 或 size limit 触发。
- [x] 删除 L0-only planner 调用路径，保留旧函数测试可覆盖的行为迁移到新 planner。
- [x] 增加测试：L0 超 part limit 输出 L1；L1 超 size limit 输出 L2；未超限不输出计划。

实现备注：新增 `internal/engine/compaction_planner.go` 和对应测试，原 `level0CompactionCandidates` 路径已替换为通用 planner。

### Task 3: 级联执行与逐层输出配置

**Files:**
- Modify: `internal/engine/lifecycle.go`
- Modify: `internal/engine/engine_test.go`

- [x] 将 `compactPartsLocked` 改为接收 `compactionPlan` 或 output options。
- [x] `compactionOutput` 按 plan 使用 `MaxOutputPartBytes` 和 `Compression`。
- [x] `maybeCompactLocked` 执行 bounded cascade，执行成功后重新规划下一层。
- [x] `Compact()` flush 后执行 drain，直到没有计划或达到 `MaxCascadeSteps`。
- [x] 增加测试：多批 L0 flush 后自动级联到 L2，查询结果保持 LWW 正确。

实现备注：`TestLevelCompactionCascadesToLevelTwo` 覆盖真实写入触发 L0->L1->L2；manual `Compact()` 保留全量压缩兜底，避免破坏原行为。

### Task 4: pprof 与 e2e 暴露

**Files:**
- Modify: `tests/pprof/storage_engine/main.go`
- Modify: `tests/pprof/storage_engine/main_test.go`
- Modify: `tests/pprof/README.md`
- Modify: `tests/e2e/compaction_integrity/main.go`

- [x] pprof 增加 `-compaction-levels` 参数，格式为 `level:part_limit:size_limit:max_output_bytes`，支持多个逗号分隔。
- [x] pprof 增加 `-compaction-max-cascade-steps` 参数。
- [x] e2e compaction integrity 验证 manifest 中至少出现 L2 Part，并验证查询数据完整。
- [x] README 增加 level 参数示例。

实现备注：`tests/pprof/storage_engine` 已覆盖参数解析、非法 spec、storage options 转换；`tests/e2e/compaction_integrity` 会读取 manifest 校验 L2 Part。

### Task 5: 验证与收尾

**Files:**
- Modify: `docs/superpowers/plans/2026-06-17-level-compaction.md`

- [x] 运行 `goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .`。
- [x] 运行 `go test -count=1 ./... -coverprofile=coverage.out -timeout 600s`。
- [x] 运行 `go tool cover -func=coverage.out | tail -1`，覆盖率需要 `>=90.0%`。
- [x] 运行 `golangci-lint run --timeout 12m`。
- [x] 逐个执行 `tests/e2e/*`：`go build -o testbin . && timeout 120s ./testbin`。
- [x] 清理 `coverage.out`、`testbin`、`*.prof`、`*.cover`。

验证备注：Go 全量测试通过，总覆盖率 `90.0%`；`golangci-lint` 输出 `0 issues.`；e2e 独立二进制全部通过并已清理产物。
