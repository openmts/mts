# 商业级写入性能优化专项设计

## 背景

当前 10M wide10 写入测试结果不可接受：`10M points / 100M samples` 写入 workload 耗时 `520.663s`，进程总耗时 `8:49.53`，RSS peak `1.84GiB`，累计分配 `108.46GB`。主热点不是 WAL，而是 MemTable 内存估算、MemTable/flush 物化、public API map 转换、Catalog 解析和 SSTable 打开校验。

参考主流 TSDB 的共同做法：

- Prometheus 将 head block 保持在内存并用 WAL 保护，落盘 block/chunk 使用紧凑格式，容量规划通常按 bytes/sample 估算，而不是按对象图堆积。
- VictoriaMetrics 强调高压缩、后台 merge、资源限制和高效本地目录布局。
- InfluxDB TSM/TSI 的思路是 WAL/cache 与 TSM 文件分层，compaction 后形成紧凑列式文件。

本专项目标是把 mts 写入链路从当前的 map/object-oriented 热路径，改造成 batch/columnar/streaming 的商业级写入路径。

## 目标

本专项以当前 Fedora 44、16 core、12GiB 内存机器为基准，使用 `tests/pprof/storage_engine` 或等价 scale gate，测试 `10M points / 100K series / wide10 / 100M samples`。

第一阶段验收目标：

- 10M write no-profile 总耗时 `<= 120s`。
- 10M write RSS peak `<= 768MiB`。
- 10M write total alloc `<= 35GB`。
- `memtable.(*columnBuffer).approxMemoryBytes` 不再出现在 CPU top 10。
- `os.statNolog` 不再出现在 write alloc_space top 10。
- 数据正确性、WAL recovery、flush atomicity、e2e 全部保持通过。

最终商业目标：

- 10M write no-profile 总耗时 `<= 60s`。
- 10M write RSS peak `<= 512MiB`。
- 10M write total alloc `<= 20GB`。
- 落盘体积不高于当前无压缩基线；启用压缩后显著下降。

## 方案选择

推荐选择结构性改造，而不是只调参数。

- 方案 A：只降低 MemTable 阈值。可以降低 RSS，但会制造大量 SSTable，写放大和读放大会恶化，不能达到商业目标。
- 方案 B：重构写入热路径。新增 O(1) 内存计数、series-wide MemTable、streaming flush、typed batch fast path、SSTable 校验去 syscall。改动范围可控，能保持现有 API 兼容，是推荐方案。
- 方案 C：重写存储引擎。长期可能更干净，但当前已有 WAL/MemTable/SSTable/compaction/query 基础，重写会丢掉大量已验证能力，不适合作为下一步。

## EARS 任务清单

### Task 1：建立 10M 写入基线门禁

- When 运行 10M wide10 写入基准时，系统应支持 no-profile、cpu-profile、mem-profile 三种独立模式，避免 profile 互相污染。
- When benchmark 输出报告时，系统应拆分 `build point`、`public API conversion`、`metadata resolve`、`WAL append`、`MemTable apply`、`flush`、`SSTable open/validate` 的耗时和内存指标。
- When benchmark 结束时，系统应输出 `rss_peak_bytes`、`heap_sys_bytes`、`heap_alloc_after_gc_bytes`、`total_alloc_bytes`、`mallocs`、`num_gc`、`sstable_count`、`data_dir_bytes`。
- When benchmark 结束时，系统应按文件类型输出磁盘空间分布，包括 `values.bin`、`index.bin`、`timestamps.bin`、`series_index.bin`、WAL 和 catalog。
- If benchmark 失败、超时或超过阈值，系统应输出机器可读 JSON 报告并返回非零退出码。
- 验收：新增/扩展 scale gate 后，能复现当前 10M 写入基线，并能在后续任务中自动对比 before/after。

### Task 2：MemTable 内存估算 O(1) 化

- When 写入 sample 到 MemTable 时，系统应增量维护 MemTable 当前字节数，而不是通过 `approxTableDataBytes` 全量遍历所有列。
- When slice reserve/grow 发生时，系统应按 old capacity 和 new capacity 的差值更新内存计数。
- When string sample 写入时，系统应把字符串 payload 长度计入内存估算。
- When bool bitset 扩容时，系统应按 bitset backing array 增量计数。
- When SnapshotAndReset 执行时，系统应把当前内存字节数转移到 Snapshot，并把新 MemTable 计数重置。
- When Snapshot.Release 执行时，系统应清理计数并释放 backing references。
- If Restore(snapshot) 被调用，系统应恢复 sample count 和 memory bytes，保证 flush 失败后内存治理仍正确。
- 验收：10M write CPU top 中 `columnBuffer.approxMemoryBytes` 和 `approxTableDataBytes` 不再进入 top 10。

