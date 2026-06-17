# MemTable Columnar Memory Design

## 背景

100K wide10 写入性能测试中，RSS peak 受两类内存影响：压测工具预生成 points 的输入内存，以及引擎内部 MemTable/WAL/flush 编码内存。补充测试表明去掉预生成 points 后 RSS peak 从约 439 MiB 降到约 175 MiB；降低 `memtable-max-samples` 后进一步降到约 58 MiB。因此需要先隔离测量口径，再优化 MemTable 的驻留结构。

## EARS 需求

- 当 pprof 工具启用 `-prebuild-points` 时，系统应在预生成完成后记录输入数据持有造成的内存基线，并在 workload 前后分别输出指标。
- 当 pprof 工具运行 write/read/query/compact/replay 时，系统应输出 workload 前、workload 后、可选 profile 后的 heap/RSS 指标，便于区分造数内存和引擎内存。
- 当 MemTable 接收 float64、int64、string、bool 样本时，系统应按列分类型存储 timestamps、writeSeqs 和 typed values，不应把每个样本长期存为 `VersionedSample` + `FieldValue` 结构体。
- 当 MemTable flush 或 query 需要 `model.ColumnData` 时，系统应按需 materialize `[]VersionedSample`，保持 engine/sstable 边界契约不变。
- 当 bool 样本写入 MemTable 时，系统应使用 bitset 驻留，避免每个 bool 样本占用一个完整 `FieldValue`。
- 当 string 样本写入 MemTable 时，系统应保留 Go string 值并避免复制字符串内容；后续 SSTable dictionary/payload compression 负责落盘压缩。
- 当 Snapshot 释放时，系统应清空 typed slices 和 bitset，允许 GC 回收或复用 table 容器。
- 当已有乱序写入、重复 timestamp、writeSeq 去重、query 过滤、snapshot restore 流程运行时，系统应保持现有语义。

## 方案

### 方案 A：MemTable typed columnar layout

`columnBuffer` 改为：

```go
timestamps []int64
writeSeqs  []uint64
floats     []float64
ints       []int64
strings    []string
bools      []uint64
count      int
```

append 时按字段类型写入对应 value slice。query/flush 时再 materialize `VersionedSample`。这是本次采用方案。

优点：内存布局接近真实时序数据库 head/memtable；数值列更紧凑；bool 极小；对 sstable/engine 外部接口侵入小。缺点：查询/flush materialize 时仍会产生临时 `VersionedSample`，后续可继续把 SSTable writer 改成 typed column reader 来进一步减少峰值。

### 方案 B：立即修改 engine/sstable 边界为 typed column reader

直接让 MemTable 输出 typed iterator，SSTable writer 不再接收 `[]VersionedSample`。

优点：最终内存与 CPU 更优。缺点：影响 engine、compaction、sstable、query 多层接口，当前改动面过大，风险高。

### 方案 C：保留 MemTable 结构，仅调小 flush 阈值

优点：改动小。缺点：没有解决结构性内存浪费，只是把峰值转移为更多 SSTable 和 compaction 压力。

## 验收

- MemTable 单元测试覆盖四种类型 round trip、bool bitset、Snapshot/Restore、query filter、重复 timestamp 合并语义。
- pprof 工具测试覆盖 prebuild metrics、compression algorithm、read mode。
- 100K wide10 无 prebuild、`memtable-max-samples=1200000` 写入 RSS peak 应低于当前约 175 MiB 基线。
- 100K wide10 无 prebuild、`memtable-max-samples=100000` 写入 RSS peak 应不高于当前约 58 MiB 基线。
- `go test ./...`、`golangci-lint`、`tests/e2e` 全部通过。

