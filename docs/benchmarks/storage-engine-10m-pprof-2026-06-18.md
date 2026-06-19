# Storage Engine 10M pprof 执行报告 2026-06-18

## 结论

本次在提交 `a33bc0a perf(wal): 优化批量写入 identity 查重` 后，执行 10M wide10 数据规模的写入和纯读取 pprof。wide10 表示每个 point 包含 10 个字段：5 个 `float64`、3 个 `int64`、1 个 `string`、1 个 `bool`，因此 10M points 对应 100M field samples。

写入可以完成，整体资源占用在当前机器上没有 OOM，但仍有明显优化空间：写入 CPU 第一热点是 MemTable 内存估算，读取/打开第一热点是 SSTable 校验路径中的大量 `stat` 系统调用。

## 环境

- OS：Fedora 44，Linux `7.0.12-201.fc44.x86_64`
- Go：`go1.26.2 linux/amd64`
- CPU：16 cores
- 内存：12GiB，总可用约 6.9GiB
- 数据目录：`/tmp/mts-10m-run/data`

## 测试口径

- points：`10000000`
- series：`100000`
- field layout：`wide10`
- write batch size：`4096`
- MemTable max samples：`5000000`
- compression：`off`
- compaction：`false`
- write：写完 `flush-on-exit`
- read：复用 write 生成的数据目录，执行纯 `mode=read`，`query-repeat=20`
- 说明：未预生成 points，避免造数数组直接抬高 RSS。

## 执行命令

```bash
go build -o /tmp/mts-10m-run/storage_engine ./tests/pprof/storage_engine

timeout 1800s /usr/bin/time -v /tmp/mts-10m-run/storage_engine \
  -mode=write -field-layout=wide10 -points=10000000 -series=100000 \
  -memtable-max-samples=5000000 -write-batch-size=4096 -flush-on-exit \
  -data-dir=/tmp/mts-10m-run/data \
  -cpu-profile=/tmp/mts-10m-run/write.cpu.prof \
  -mem-profile=/tmp/mts-10m-run/write.mem.prof

timeout 600s /usr/bin/time -v /tmp/mts-10m-run/storage_engine \
  -mode=read -field-layout=wide10 -points=10000000 -series=100000 \
  -memtable-max-samples=5000000 -write-batch-size=4096 -query-repeat=20 \
  -data-dir=/tmp/mts-10m-run/data \
  -cpu-profile=/tmp/mts-10m-run/read.cpu.prof \
  -mem-profile=/tmp/mts-10m-run/read.mem.prof
```

## 10M write 指标

| 指标 | 数值 |
| --- | ---: |
| workload duration | `520663ms` |
| process elapsed | `8:49.53` |
| 吞吐，按 workload | `19206 points/s`，约 `192060 samples/s` |
| 吞吐，按进程总耗时 | `18884 points/s`，约 `188840 samples/s` |
| CPU | `127%` |
| user time | `583.44s` |
| system time | `93.65s` |
| RSS peak | `1973526528 bytes`，约 `1.84GiB` |
| max resident set，time -v | `1927272 KB` |
| heap alloc after workload | `1277991936 bytes` |
| heap alloc after profile | `279930784 bytes` |
| heap sys | `1924136960 bytes` |
| total alloc | `108.46GB` |
| mallocs | `730681643` |
| num GC | `268` |
| GC pause total | `12.59ms` |
| storage peak bytes | `317691720` |
| SSTable count | `20` |
| final data dir bytes | `2187921564`，约 `2.04GiB` |

## 10M read 指标

| 指标 | 数值 |
| --- | ---: |
| process elapsed | `1:53.87` |
| workload duration | `1233ms` |
| CPU | `109%` |
| user time | `57.62s` |
| system time | `67.25s` |
| RSS peak | `649441280 bytes`，约 `619MiB` |
| max resident set，time -v | `633104 KB` |
| heap alloc before workload | `367298728 bytes` |
| heap alloc after profile | `279461240 bytes` |
| heap sys | `632291328 bytes` |
| total alloc | `20.00GB` |
| mallocs | `170856479` |
| num GC | `124` |
| query rows streamed | `2000` |
| query samples returned | `20000` |
| value pages read | `4000` |
| parts scanned | `400` |
| query errors | `0` |

