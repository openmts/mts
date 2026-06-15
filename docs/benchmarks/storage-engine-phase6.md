# Storage Engine Phase 6 Benchmark

## 目标

本轮优化针对写入时 MemTable 列缓冲的反复扩容。`ApplyBatch` 会先统计本批次每个 `(seriesID, fieldID)` 的样本数，为 column buffer 精确预留容量；统计用的 reservation map 通过 `sync.Pool` 复用，并在超大批次后丢弃，避免常驻内存膨胀。

## 基线

命令：

```bash
go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s
```

关键结果：

```text
BenchmarkEngineWriteBatch/points=10000       21.9-22.4ms/op  33.3MB/op  132.3k allocs/op
BenchmarkEngineWriteWideBatch/points=10000   44.4-45.6ms/op  64.7MB/op  166.0k allocs/op
```

wide10 alloc profile 主要热点：

```text
columnBuffer.appendSample                    470.9MB alloc_space
makeWideBenchPoints                          446.7MB alloc_space
Catalog.resolveFieldsNoSnapshotLocked        273.7MB alloc_space
memtable.compactSamples                      166.0MB alloc_space
```

## 优化后

命令：

```bash
go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s
```

关键结果：

```text
BenchmarkEngineWriteBatch/points=10000       23.5-23.8ms/op  28.4MB/op  129.6k allocs/op
BenchmarkEngineWriteWideBatch/points=10000   46.6-49.1ms/op  52.4MB/op  159.1k allocs/op
```

wide10 alloc profile 主要变化：

```text
columnBuffer.appendSample                    基本退出 alloc_space 热点
columnBuffer.reserve                         130.8MB alloc_space
MemTable.reserveBatchLocked                   16.1MB alloc_space
```

## 结论

- 默认 10K 写入内存约下降 `14.8%`，wide10 10K 写入内存约下降 `19.0%`。
- wide10 10K 分配次数从约 `166.0k allocs/op` 降到约 `159.1k allocs/op`。
- 10K 场景耗时有小幅回退，原因是批量统计需要额外遍历字段并访问 reservation map。
- 该取舍优先降低千万级写入下的内存峰值和 GC 压力；下一轮可继续从 catalog 字段解析和 tag clone 入手，同时评估是否需要引入 schema cache。

备注：本机未安装 `benchstat`，本轮对比使用原始 benchmark 输出。
