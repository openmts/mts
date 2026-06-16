# Storage Engine Phase 7 Benchmark

## 目标

本轮优化针对 catalog 批量解析写入点时的临时 tag clone。`ResolvePoints` 只服务 engine 同步写入热路径，返回的 `ResolvedPoint.Tags` 会立即用于 WAL 编码，MemTable 和 SSTable 不会持久引用该 map。因此批量解析可以借用输入 tags，避免每个 point 都 clone 一份临时 map。

`ResolvePoint` 单点 API 仍保持防御性 clone，Series 元数据也仍然 clone tags，避免外部修改污染 catalog。

## 基线

命令：

```bash
go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s
```

关键结果：

```text
BenchmarkEngineWriteBatch/points=10000       23.0-23.1ms/op  28.4MB/op  129.6k allocs/op
BenchmarkEngineWriteWideBatch/points=10000   46.8-47.3ms/op  52.4MB/op  159.1k allocs/op
```

wide10 alloc profile 热点：

```text
catalog.cloneTags                            71.0MB alloc_space / 477k alloc_objects
catalog.resolveFieldsNoSnapshotLocked        231.2MB alloc_space
memtable.compactSamples                      138.4MB alloc_space
memtable.columnBuffer.reserve                131.3MB alloc_space
```

## 优化后

命令：

```bash
go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s
```

关键结果：

```text
BenchmarkEngineWriteBatch/points=10000       21.3-21.9ms/op  25.0MB/op  109.6k allocs/op
BenchmarkEngineWriteWideBatch/points=10000   45.9-46.4ms/op  49.0MB/op  139.1k allocs/op
```

wide10 alloc profile 主要变化：

```text
catalog.cloneTags                            退出主要热点
catalog.ResolvePoints                        alloc_objects 从约 1.42M 降到约 0.90M
```

## 结论

- 默认 10K 写入内存约下降 `11.9%`，分配次数下降约 `15.4%`。
- wide10 10K 写入内存约下降 `6.4%`，分配次数下降约 `12.6%`。
- 本轮同时小幅改善耗时，wide10 10K 从约 `47ms/op` 降到约 `46ms/op`。
- 下一轮主要热点是字段名解析：`resolveFieldsNoSnapshotLocked` 仍会为每个 point 构造并排序 field name slice。可选方向是引入 schema cache 或复用 field-name scratch，但需要谨慎保持 WAL/测试确定性。

备注：本机未安装 `benchstat`，本轮对比使用原始 benchmark 输出。
