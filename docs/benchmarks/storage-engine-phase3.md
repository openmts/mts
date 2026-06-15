# Storage Engine Phase 3 Benchmarks

## 环境

- GOOS: `linux`
- GOARCH: `amd64`
- CPU: `AMD Ryzen 7 7840HS w/ Radeon 780M Graphics`

## 改动摘要

- `Engine.Write` 改为批量 resolve 并按 shard 分组。
- `Shard.WriteBatch` 将同 shard batch 写成一个 WAL record。
- `MemTable.SnapshotAndReset` 改为 swap，flush 失败时恢复 snapshot。
- WAL batch 编码增加精确容量预估，减少大 batch 扩容。
- SSTable Part 改为懒加载 index rows，降低大量 Part 的常驻内存。
- pprof/benchmark 增加 `wide10`：5 个 `float64`、3 个 `int64`、1 个 `string`、1 个 `bool`。

## Phase 2 基线

命令：

```bash
go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s
```

结果摘要：

```text
BenchmarkEngineWriteBatch/points=1000-16       ~14.4ms/op   ~8.8MB/op    ~62.5k allocs/op
BenchmarkEngineWriteBatch/points=10000-16      ~51.5ms/op   ~45.8MB/op   ~254.4k allocs/op
```

## Phase 3 结果

命令：

```bash
go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s
```

结果：

```text
BenchmarkEngineWriteBatch/points=1000-16         	     241	   4963662 ns/op	 4915658 B/op	   25411 allocs/op
BenchmarkEngineWriteBatch/points=1000-16         	     244	   4948569 ns/op	 4915717 B/op	   25410 allocs/op
BenchmarkEngineWriteBatch/points=1000-16         	     246	   4841244 ns/op	 4915205 B/op	   25410 allocs/op
BenchmarkEngineWriteBatch/points=10000-16        	      39	  29869020 ns/op	42887569 B/op	  139252 allocs/op
BenchmarkEngineWriteBatch/points=10000-16        	      39	  30140885 ns/op	42886368 B/op	  139250 allocs/op
BenchmarkEngineWriteBatch/points=10000-16        	      36	  29873163 ns/op	42885817 B/op	  139248 allocs/op
BenchmarkEngineWriteWideBatch/points=1000-16     	     132	   9141961 ns/op	10048553 B/op	   42010 allocs/op
BenchmarkEngineWriteWideBatch/points=1000-16     	     130	   9122065 ns/op	10047655 B/op	   42009 allocs/op
BenchmarkEngineWriteWideBatch/points=1000-16     	     132	   9058981 ns/op	10047691 B/op	   42010 allocs/op
BenchmarkEngineWriteWideBatch/points=10000-16    	      18	  63775185 ns/op	87751246 B/op	  181163 allocs/op
BenchmarkEngineWriteWideBatch/points=10000-16    	      16	  63083110 ns/op	87749378 B/op	  181157 allocs/op
BenchmarkEngineWriteWideBatch/points=10000-16    	      18	  62689564 ns/op	87750882 B/op	  181162 allocs/op
```

## 对比

- 4 字段 1K：约 `14.4ms -> 4.9ms`，`8.8MB -> 4.9MB`，`62.5k -> 25.4k allocs/op`。
- 4 字段 10K：约 `51.5ms -> 30.0ms`，`45.8MB -> 42.9MB`，`254.4k -> 139.3k allocs/op`。
- wide10 10K：新增基线约 `63ms/op`、`87.8MB/op`、`181.2k allocs/op`。

## Pprof Smoke

命令：

```bash
go run ./tests/pprof/storage_engine -mode=write -field-layout=wide10 -points 100000 -series 1000 -mem-profile /tmp/mts-wide10-mem.prof
go tool pprof -inuse_space -top /tmp/mts-wide10-mem.prof
go tool pprof -alloc_space -top /tmp/mts-wide10-mem.prof
rm -f /tmp/mts-wide10-mem.prof
```

结果摘要：

- 100K wide10 写入完成。
- SSTable lazy index 后 in-use heap 约 `12.8MB`。
- lazy index 前同 workload in-use heap 约 `40MB`，主要常驻在 `sstable.readColumnRefs` / `decodeIndexRows`。
- alloc_space 约 `1696MB`，主要来自 MemTable 多层 map、flush query 重组和 SSTable 写入。

## Million-Scale Smoke

命令：

```bash
timeout 1200s go run ./tests/pprof/storage_engine -mode=write -field-layout=wide10 -points 1000000 -series 10000 -mem-profile /tmp/mts-wide10-1m-mem.prof
go tool pprof -inuse_space -top /tmp/mts-wide10-1m-mem.prof
rm -f /tmp/mts-wide10-1m-mem.prof
```

结果摘要：

- 1M wide10 写入完成，耗时约 `23s`。
- 写入结束后的 in-use heap 约 `10.8MB`。
- profile 已清理。
