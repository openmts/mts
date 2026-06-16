# Storage Engine Phase 9 Benchmark

## 目标

继续降低 10K 写入路径的临时分配和 GC 压力，重点处理 Phase 8 后仍然明显的 catalog 解析、shard 分组和 flush 样本复制成本。

## 优化内容

- Catalog 批量写入借用输入 tags，并用批内 `ResolvedField` arena 减少每点字段切片分配。
- Catalog 增加运行时字段 schema cache，字段集合完全命中时跳过字段名排序和 field key 拼接。
- Catalog 增加单 tag series 运行时索引，重复写入不再每点拼接 series key。
- Engine 批内缓存 shard lookup，避免同一 shard 每点生成 shard ID 字符串。
- WAL tag 编码增加 0/1 tag 快路径，多 tag 仍保持稳定排序。
- MemTable 批量预留时使用 dense column samples，flush 全范围有序无重复列时零拷贝传给 SSTable。

## Phase 8 基线

```text
BenchmarkEngineWriteBatch/points=10000       20.8-21.1ms/op  24.9MB/op   99.5k allocs/op
BenchmarkEngineWriteWideBatch/points=10000   44.8-45.9ms/op  48.8MB/op  129.0k allocs/op
```

## Phase 9 结果

命令：

```bash
go test -run '^$' -bench 'BenchmarkEngineWrite(Wide)?Batch/points=10000$' -benchmem -count=5 ./internal/bench
```

结果：

```text
BenchmarkEngineWriteBatch/points=10000       16.5-16.9ms/op  21.1MB/op  58.0k allocs/op
BenchmarkEngineWriteWideBatch/points=10000   33.9-35.2ms/op  39.2MB/op  85.1k allocs/op
```

## 结论

- 默认 10K 写入相对 Phase 8：分配次数下降约 `41.7%`，分配字节下降约 `15.3%`。
- wide10 10K 写入相对 Phase 8：分配次数下降约 `34.0%`，分配字节下降约 `19.7%`。
- 本轮不改变 WAL/SSTable/Catalog 的磁盘格式，只增加运行时索引和批内内存布局优化。
