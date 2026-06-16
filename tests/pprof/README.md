# pprof

这里存放用于性能问题剖析的独立 workload。示例：

```bash
cd tests/pprof/storage_engine
go build -o storage_engine .
./storage_engine -points 100000 -series 100 -cpu-profile cpu.prof -mem-profile mem.prof
go tool pprof cpu.prof
go tool pprof -alloc_space mem.prof
rm -f storage_engine cpu.prof mem.prof
```

约定：默认使用临时数据目录并自动清理；如需复查磁盘文件，可通过 `-data-dir` 指定目录。
运行结束会输出 `metrics sstable_count=... data_dir_bytes=... heap_alloc_bytes=... heap_sys_bytes=... heap_total_alloc_bytes=... mallocs=... frees=... num_gc=... pause_total_ns=... rss_bytes=... rss_peak_bytes=...`，用于固定参数下对比 SSTable 数、落盘体积、运行时 heap、GC 和 RSS 峰值。

1M wide10 写入时建议显式放大 MemTable，避免默认 `8192` samples 触发过多小 SSTable：

```bash
./storage_engine -mode=write -field-layout=wide10 -points=1000000 -series=10000 \
  -memtable-max-samples=1000000 -write-batch-size=4096 -flush-on-exit \
  -data-dir=/tmp/mts-pprof-wide10
```

需要观察 compaction 后的最终落盘结构时，可以启用本地 compaction 参数：

```bash
./storage_engine -mode=query -field-layout=wide10 -points=1000000 -series=10000 -query-repeat=1 \
  -memtable-max-samples=1000000 -compaction-enabled -compaction-level0-part-limit=4 -flush-on-exit \
  -compaction-max-output-part-bytes=268435456 \
  -data-dir=/tmp/mts-pprof-wide10-compact
```
