# Storage Engine Phase 5 Benchmarks

## 环境

- GOOS: `linux`
- GOARCH: `amd64`
- CPU: `AMD Ryzen 7 7840HS w/ Radeon 780M Graphics`

## 改动摘要

- SSTable value block 新增 v3 编码，新写入默认使用 v3。
- value block v3 不再重复保存完整 timestamp delta，而是引用同 series row 的 time block ordinal。
- v3 支持 `aligned` 和 `indexed` 两种 time reference 模式：
  - `aligned`：列样本与 row time block 完全同位，不保存 ordinal。
  - `indexed`：稀疏列保存 row time ordinal delta。
- 读取 v3 value block 时通过 row time block 还原 timestamp，并按 query time range 过滤输出样本。
- 保留 v2 value block 解码兼容。
- `collectTimestamps` 对 aligned wide columns 增加快路径，避免为 timestamp 并集构建 map。
- `writeBlock` 改为池化 frame buffer + 单次写入，避免三段写带来的 syscall 放大。

## Phase 4 基线

结果摘要：

```text
BenchmarkEngineWriteBatch/points=10000-16       ~23.2ms/op   ~34.3MB/op   ~133.7k allocs/op
BenchmarkEngineWriteWideBatch/points=10000-16   ~46.7ms/op   ~66.4MB/op   ~167.6k allocs/op
```

1M wide10 pprof smoke：约 `19.5s`，in-use heap 约 `10.8MB`，alloc_space 约 `9.25GB`。

## Phase 5 结果

命令：

```bash
go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s
```

结果：

```text
BenchmarkEngineWriteBatch/points=1000-16         	     273	   4350453 ns/op	 3857067 B/op	   21434 allocs/op
BenchmarkEngineWriteBatch/points=1000-16         	     272	   4270732 ns/op	 3857211 B/op	   21434 allocs/op
BenchmarkEngineWriteBatch/points=1000-16         	     285	   4220732 ns/op	 3856700 B/op	   21434 allocs/op
BenchmarkEngineWriteBatch/points=10000-16        	      54	  21982906 ns/op	33330574 B/op	  132347 allocs/op
BenchmarkEngineWriteBatch/points=10000-16        	      49	  22053420 ns/op	33330683 B/op	  132348 allocs/op
BenchmarkEngineWriteBatch/points=10000-16        	      49	  22637946 ns/op	33332222 B/op	  132353 allocs/op
BenchmarkEngineWriteWideBatch/points=1000-16     	     150	   7938009 ns/op	 7494225 B/op	   33122 allocs/op
BenchmarkEngineWriteWideBatch/points=1000-16     	     148	   8015350 ns/op	 7493972 B/op	   33122 allocs/op
BenchmarkEngineWriteWideBatch/points=1000-16     	     151	   7887006 ns/op	 7493420 B/op	   33121 allocs/op
BenchmarkEngineWriteWideBatch/points=10000-16    	      26	  44394828 ns/op	64686445 B/op	  166036 allocs/op
BenchmarkEngineWriteWideBatch/points=10000-16    	      24	  44570588 ns/op	64688174 B/op	  166039 allocs/op
BenchmarkEngineWriteWideBatch/points=10000-16    	      26	  44106233 ns/op	64687920 B/op	  166039 allocs/op
```

## 对比

- 4 字段 10K：约 `23.2ms -> 22.2ms`，`34.3MB -> 33.3MB`，`133.7k -> 132.3k allocs/op`。
- wide10 10K：约 `46.7ms -> 44.4ms`，`66.4MB -> 64.7MB`，`167.6k -> 166.0k allocs/op`。
- wide10 1K：约 `8.1ms -> 7.9ms`，`7.8MB -> 7.5MB`，`34.4k -> 33.1k allocs/op`。

## Disk Size Smoke

命令：

```bash
rm -rf /tmp/mts-phase5-size
timeout 600s go run ./tests/pprof/storage_engine -data-dir /tmp/mts-phase5-size -mode=write -field-layout=wide10 -points 100000 -series 1000
du -sb /tmp/mts-phase5-size
rm -rf /tmp/mts-phase5-size
```

结果：

```text
29394191 /tmp/mts-phase5-size
```

100K wide10 写入后的数据目录约 `29.4MB`。v3 aligned value block 的单元测试同时验证了 dense columns 的 v3 payload 小于 v2 payload。

## Pprof Smoke

命令：

```bash
time -p timeout 1200s go run ./tests/pprof/storage_engine -mode=write -field-layout=wide10 -points 1000000 -series 10000 -mem-profile /tmp/mts-wide10-phase5-mem.prof
go tool pprof -inuse_space -top /tmp/mts-wide10-phase5-mem.prof
go tool pprof -alloc_space -top /tmp/mts-wide10-phase5-mem.prof
rm -f /tmp/mts-wide10-phase5-mem.prof
```

结果摘要：

- 1M wide10 写入完成，`real 20.15s`。
- 写入结束后的 in-use heap 约 `9.3MB`。
- 1M wide10 `alloc_space` 约 `9.16GB`。
- 主要累计分配热点：
  - `memtable.ensureColumn`: 约 `1.81GB`
  - pprof workload point 构造: 约 `1.53GB`
  - `sstable.groupColumns`: 约 `1.41GB`
  - `catalog.resolveFieldsNoSnapshotLocked`: 约 `0.97GB`
  - `memtable.compactSamples`: 约 `0.60GB`

## Query Smoke

命令：

```bash
time -p timeout 600s go run ./tests/pprof/storage_engine -mode=query -field-layout=wide10 -points 100000 -series 1000 -query-repeat 10
```

结果：

```text
real 8.60
user 6.02
sys 4.28
```

## 取舍记录

曾测试 `writeBlock` 三段写入 header、payload、crc，alloc_space 可下降到约 `8.76GB`，但 1M wide10 写入 `real` 上升到约 `32.28s`，`sys` 时间明显放大。最终采用池化 frame buffer + 单次写入，保证内存复用同时避免 syscall 放大。

## 后续优化重点

- `sstable.groupColumns` 仍是主要 flush 热点，下一步应让 MemTable snapshot 直接输出按 series 分组的列，绕过 map grouping。
- Catalog 字段 resolve 仍占约 `0.97GB alloc_space`，需要减少每点字段排序和临时 map 构造。
- pprof workload 构造占比高，后续 benchmark 可增加预生成 points 模式，隔离引擎成本。
