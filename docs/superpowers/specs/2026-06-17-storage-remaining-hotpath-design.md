# Storage Remaining Hotpath Design

## 背景

Phase 12-18、hotpath allocation、P1 performance 和 single-format 已完成。上一轮只读检视确认仍有 5 类优化空间：SSTable indexed/compressed page streaming、Catalog 多 tag key 分配、MemTable 查询 clone、value page index 两遍扫描自适应、pprof 指标补齐。本轮目标是一次性处理这些剩余项，不引入新的持久化版本兼容逻辑。

## EARS 需求

- 当读取 SSTable aligned plain value page 时，系统应继续直接填充目标 samples，不应构造 typed values 中间切片。
- 当读取 SSTable indexed plain value page 时，系统应直接按 ordinal、writeSeq 和 typed values 追加命中 samples，不应同时保留 timestamps、writeSeqs 和 samples 三段中间切片。
- 当读取 compressed page 的 plain value codec 时，系统应支持把 typed values 直接填入已构造的 samples，避免额外 `[]FieldValue` 中间切片。
- 当 Catalog 在同一批次解析重复多 tag series 时，系统应复用 canonical key 构造缓冲和 batch cache，减少 keys/parts 临时 slice 与字符串拼接分配。
- 当查询 MemTable 当前数据时，系统应在读锁内直接扫描 tableData 并构造查询结果，不应先 clone 整个 tableData。
- 当读取 value page index 时，系统应保留窄查询低内存两遍扫描路径；当查询命中全部 page 或绝大多数 page 时，系统应走单遍读取路径减少 CPU 扫描。
- 当运行 `tests/pprof/storage_engine` 时，系统应输出 GC、总分配、malloc/free、RSS 和 VmHWM 指标，便于后续性能回归对比。

## 设计

### SSTable Page Streaming

保留当前 page kind，不改变落盘格式。plain page 解码拆成 aligned 与 indexed 两条路径：

- aligned：沿用当前 `readAlignedSamples` + `fillAligned*Values`。
- indexed：新增直接读取 ordinal 和 writeSeq 的路径，把 timestamp/writeSeq 暂存在目标 samples，仅对 query 命中的样本 append；随后按 field type 读取 values 并对命中样本原地填值。

compressed page 暂不重写压缩 payload 结构。对 plain value codec 增加直接填充 helper；非 plain codec 仍按现有解压 slice 读取，避免一次性改动 XOR/delta/dictionary 的稳定路径。

### Catalog Key Scratch

Catalog 多 tag key 的稳定字符串仍是 `seriesByKey` 的 map key，不能用非稳定 byte slice。优化点放在批量 resolve：

- `resolveBatchCache` 增加 `scratch []string`，复用 tag keys buffer。
- `seriesKeyWithScratch` 对多 tag 只分配最终 key，不再每次分配 keys 和 parts 两个 slice。
- 单点路径继续使用 `seriesKey`，它内部也改为同一个低分配 builder。

### MemTable Query Without Clone

`Snapshot()` 保持复制语义，用于调用方需要稳定快照的场景。`MemTable.Query` 改为：

- 持有 RLock。
- 直接调用共享的 `columnsFromData(m.data, query)`。
- 该函数会为返回结果构造新的 `[]ColumnData` 和样本切片，不暴露内部 tableData 的可变 map。

### Value Page Index Adaptive Scan

当前两遍扫描适合窄查询，因为不物化 pages。新增 fast path：

- 第一遍读取 header 后，如果 query 覆盖 page index 的整体 min/max 且 pageCount 大于 0，直接单遍读取所有 pages。
- 对部分命中仍保留两遍扫描，控制 RSS 和临时对象。

### Pprof Metrics

扩展 `runMetrics`：

- `heap_alloc_bytes`
- `heap_sys_bytes`
- `total_alloc_bytes`
- `mallocs`
- `frees`
- `num_gc`
- `pause_total_ns`
- `rss_bytes`
- `rss_peak_bytes`

Linux 环境下从 `/proc/self/status` 读取 `VmRSS` 和 `VmHWM`。读取失败时输出 0，不让指标采集影响 workload 成败。

## 验证策略

- 定向测试：
  - `go test -count=1 ./internal/sstable -run 'TestValuePage|TestCompressed' -timeout 180s`
  - `go test -count=1 ./internal/catalog -run 'TestSeriesKey|TestResolvePoints' -timeout 180s`
  - `go test -count=1 ./internal/memtable -run 'TestMemTable|TestQuery' -timeout 180s`
  - `go test -count=1 ./tests/pprof/storage_engine -timeout 180s`
- 核心回归：`go test -count=1 ./internal/wal ./internal/sstable ./internal/catalog ./internal/memtable ./internal/engine -timeout 180s`
- 全量：`go test -count=1 ./... -coverprofile=coverage.out -timeout 600s`，总覆盖率不低于 90.0%。
- Lint：`golangci-lint run --timeout 12m`。
- E2E：逐个 build/run `tests/e2e/*` 并清理二进制。

## 非目标

- 不改变当前 SSTable/WAL/Catalog 持久化格式。
- 不恢复任何版本兼容读取分支。
- 不引入无上限对象池。
- 不修改公开 API。
