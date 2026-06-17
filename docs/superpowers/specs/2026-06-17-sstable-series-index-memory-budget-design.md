# SSTable Series Index And Memory Budget Design

## 目标

用户问题本质：当前全时间范围单 series 查询会命中多个 SSTable，并在每个 SSTable 内扫描 index row；同时写入阶段缺少存储层总内存阈值，容易在小内存环境中让 RSS 峰值不可控。

## EARS 需求

- 当查询包含 `SeriesIDs` 过滤时，SSTable 应通过二级索引定位候选 index row，不应线性扫描完整 index。
- 当查询时间范围、字段集合或 series 集合与 SSTable 不相交时，SSTable 应在 part 级元数据阶段返回空流。
- 当存储层 MemTable 样本总量达到软阈值时，Engine 应主动 flush 所有 shard，释放 MemTable 内存。
- 当 flush 后 MemTable 样本总量仍超过硬阈值时，Engine 应返回明确错误，防止写入路径继续扩大 RSS。
- 当未配置内存阈值时，Engine 应保持现有行为。

## 设计

SSTable 新增独立二进制文件 `series_index.bin`，每条记录保存 `series_id -> index row blockRef + min/max time + fieldIDs`。写入 part 时，`index.bin` 从单块改为每个 series 一个 index row block；metadata 持有 series index 的 blockRef。读取时，如果 query 带有 `SeriesIDs`，先读 series index，仅打开命中的 row block，再读取时间和值页。未带 series 过滤时仍按 metaindex/index 顺序扫描。

内存阈值采用 Engine 级配置 `StorageMemoryOptions`。`SoftSampleLimit` 表示所有 shard MemTable 样本总量达到该值后主动 flush；`HardSampleLimit` 表示 flush 后仍超过阈值则拒绝写入。这里使用样本数作为第一阶段可控指标，避免依赖平台 RSS 采样的滞后与噪声。

## 验证

- SSTable 单元测试覆盖二级索引写入、读取、未知 series 返回空结果。
- Engine 单元测试覆盖软阈值触发 flush、硬阈值报错。
- 定向执行 `go test ./internal/sstable ./internal/engine ./... -timeout 10m`，必要时补跑 pprof 读写用例。
