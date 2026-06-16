# Storage Engine Phase Gap Analysis

## 背景

当前存储层已经完成 Phase 2-11 的核心建设：二进制 WAL/Catalog/SSTable/Manifest、append-only MemTable、SSTable v3 time ordinal、写入热路径优化、查询和 block I/O 分配优化。最近一次提交为：

```text
6ff40da perf(storage): 优化存储热路径分配
```

本分析只针对 phase 文档与当前实现之间的缺口，不直接修改存储实现。

## 已完成度判断

### 已基本完成

- 持久化格式从 JSON 切换为版本化二进制 envelope。
- WAL batch payload 二进制化，支持强同步、按记录数/字节数批量 fsync、segment 滚动与 replay。
- Catalog snapshot/WAL 二进制化，支持 series、field、schema cache 和常见 tag 快路径。
- MemTable 改为 append-only column buffer，flush 使用 swap snapshot。
- SSTable Part 使用 `metadata.bin`、`metaindex.bin`、`index.bin`、`timestamps.bin`、`values.bin` 的二进制分文件结构。
- SSTable value block v3 使用 row time ordinal，wide10 场景避免重复保存 timestamp。
- 查询读取复用 Part 文件句柄和 block frame buffer。
- query/compaction 合并已从常规 map 合并改为有序线性合并快路径。
- e2e、pprof 目录和基本性能文档已建立。

### 部分完成但还未产品化

1. **查询 API 与内存模型**
   - 原始设计写的是 `ColumnIterator` / `RowIterator`。
   - 当前公开 API 返回完整 `[]ColumnSeries` / `[]Row`。
   - `QueryRows` 使用 `map[rowKey]Row` materialize 全量结果，宽查询和长时间范围下仍会放大内存。

2. **查询并发边界**
   - 当前 `queryColumnData` 在遍历 shard 查询时持有 Engine 全局锁。
   - 这保证简单正确，但长查询会阻塞写入、flush、compaction 和 retention。

3. **SSTable block 内剪枝**
   - Phase 5 文档已指出“query 时仍需读取整块 value payload”。
   - 当前 index 只能剪枝到 series row 和 field value block，不能在 value block 内按时间页跳过。

4. **压缩策略**
   - 当前实现有 delta timestamp、v3 ordinal、bool bitset、string length encoding。
   - 原始设计里的 delta-of-delta、Gorilla/XOR、int bitpacking、string dictionary 仍未实现。
   - `model.Options` 还没有 compression 配置。

5. **Compaction 策略**
   - 当前有同步 size-tiered 起步实现和 flush 后 maybe compact。
   - `BackgroundInterval`、`MaxOutputPartBytes` 的产品化能力不足：没有后台调度循环，没有按目标大小拆分输出 Part。
   - orphan Part 清理策略还停留在“不暴露即可”，缺少维护任务。

6. **Retention 与 delete/tombstone**
   - 当前 retention 以 shard 为删除边界。
   - 原始设计预留了 `DeleteSeriesRecord`、`DeleteRangeRecord`，但删除/墓碑未实现。

7. **WAL 耐久策略**
   - 当前支持 `Sync`、`BatchRecords`、`BatchBytes`。
   - 原始设计提到 `BatchFsync{MaxRecords, MaxBytes, Interval}`，但 interval 型批量 fsync 未实现。
   - checkpoint 仍以 flush 后 truncate all 为主，没有 segment-level checkpoint 回收。

8. **metadata/database/measurement/retention policy 管理**
   - 当前只有默认 database/rp 和点写入时隐式创建。
   - 还没有显式 metadata API：创建/删除 database、measurement、retention policy，查询 schema，列出 series/fields。

9. **性能基线治理**
   - Phase 6-11 多数只有 benchmark 文档，没有对应 spec/plan。
   - 当前 benchmark 多使用原始输出，本机未安装 `benchstat`，缺少统计显著性比较。
   - pprof workload 仍会把造数成本混入 profile，Phase 5 已指出需要预生成 points 模式。

## 优先级建议

1. Phase 12：查询迭代器与锁边界。它直接降低读取峰值内存，也为后续 block page index 提供流式消费接口。
2. Phase 13：SSTable block page index。它解决“读整块 value block”的结构问题。
3. Phase 14：可配置 typed compression。它继续压缩落盘结构。
4. Phase 15：Compaction 后台调度与输出拆分。它关系到 LSM 长期运行稳定性。
5. Phase 16：WAL interval fsync、checkpoint 与 tombstone。它补齐耐久策略和删除能力。
6. Phase 17：metadata 管理 API。它是继续上层 engine/API 的前置能力。
7. Phase 18：benchmark/pprof 治理。它让性能优化可复现、可回归。

## 非目标

- 不在一个阶段内同时修改查询 API、SSTable 格式、compaction 调度和 metadata API。
- 不引入外部压缩库，除非单独评估压缩率、CPU、兼容风险。
- 不为了性能牺牲二进制格式兼容和崩溃恢复语义。
