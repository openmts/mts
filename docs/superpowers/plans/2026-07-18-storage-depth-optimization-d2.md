# Storage Depth Optimization D2/D3 Plan

> **For agentic workers:** 按任务顺序实施；每完成一项打勾并写实现备注。

**Goal:** 在 D1 已闭环基础上，实施 D2 深度优化与 D3 工程化文档/可观测补齐；**P3-01 明确暂不处理**。

## Tasks

- [x] 刷新 EARS 清单与旧文档：P3-01 暂不处理
- [x] EARS-D2-01 可校准查询代价 + 测试
- [x] EARS-D2-02 SSTable block fuzz
- [x] EARS-D2-03 MaxConcurrentCompaction 全局维护并发
- [x] EARS-D2-04 确认 registry 共享闭环
- [x] EARS-D3-01/02/03/04 文档与可观测补齐
- [x] 全量 make test/e2e/lint + 核心 race + bench
- [x] 更新状态并 commit

## 非目标

- EARS-P3-01 列式 WAL / 分区并行 compact / 对象存储冷层（**暂不处理**）

## 实现备注

### D2-01
- estimateMatchedPartStats / proportionalPartRows / estimateQuerySamples
- public QueryExplain/Cost 同步字段

### D2-02
- block_fuzz_test.go FuzzBlockRoundTrip + CorruptCRC

### D2-03
- MaxConcurrentCompaction 默认 1；scheduler concurrency_limit

### D2-04
- 沿用 P2-04 operation_registry，无额外弱实现

### D3
- docs/ops/* + compatibility 矩阵 + CHANGELOG + mts_tombstones_pending


### 验证
- make test / make e2e / golangci-lint 0 issues
- race core packages OK
- WriteBatch 10k median +2.01% vs baseline