### Task 3：MemTable 从 series-field 小列对象改为 series-wide 结构

- When 写入 wide10 point 时，系统应按 `seriesID` 聚合到 series buffer，而不是为每个 `seriesID + fieldID` 创建大量小 `columnBuffer`。
- When 同一 point 包含多个 fields 时，系统应只存储一次 timestamp 和 writeSeq，并为每个 field 存储对应 typed value。
- When series buffer 首次看到 fieldID 时，系统应创建 field slot，并记录 field type。
- When field 缺失时，系统应能表示该时间点没有该 field 值，不能生成错误样本。
- When flush/query 需要列式输出时，系统应从 series-wide buffer 以 iterator 方式生成 `(seriesID, fieldID)` 列视图。
- If out-of-order 或重复 timestamp 写入发生，系统应保留现有 writeSeq 覆盖语义。
- 验收：100K series * 10 fields 场景下，MemTable 对象数应接近 series 数和字段数的组合结构，不再接近百万级 columnBuffer 小对象。

### Task 4：Flush 改为真正 streaming 输出

- When MemTable flush 触发时，系统应从 snapshot 创建 column/series iterator，而不是一次性构造完整 `[]model.ColumnData`。
- When PartWriter 写入时，系统应逐 series 或固定 batch series 写出数据，单批内存应受配置上限控制。
- When flush 成功写完 part 并提交 manifest 后，系统应释放 snapshot backing references。
- If part 写入失败、manifest 写入失败或 WAL checkpoint 失败，系统应保留或恢复 snapshot，保证数据不丢。
- When flush 失败并恢复 MemTable 时，系统应恢复 sample count、memory bytes 和 series buffers。
- When flush 正在执行时，新写入应进入新 MemTable，不应与正在 flush 的 snapshot 共享可变 backing array。
- 验收：10M write 的 RSS peak 应显著低于当前 `1.84GiB`；flush 阶段不再出现 `materializeSortedColumnSamples` GB 级分配。

### Task 5：新增商业写入 fast path，绕开 map-heavy public API

- When 调用新的 batch ingestion API 时，系统应允许调用方用 typed batch/builder 写入 measurement、series tags、timestamps 和 typed field arrays。
- When 使用 fast path 时，系统不应为每个 point 构造 `map[string]FieldValue`。
- When 使用 fast path 时，系统不应为每个 point clone tags map。
- When 使用 fast path 时，系统应明确输入所有权：写入调用返回前可以借用输入；返回后不能保留外部可变切片引用，除非 API 文档声明 builder 交权。
- If 调用方继续使用现有 `Write([]Point)` API，系统应保持兼容和安全 clone 语义。
- When pprof/scale 工具运行 10M 写入时，系统应支持 `-ingest-path=public|typed`，默认使用 typed 路径评估商业写入能力。
- 验收：10M write alloc_space 中 `main.wide10WorkloadPoint`、`toModelPoint`、`cloneStringMap` 不再是 typed 路径的 top 10 热点。

### Task 6：Catalog 批量解析和字典缓存优化

- When batch 内多个 point 属于同一 measurement 和相同 field set 时，系统应复用 fieldID 解析结果。
- When series tags 是单 tag 且 tag key 稳定时，系统应走无排序、无临时字符串拼接的 series lookup fast path。
- When tags 为多 tag 时，系统应使用可复用 scratch 或结构化 key，避免每条 point 都分配新 key。
- When batch 中已有 seriesID 时，系统应跳过重复 series key 构建。
- If schema 冲突发生，系统应返回明确错误并保持 batch 原子性。
- 验收：10M write alloc_space 中 `catalog.ResolvePoints` 明显下降，且 catalog schema/series 测试全部通过。

### Task 7：WAL typed batch 编码和 buffer 复用

- When typed ingestion path 写入 WAL 时，系统应直接编码 typed batch，不应先退化为 map-based point。
- When batch 中 series 和 field 字典重复时，系统应只编码一次 identity 和 field name，并用 ref 引用。
- When WAL encoder 处理连续 batch 时，系统应复用 scratch buffer，避免每 batch 大量重新分配。
- When WAL frame 超过 segment 或 batch 阈值时，系统应按现有 segment 策略切分，保证 replay 正确。
- If WAL append 或 fsync 失败，系统应返回错误且不更新 MemTable。
- 验收：10M write 中 `wal.encodeBatch` 和 `wal.batchIdentities` 不再进入 alloc_space top 10。

