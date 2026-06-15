# Storage Engine Phase 4 Benchmarks

## 环境

- GOOS: `linux`
- GOARCH: `amd64`
- CPU: `AMD Ryzen 7 7840HS w/ Radeon 780M Graphics`

## 改动摘要

- MemTable 从 `series -> field -> timestamp` 三层 map 改为 `(seriesID, fieldID)` 列 buffer。
- MemTable 写入阶段只追加样本，查询和 flush 阶段统一做 LWW 压实。
- `Snapshot.Columns` 为 flush 提供显式列式输出，保持按 series、field、timestamp 排序。
- 列 buffer 首样本内联，减少单样本列的 slice 分配。
- SSTable 写入对已经按 timestamp 排序的列跳过 clone+sort，仅在未排序输入时复制并排序。
- 验证并放弃了 `map[columnKey]columnBuffer` 值存储实验：它降低少量 allocs/op，但会增加结构体复制，导致 10K wide10 延迟和 1M alloc_space 变差。

## Phase 3 基线

命令：

```bash
go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s
```

结果摘要：

```text
BenchmarkEngineWriteBatch/points=10000-16       ~30.0ms/op   ~42.9MB/op   ~139.3k allocs/op
BenchmarkEngineWriteWideBatch/points=10000-16   ~63.0ms/op   ~87.8MB/op   ~181.2k allocs/op
```

Phase 3 100K wide10 `alloc_space` 约 `1696MB`；1M wide10 写入约 `23s`，in-use heap 约 `10.8MB`。

## Phase 4 结果

命令：

```bash
go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s
```

结果：

```text
BenchmarkEngineWriteBatch/points=1000-16         	     276	   4299236 ns/op	 3978306 B/op	   22305 allocs/op
BenchmarkEngineWriteBatch/points=1000-16         	     274	   4314275 ns/op	 3978665 B/op	   22306 allocs/op
BenchmarkEngineWriteBatch/points=1000-16         	     276	   4334648 ns/op	 3978624 B/op	   22306 allocs/op
BenchmarkEngineWriteBatch/points=10000-16        	      51	  23108988 ns/op	34333841 B/op	  133745 allocs/op
BenchmarkEngineWriteBatch/points=10000-16        	      50	  23107923 ns/op	34332760 B/op	  133742 allocs/op
BenchmarkEngineWriteBatch/points=10000-16        	      45	  23289693 ns/op	34333037 B/op	  133744 allocs/op
BenchmarkEngineWriteWideBatch/points=1000-16     	     147	   8091352 ns/op	 7770991 B/op	   34419 allocs/op
BenchmarkEngineWriteWideBatch/points=1000-16     	     144	   8197514 ns/op	 7771457 B/op	   34420 allocs/op
BenchmarkEngineWriteWideBatch/points=1000-16     	     146	   8124505 ns/op	 7771826 B/op	   34420 allocs/op
BenchmarkEngineWriteWideBatch/points=10000-16    	      25	  46063520 ns/op	66429380 B/op	  167570 allocs/op
BenchmarkEngineWriteWideBatch/points=10000-16    	      24	  46378810 ns/op	66431562 B/op	  167576 allocs/op
BenchmarkEngineWriteWideBatch/points=10000-16    	      21	  47663316 ns/op	66431123 B/op	  167577 allocs/op
```

## 对比

- 4 字段 10K：约 `30.0ms -> 23.2ms`，`42.9MB -> 34.3MB`，`139.3k -> 133.7k allocs/op`。
- wide10 10K：约 `63.0ms -> 46.7ms`，`87.8MB -> 66.4MB`，`181.2k -> 167.6k allocs/op`。
- wide10 1K：约 `9.1ms -> 8.1ms`，`10.0MB -> 7.8MB`，`42.0k -> 34.4k allocs/op`。

## Pprof Smoke

命令：

```bash
timeout 1200s go run ./tests/pprof/storage_engine -mode=write -field-layout=wide10 -points 1000000 -series 10000 -mem-profile /tmp/mts-wide10-phase4-mem.prof
go tool pprof -inuse_space -top /tmp/mts-wide10-phase4-mem.prof
go tool pprof -alloc_space -top /tmp/mts-wide10-phase4-mem.prof
rm -f /tmp/mts-wide10-phase4-mem.prof
```

结果摘要：

- 1M wide10 写入完成，耗时约 `19.5s`。
- 写入结束后的 in-use heap 约 `10.8MB`。
- 1M wide10 `alloc_space` 约 `9.25GB`。
- 主要累计分配热点：
  - `memtable.ensureColumn`: 约 `1.84GB`
  - `tests/pprof` 的 workload point 构造: 约 `1.50GB`
  - `sstable.groupColumns`: 约 `1.35GB`
  - `catalog.resolveFieldsNoSnapshotLocked`: 约 `1.01GB`
  - `memtable.compactSamples`: 约 `0.59GB`

## 后续优化重点

- Catalog 字段 resolve 热路径：减少字段排序和 map 构造。
- SSTable flush：让 MemTable snapshot 直接输出按 series 分组的数据，进一步减少 `groupColumns` 分配。
- pprof workload 构造：新增预生成 point 模式，区分测试数据构造成本和引擎写入成本。