read 的进程总耗时主要消耗在打开已有 SSTable 与校验索引元数据；实际 query workload 只有 `1233ms`。

## 磁盘空间明细

最终数据目录总大小：`2187921564 bytes`，约 `2.04GiB`。

| 文件类型 | 文件数 | 总大小 |
| --- | ---: | ---: |
| `values.bin` | 20 | `1683018720` |
| `index.bin` | 20 | `388928378` |
| `timestamps.bin` | 20 | `60000000` |
| `series_index.bin` | 20 | `53386224` |
| `snapshot.bin` | 1 | `2581886` |
| `metadata.bin` | 21 | `3302` |
| `MANIFEST.bin` | 1 | `2026` |
| `metaindex.bin` | 20 | `1013` |
| `000040.wal` | 1 | `15` |
| `strings.bin` | 20 | `0` |
| `catalog.wal` | 1 | `0` |

按 10M points 计算，落盘约 `218.79 bytes/point`；按 100M field samples 计算，约 `21.88 bytes/sample`。

## write pprof 热点

### CPU

- `memtable.(*columnBuffer).approxMemoryBytes`: `35.12% flat`
- `syscall6`: `16.11% flat`
- `runtime.spanClass.sizeclass`: `11.43% flat`
- `runtime.scanObject`: `9.55% cum`
- `runtime.tryDeferToSpanScan`: `13.63% cum`
- `memtable.approxTableDataBytes`: `37.35% cum`
- `memtable.(*columnBuffer).reserve`: `2.21% cum`
- `catalog.resolveFieldsFromSchema`: `0.60% cum`

判断：写入 CPU 最大问题不是 WAL，而是 MemTable 内存估算在大 MemTable 下被频繁调用，`approxTableDataBytes` 会反复遍历列结构，100M samples 下成本被放大。

### alloc_space

- `main.wide10WorkloadPoint`: `14.67GB`
- `memtable.(*MemTable).reserveBatchLocked`: `11.92GB flat / 21.86GB cum`
- `mts.toModelPoint`: `11.49GB`
- `os.statNolog`: `8.55GB`
- `catalog.ResolvePoints`: `8.42GB`
- `memtable.materializeSortedColumnSamples`: `5.92GB`
- `memtable.ensureColumn`: `5.32GB`
- `wal.batchIdentities`: `1.83GB flat / 1.97GB cum`
- `wal.encodeBatch`: `1.51GB flat / 4.48GB cum`

判断：WAL 优化后 `wal.batchIdentities` 不是最主要问题。更大的分配来自造数、public API 转换、MemTable reserve、Catalog resolve 和 SSTable 校验/文件 stat。

## read pprof 热点

### CPU

- `syscall6`: `68.50% flat`
- `sstable.validateColumnRefs`: `80.71% cum`
- `sstable.validateBlockRefWithinFile`: `57.00% cum`
- `storagefs.Stat`: `56.55% cum`
- `sstable.validateValueIndex`: `52.44% cum`
- `sstable.readBlockPayloadFrom`: `24.00% cum`
- `os.File.ReadAt`: `22.43% cum`

判断：10M 数据冷启动读的 CPU/系统时间主要被 SSTable 打开时的 block ref 校验吞掉，大量 `Stat`/`ByteSliceFromString`/路径处理带来系统调用和分配。这个路径是当前最明确的读取优化点。

### alloc_space

- `os.statNolog`: `8.53GB`
- `syscall.ByteSliceFromString`: `3.26GB`
- `internal/bytealg.MakeNoZero`: `3.20GB`
- `sstable.readFilteredColumnRefsInto`: `0.88GB`
- `catalog.cloneTags`: `0.64GB`
- `sstable.unmarshalValuePageIndex`: `0.59GB`
- `sstable.decodeIndexRows`: `1.46GB cum`
- `mts.Open`: `17.76GB cum`