### Task 8：SSTable 写后打开和校验去系统调用化

- When part 刚由当前进程写出并已获得 metadata 时，系统应使用已知 block refs 和组件文件大小完成轻量打开，不应执行完整 cold-open 校验。
- When cold-open 已存在 part 时，系统应在打开组件文件时缓存 `index/timestamps/values/series_index/metaindex` 文件大小。
- When 校验 block ref 是否越界时，系统应使用缓存文件大小，不应每个 block ref 调用 `Stat`。
- When 校验 value page refs 时，系统应批量校验边界，并避免重复读取同一 index payload。
- If 文件被截断、缺失或 checksum 错误，系统仍应在 cold-open 或读取时检测并返回明确错误。
- 验收：10M write/read 中 `os.statNolog`、`syscall.ByteSliceFromString`、`storagefs.Stat` 不再是 top alloc/CPU 热点。

### Task 9：SSTable index/value page 元数据加载降分配

- When 打开 part 时，系统应懒加载或 view 解码大型 index/value page metadata，避免把所有 index rows 复制成独立对象。
- When 查询只命中指定 seriesID 时，系统应只解码命中的 series index row 和必要 field refs。
- When value page index 只用于定位 page 时，系统应优先使用 zero-copy view 或 scratch arena。
- When Part.Close 执行时，系统应释放 cached view 和 file handles。
- 验收：10M read cold-open alloc_space 从当前 `20GB` 量级大幅下降，`sstable.unmarshalValuePageIndex` 和 `decodeIndexRows` 不再是主要分配源。

### Task 10：自适应 flush 和内存背压

- When MemTable active bytes 达到软阈值时，系统应触发 flush，而不是仅依赖 sample count。
- When series cardinality 很高且每列样本很少时，系统应降低单 MemTable 目标样本数，避免对象图膨胀。
- When flush 速度低于写入速度时，系统应通过 backpressure 或硬阈值拒绝写入，避免 RSS 失控。
- When配置 `StorageMemoryOptions` 时，系统应允许设置全局 storage memory soft/hard limit，并在 pprof/scale 用例中暴露。
- If 达到 hard limit，系统应返回 `ErrStorageMemoryLimitExceeded`，不能 OOM。
- 验收：在小内存限制场景下，10M 写入应失败为可控错误或稳定完成，不能被 OS OOM kill。

### Task 11：写入并发流水线

- When batch 写入进入 engine 时，系统应把解析、WAL append、MemTable apply 和 flush I/O 解耦为有界流水线。
- When WAL durability 策略为 async/batched fsync 时，系统应合并多个 batch 的 fsync。
- When MemTable flush 执行时，系统应允许新 MemTable 接收后续写入。
- When flush backlog 超过阈值时，系统应触发 backpressure。
- If 同一 shard 内存在乱序写或覆盖写，系统应保持 writeSeq 单调和查询语义正确。
- 验收：10M write CPU 利用率应显著高于当前 `127%`，在 16 core 机器上至少能稳定利用多个核心而不引入数据竞争。

### Task 12：商业级 10M/100M 回归门禁

- When 每次完成写路径优化时，系统应运行 100K、1M、10M 三档基准，10M 可作为手动 gate。
- When 运行 10M no-profile gate 时，系统应输出 JSON 报告并写入 `docs/benchmarks`。
- When 运行 10M pprof gate 时，系统应采集 CPU、alloc_space、alloc_objects，并自动提取 top 20。
- When 性能不达标时，系统应在报告中列出阻塞热点和下一步动作。
- If 基准生成临时数据、profile 或二进制，系统应在报告完成后清理。
- 验收：达到第一阶段目标后，提交性能报告；达到最终商业目标后，更新 README 或 benchmark guide 的推荐写入配置。

## 实施顺序

1. Task 1：先把测量口径固定。
2. Task 2：先消除 MemTable 内存估算 CPU 热点，风险低、收益明确。
3. Task 8：并行处理 SSTable `Stat` 系统调用，降低写后打开和冷启动读成本。
4. Task 4：把 flush 从全量物化改为 streaming，这是 RSS 降低的关键。
5. Task 3：把 MemTable 结构升级为 series-wide，降低高基数下对象数量。
6. Task 5、6、7：构建 typed ingestion fast path，降低 map 和 WAL 编码分配。
7. Task 9、10、11：完善索引加载、内存背压和写入流水线。
8. Task 12：用 10M/100M gate 固化结果。

