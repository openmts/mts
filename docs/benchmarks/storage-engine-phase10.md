# Storage Engine Phase 10 Benchmark

## 目标

根据 100K wide10 pprof 结果优化查询和 compaction 读路径。Phase 9 后，综合 workload 中 `sstable.readBlock` 每次读取 block 都重新 `os.Open`/`Close` 文件，导致 syscall、文件对象分配和路径拼接成为主要热点。

## 优化内容

- `sstable.Part` 在 `OpenPart` 时打开并持有 `index.bin`、`timestamps.bin`、`values.bin` 的只读句柄。
- `Part.Query`、compaction 读取候选 part 时复用已打开文件，通过 `ReadAt` 读取 block。
- 新增 `Part.Close`，`Shard.Close`、retention、compaction 替换旧 part 时统一释放句柄。
- 异常路径补齐资源释放，避免 shard 打开过程中后续 WAL/replay 失败时泄漏已打开 part。

## pprof 对比

命令：

```bash
go build -o /tmp/mts-pprof-run/storage_engine ./tests/pprof/storage_engine
/tmp/mts-pprof-run/storage_engine -mode=query -field-layout=wide10 -points=100000 -series=1000 -query-repeat=20 \
  -cpu-profile=/tmp/mts-pprof-run/query_cpu.prof \
  -mem-profile=/tmp/mts-pprof-run/query_heap.prof
```

优化前：

```text
运行时间约 8.5s
CPU: os.OpenFile cum 43.92%, sstable.(*Part).readValueColumn cum 53.49%
alloc_space: os.newFile 93.5MB, syscall.ByteSliceFromString 75.5MB, filepath.Join/strings.Join 78.0MB
```

优化后：

```text
运行时间约 3.5s
CPU: os.OpenFile 不再进入 top；sstable.(*Part).readValueColumn cum 13.98%
alloc_space: readBlockFrom 38.9MB，文件 open/path 拼接相关分配不再进入 top
```

## 后续热点

- `engine.mergeSamples` / `engine.mergeColumnData` 仍是 query/compact 主要分配来源。
- `sstable.readBlockFrom` 仍为每个 block 分配 frame，可考虑读缓冲复用或按 payload 分配。
- 写入侧主要热点仍在 `sstable.writeBlock` 和 `memtable.ensureColumn`，属于下一轮优化范围。
