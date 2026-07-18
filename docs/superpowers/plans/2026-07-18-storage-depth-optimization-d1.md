# Storage Depth Optimization D1 Plan

> **For agentic workers:** 按任务顺序实施；每完成一项打勾并写实现备注。

**Goal:** 在 P0–P2 已闭环基础上，完成剩余项 EARS 清单落库，并实施 D1 深度优化：查询默认保护、MemTable 乱序降载、超大文件继续拆分。明确 **P3-01 暂不处理**。

**Architecture:**
- 默认查询保护在 `normalizeQuery` / Options 层注入，不改变显式 budget/limit 语义。
- 乱序降载：MemTable 统计驱动 shard flush 阈值，阈值可配置，默认保守。
- 文件拆分保持行为不变，优先 memtable / wal encoding / shard 边界清晰模块。

## Tasks

- [x] 生成 `docs/review/code-review-2026-07-18-0853.md` 剩余 EARS 清单
- [x] 更新旧检视文档 P3-01 → 暂不处理
- [x] EARS-D1-01 查询默认 budget/limit 保护 + 测试
- [x] EARS-D1-02 MemTable 乱序降载 + 测试
- [x] EARS-D1-03 超大文件再拆分（≥2 文件）
- [x] 全量 make test/e2e/lint + 核心 race + bench 对比
- [x] 更新新/旧文档状态并 commit

## 非目标

- EARS-P3-01 列式 WAL / 并行 compact / 对象存储冷层（**暂不处理**）
- D2/D3 全量一次做完（可开后续计划）

## 实现备注

（执行时追加）


### D1-01
- QueryProtectionOptions；DefaultMaxSamples=1_000_000
- applyQueryProtection 仅填充未设置字段

### D1-02
- MemTable DisorderRatio/AppendedSamples
- Shard shouldFlushMemTable；默认 ratio=0.25 min=1024

### D1-03
- memtable: column_buffer.go / compact.go
- wal: encoding_estimate/tombstone/value