## 非目标

- 不牺牲 WAL crash recovery。
- 不牺牲乱序写和 writeSeq 覆盖语义。
- 不移除现有 public `Write([]Point)` API。
- 不通过关闭校验来掩盖数据损坏；校验可以分层和缓存，但不能消失。

## 实施状态

### 2026-06-18 执行批次 1

- 已完成 Task 2 的第一阶段实现：`MemTable.ApproxMemoryBytes()` 和 `Snapshot.ApproxMemoryBytes()` 改为 O(1) 增量计数，不再全量扫描所有 column buffer。
- 已完成 Task 2 的一致性保护：新增测试覆盖 active、snapshot、restore 后的增量计数必须等于旧的全量计算口径。
- 已完成 Task 8 的第一阶段实现：`OpenPart` 时缓存 SSTable 组件文件大小，metadata/index/time/value/page ref 校验复用缓存尺寸，不再对每个 block ref 执行 `storagefs.Stat`。
- 已完成 Task 8 的完整性保护：缺失组件、越界 metadata ref、checksum 损坏仍由现有测试覆盖；新增 `validateBlockRefWithinSize` 边界测试。
- 已验证命令：`go test ./internal/memtable -count=1 -timeout 120s`。
- 已验证命令：`go test ./internal/memtable ./internal/sstable ./internal/engine -count=1 -timeout 300s`。
- 未完成项保持待实施：streaming flush、series-wide MemTable、typed ingestion fast path、Catalog batch cache、WAL typed batch、自适应 flush/backpressure、写入流水线、10M/100M 回归门禁。

### 2026-06-19 执行批次 2

- 已完成 Task 4 的 streaming flush：`Snapshot.ForEachSeries` 和 `PartWriter.AddSeries` 支持逐 series 写出，不再物化完整 `[]ColumnData`。
- 已完成 Task 5 的 typed ingestion fast path：`WriteTypedBatch` 内部改为 `ResolvedTypedBatch`，保留列式字段数组和 `seriesIDs`，避免行式 `ResolvedPoint`/`ResolvedField` 展开。
- 已完成 Task 6 的 typed Catalog 批量解析：新增 `ResolveTypedBatchColumns`，字段定义按列解析，seriesID 按行解析，字段值切片借用输入 batch。
- 已完成 Task 6 的高基数 checkpoint 优化：Catalog snapshot checkpoint 阈值随 series/field 规模增长，避免导入 100K series 时频繁全量克隆。
- 已完成 Task 7 的 WAL typed batch 编码：`AppendTyped` 直接编码 typed batch，WAL record 格式仍兼容现有 replay。
- 已完成 Task 8 的 warm open：flush 和 compaction 产物写后打开使用 `OpenPartTrusted`，cold open 仍保留深度校验。
- 已完成 Task 12 的 10M scale gate：`10M / wide10 / typed / batch=4096` 写入为 `22.997944130s`，RSS peak `83,591,168 bytes`，total alloc `8,391,999,880 bytes`。
- 已补充内存复用优化：MemTable `tableData`、`columnBuffer`、`columnKey` 使用有上限的显式 freelist；热排序使用 `slices.SortFunc`。
- 已补充 SSTable index 编码优化：`encodeIndexRowsInto` 支持复用输出 buffer，`writeIndexBlocks` 复用 per-row payload。
- 已验证命令：`go test ./internal/catalog ./internal/wal ./internal/memtable ./internal/engine . ./tests/scale/storage_10m ./tests/pprof/storage_engine -count=1`。
- 已验证命令：`go test ./internal/sstable -count=1`。
- 已验证命令：`go run ./tests/scale/storage_10m -profile soak -mode write -batch-size 4096 -ingest-path typed`。
- 当前仍未由本批次改变的项：series-wide MemTable 物理结构尚未替代 series-field column map；写入并发流水线尚未实施；10M scale 默认仍生成 `2442` 个 L0 SSTable，需要通过 compaction/flush 配置治理读放大。

## 待确认

本设计默认商业目标以单机 16 core、12GiB 内存为基准，10M wide10 no-profile 写入最终目标为 `<=60s`，RSS peak `<=512MiB`。如果目标机器规格或目标耗时不同，需要在实施计划中调整阈值。