判断：read profile 中 `mts.Open` 占 `17.76GB alloc_space`，说明冷启动加载/校验 20 个大 SSTable 的路径还不够轻。实际 query workload 本身只有约 `0.74GB cum`。

## 优化空间

1. **MemTable 内存估算增量化**：避免写入期间反复全量遍历列缓冲计算 `approxTableDataBytes`。应在 append/reserve 时维护 table-level memory counter。
2. **SSTable block ref 校验去系统调用化**：打开 part 时缓存各组件文件大小，`validateBlockRefWithinFile` 不应为每个 block ref 反复 `Stat` 文件。
3. **SSTable index/value page 元数据加载降分配**：`unmarshalValuePageIndex`、`decodeIndexRows`、`readFilteredColumnRefsInto` 在 10M 冷启动下仍有 GB 级累计分配，适合继续做 view/arena/scratch 复用。
4. **public API 写入 fast path**：`toModelPoint`、`wide10WorkloadPoint`、`cloneStringMap` 仍是累计分配大头。若目标是服务端内网 ingestion，可设计受控的 typed/batch API，减少 map 与 tags clone。
5. **文件路径与 stat 分配治理**：`os.statNolog`、`ByteSliceFromString`、`MakeNoZero` 在 write/read 都很高，说明文件校验与路径转换是大规模数据下的系统性开销。

## 2026-06-18 执行批次 1 后 no-profile scale 结果

本批次完成两项已证实热点优化：

- `MemTable.ApproxMemoryBytes()` 改为 O(1) 增量计数，避免写入路径反复扫描所有 column buffer。
- `SSTable` 打开后校验改为复用组件文件大小缓存，避免每个 block ref 重复 `storagefs.Stat`。

本节使用 `tests/scale/storage_10m` no-profile 口径，和上文 pprof 口径不同：scale 用例固定 `MemTableMaxSamples=8192`，会产生更多 SSTable；上文 pprof 用例使用 `memtable-max-samples=5000000`，SSTable 数更少但单次 MemTable 更大。因此本节主要用于确认写入耗时、RSS 和总分配趋势，不直接替代 pprof 热点分析。

### 执行命令

```bash
timeout 1200s go run ./tests/scale/storage_10m -profile standard -mode write -batch-size 4096
timeout 1200s go run ./tests/scale/storage_10m -profile soak -mode write -batch-size 4096
```

### 指标

| 口径 | points | duration | throughput | RSS peak | heap sys | total alloc | data bytes | SSTable count |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| standard write | `1,000,000` | `5.643s` | `177,206 points/s` | `40,685,568` | `40,894,464` | `6,008,170,464` | `98,042,141` | `245` |
| soak write | `10,000,000` | `52.163s` | `191,708 points/s` | `99,610,624` | `99,614,720` | `62,394,461,336` | `1,093,349,534` | `2442` |

### 判断

- 本批次后，scale 10M no-profile 写入耗时达到 `52.163s`，已经进入设计文档中的最终耗时目标 `<=60s`。
- RSS peak 为 `99.6MiB`，低于最终目标 `<=512MiB`。
- total alloc 为 `62.39GB`，仍高于最终目标 `<=20GB`，说明 public API map 构造、typed conversion、Catalog resolve、WAL 编码和 flush 物化仍需要继续优化。
- SSTable count 为 `2442`，明显不适合商用查询路径。该结果主要由 scale 用例固定 `MemTableMaxSamples=8192` 导致，后续必须推进更合理的 flush 策略、typed batch 和 compaction/part sizing gate。

## 2026-06-19 执行批次 2 后 typed write 结果

本批次继续针对 10M wide10 typed 写入热路径做结构优化：

