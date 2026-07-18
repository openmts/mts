# P3 follow-up: 列式 WAL 分配 + compact 配额

## Tasks

- [x] 列式 WAL 去掉 field 值矩阵中间分配，改为单遍 schema + 按列写出
- [x] typed 路径 dense presence + 直接标量写出
- [x] `Compaction.MaxConcurrent` 可配置并与 `MaxConcurrentCompaction` 同步
- [x] 测试 / lint / e2e / race / bench

## Bench

- WriteBatch points=10000 vs baseline median 约 +3.6%
