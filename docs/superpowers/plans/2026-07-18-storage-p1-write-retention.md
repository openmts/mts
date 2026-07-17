# Storage P1 Write Parallelism + Retention Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 完成检视报告阶段 B 的 P1-01（缩短全局写锁、跨 shard 并行写）与 P1-03（Retention 覆盖分片内过期数据）。

**Architecture:** `Engine.mu` 仅保护 shard map、内存限额检查与分片路由；`writeSeq` 改为 atomic；分片级 IO 在锁外并行。Retention 对完全过期 shard 仍整删，对部分重叠 shard 下发时间范围 tombstone。

**Tech Stack:** Go 1.26、现有 engine 测试、make test/e2e、storage_benchmark_gate。

## Global Constraints

- 简体中文；错误必须处理；0700/0600
- 每 task：单测 → make test → make e2e → bench 对比（median >10% 需解释）
- 不引入新外部依赖（并行写用 WaitGroup，不用 errgroup 包）

## Task 1: P1-01 写路径锁收敛 + 并行写

- [x] 1.1 测试：多 shard 并发写吞吐/正确性；单批跨 shard 结果完整
- [x] 1.2 `writeSeq` → `atomic.Uint64`；`nextWriteSeq()`
- [x] 1.3 `writeResolved`/`writeResolvedTyped`：锁内路由与内存检查，锁外并行 WriteBatch
- [x] 1.4 单 shard 路径避免无意义 goroutine
- [x] 1.5 race + make test + e2e + bench

## Task 2: P1-03 Retention 分片内清理

- [x] 2.1 测试：同一 shard 内旧点被删、新点保留
- [x] 2.2 `ApplyRetention`：部分重叠 shard 调 `DeleteRange`（带 writeSeq）
- [x] 2.3 更新 e2e retention 覆盖分片内过期（可选增强）
- [x] 2.4 make test + e2e + bench
- [x] 2.5 更新 review 状态

## 实现备注



## 实现备注

### 2026-07-18

**P1-01**
- `writeSeq` 改为 `atomic.Uint64`（`nextWriteSeq` / `observeWriteSeq`）
- `writeResolved` / `writeResolvedTyped`：锁内路由 + 内存检查，锁外 `writeShardBatches` 并行写
- 单 shard 不启 goroutine

**P1-03**
- 完全过期 shard：整删（原逻辑）
- 部分重叠 shard：`DeleteRange` tombstone 覆盖 `[Start, cutoff)`
- e2e retention 增加 partial 场景

**验证**
- `make test` 通过
- `make e2e` 通过（含增强 retention）
- race 关键路径通过
- golangci-lint engine：0 issues
- bench median vs P0 baseline：WriteBatch +1.23%、Wide +0.64%、Row +1.35%、Col +1.17%（均 <10%）

