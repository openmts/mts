# P3-01 Implementation Plan（POC）

**Goal:** 落地最终列式 WAL + 跨 shard 有界并行 compact（无对象存储、无旧格式兼容）。

## Tasks

- [x] 列式 WAL encode/decode + formatID=2
- [x] Append/AppendTyped/Replay 走列式
- [x] 更新 WAL 测试
- [x] Engine 跨 shard 有界并行 Compact
- [x] 并行 compact 测试 + 默认并发归一化
- [x] 全量 test/e2e/lint/race/bench
- [x] 更新 EARS 文档与 commit

## 实现备注

- WAL formatID=2，payload 全列式；POC 不兼容 formatID=1
- CompactWithResult 跨 shard worker pool，上限 MaxConcurrentCompaction（默认 min(GOMAXPROCS,4)）
- 对象存储冷层明确不做
- WriteBatch 10k vs baseline median 约 +8.5%（<10% 阈值）