- `WriteTypedBatch` 内部改为列式 resolved batch，不再展开为每行 `ResolvedPoint` 和每字段 `ResolvedField`。
- WAL 增加 typed batch encoder，保持现有 WAL record 格式和 replay 解码兼容。
- MemTable 增加 typed batch apply，直接从列式字段数组写入 column buffer。
- Catalog typed resolver 输出 `seriesIDs + resolved field columns`，并将 snapshot checkpoint 阈值随 series/field 规模自适应提升，降低高基数导入时的全量 snapshot 克隆。
- SSTable index row 编码复用 payload buffer，减少每个 index row 的临时编码分配。
- MemTable 的 `tableData`、`columnBuffer`、`columnKey` 使用有上限的显式 freelist，避免 `sync.Pool` 在频繁 GC 下丢失热对象。
- MemTable/SSTable 热排序从 `sort.Slice` 调整为 `slices.SortFunc`，减少反射分配。

### 10M scale gate

执行命令：

```bash
timeout 1200s go run ./tests/scale/storage_10m \
  -profile soak -mode write -batch-size 4096 \
  -ingest-path typed -data-dir /tmp/mts-typed-fast-10m-final
```

结果：

| 指标 | 批次 1 typed 优化后 | 批次 2 后 | 变化 |
| --- | ---: | ---: | ---: |
| duration | `27.250915837s` | `22.997944130s` | `-15.6%` |
| throughput | `366,960 points/s` | `434,821 points/s` | `+18.5%` |
| RSS peak | `87,433,216` | `83,591,168` | `-4.4%` |
| total alloc | `18,382,541,496` | `8,391,999,880` | `-54.3%` |
| mallocs | `40,160,019` | `37,445,836` | `-6.8%` |
| num GC | `1065` | `644` | `-39.5%` |
| data bytes | `1,093,350,356` | `1,093,350,356` | 持平 |
| SSTable count | `2442` | `2442` | 持平 |

判断：

- 10M typed scale 已满足最终商业目标中的耗时、RSS 和 total alloc 阈值。
- 本批次主要减少 transient heap churn，对落盘体积和 SSTable 数量没有改变。
- SSTable count 仍由当前 scale gate 的 `MemTableMaxSamples=8192` 决定，查询侧仍需要依赖 compaction 或更合理的写入配置控制读放大。

### 1M high-cardinality pprof

执行命令：

```bash
timeout 600s go run ./tests/pprof/storage_engine \
  -mode=write -field-layout=wide10 -points=1000000 -series=100000 \
  -write-batch-size=4096 -memtable-max-samples=8192 \
  -flush-on-exit -ingest-path=typed \
  -data-dir=/tmp/mts-typed-fast-pprof6/data \
  -mem-profile=/tmp/mts-typed-fast-pprof6/write.mem.prof
```

关键指标：

| 指标 | 批次 1 typed pprof | 批次 2 后 |
| --- | ---: | ---: |
| workload duration | `27.664s` | `24.820s` |
| RSS peak | `387,510,272` | `366,682,112` |
| total alloc | `5.976GB` | `2.678GB` |
| mallocs | 未记录 | `60,319,657` |
| num GC | 未记录 | `36` |

主要热点变化：

| 热点 | 批次 1 | 批次 2 后 | 说明 |
| --- | ---: | ---: | --- |
| `memtable.ensureColumn` | `820.54MB cum` | `16.21MB cum` | `tableData` 显式复用后，map bucket 反复分配基本消失 |
| `catalog.cloneTags` | `478.61MB` 量级 | `94.52MB` | 自适应 checkpoint 避免频繁全量 snapshot |
| `sstable.writeIndexBlocks` | `826.43MB cum` | `382.91MB cum` | index row payload buffer 复用 |
| `memtable.sortedColumnKeys` | `148.91MB` | 未进入 top | columnKey slice 复用 |
| `internal/reflectlite.Swapper` | `142.01MB` | `24MB` | 热路径排序改为 `slices.SortFunc` |

## 清理

本轮 pprof、数据目录和临时二进制位于 `/tmp/mts-10m-run`，报告写完后可删除，不应提交临时产物。
