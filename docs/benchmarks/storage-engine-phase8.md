# Storage Engine Phase 8 Benchmark

## 目标

本轮继续优化 catalog 写入热路径。Phase 7 后的 profile 显示：

- `catalog.resolveFieldsNoSnapshotLocked` 仍是主要分配热点。
- `catalog.seriesKey` 在单 tag 写入场景中仍会构造 slice、排序并调用 `strings.Join`。

先试验了 `measurement -> field name -> id` 分层字段索引，避免 `fieldKey(measurement, name)` 拼接；benchmark 未显示可观收益，已回退该复杂度。最终保留的是 `seriesKey` 的 0/1 tag 快路径。

## 基线

命令：

```bash
go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s
```

关键结果：

```text
BenchmarkEngineWriteBatch/points=10000       21.3-21.9ms/op  25.0MB/op  109.6k allocs/op
BenchmarkEngineWriteWideBatch/points=10000   45.9-46.4ms/op  49.0MB/op  139.1k allocs/op
```

## 优化后

命令：

```bash
go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s
```

关键结果：

```text
BenchmarkEngineWriteBatch/points=10000       20.8-21.1ms/op  24.9MB/op   99.5k allocs/op
BenchmarkEngineWriteWideBatch/points=10000   44.8-45.9ms/op  48.8MB/op  129.0k allocs/op
```

## 结论

- 默认 10K 写入分配次数下降约 `9.2%`。
- wide10 10K 写入分配次数下降约 `7.3%`。
- 内存分配量小幅下降，耗时小幅改善。
- 下一轮主热点仍是 `resolveFieldsNoSnapshotLocked`，更有效方向是字段 schema cache 或批量解析时复用字段顺序，但需要保证 WAL 编码和字段顺序测试的确定性。

备注：本机未安装 `benchstat`，本轮对比使用原始 benchmark 输出。
