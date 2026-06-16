# Storage Engine Phase 11 Benchmark

## 目标

基于 Phase 10 后的 100K wide10 query pprof 继续优化存储热路径，重点减少临时 `map`、block frame 分配、无条件排序和每 block seek。

## 优化内容

- `engine.mergeColumnData` 不再先构建 column map。改为按 `(series_id, field_id)` 排序后处理连续分组。
- 有序样本使用线性合并；单列重复 timestamp、双列合并和多列合并都按 `WriteSeq` 保留最新版本。
- 乱序样本保留 map fallback，确保正确性不依赖调用方完全有序。
- `sstable.readBlockPayloadFrom` 复用 block frame，`Part` decode 完成后释放，不再为每个读取 block 克隆 frame。
- `blockWriter` 记录顺序 offset，批量写入 `timestamps.bin` 和 `values.bin` 时避免每个 block `Seek(End)`。
- `collectTimestamps` 对有序稀疏列使用线性归并去重，写入热路径改用连续 series 分组，减少 `groupColumns` map。
- `columnFromBlock` 仅在检测到乱序样本时排序，避免查询阶段无条件 `sortSamples`。

## pprof 对比

命令：

```bash
go build -o /tmp/mts-pprof-run/storage_engine_phase11 ./tests/pprof/storage_engine
/tmp/mts-pprof-run/storage_engine_phase11 -mode=query -field-layout=wide10 -points=100000 -series=1000 -query-repeat=20 \
  -cpu-profile=/tmp/mts-pprof-run/query_phase11_cpu.prof \
  -mem-profile=/tmp/mts-pprof-run/query_phase11_heap.prof
```

Phase 10 后：

```text
运行时间约 3.5s
alloc_space total 约 1.98GB
alloc_objects total 约 11.0M
热点：engine.mergeSamples 241.6MB，engine.mergeColumnData 432.4MB cum，sstable.readBlockFrom 1.14M objects，sstable.sortSamples 860k objects
```

Phase 11 后：

```text
运行时间约 3.20s
alloc_space total 约 1.45GB
alloc_objects total 约 6.89M
engine.mergeSamples 不再进入 top
sstable.readBlockFrom 不再进入 top
sstable.releaseBlockFrame 不再进入 top
sstable.sortSamples 不再进入 top 20
```

## 剩余热点

- `engine.mergeOrderedSamples` 仍是查询合并 CPU 热点，主要来自多 part 有序归并。
- `memtable.ensureColumn` 和 `memtable.compactSamples` 是写入阶段主要对象分配来源。
- `sstable.readFloatSamples`、`readWriteSeqs`、`readIntSamples` 仍会为返回结果 materialize slice，这是当前公开查询返回结构需要承担的成本。
