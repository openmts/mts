# 存储层 Metrics

本文档列出存储层当前暴露的生产运维指标。指标不携带 series、measurement、tag value 等高基数字段。

## WAL

| 指标 | 类型 | 含义 | 告警建议 |
| --- | --- | --- | --- |
| `mts_wal_append_records_total` | counter | WAL 追加记录数 | 写入流量异常下降时排查上游和写入拒绝 |
| `mts_wal_append_errors_total` | counter | WAL 追加失败数 | 大于 0 持续增长告警 |
| `mts_wal_append_latency_seconds_sum/count` | counter | WAL 追加累计耗时和次数 | 分位延迟需结合外部 PromQL 计算 |
| `mts_wal_sync_total` | counter | fsync 次数 | 与写入策略对齐，异常增高排查批量参数 |
| `mts_wal_sync_errors_total` | counter | fsync 失败数 | 大于 0 立即告警 |
| `mts_wal_sync_latency_seconds_sum/count` | counter | fsync 累计耗时和次数 | 延迟持续升高排查磁盘 |
| `mts_wal_checkpoint_total` | counter | checkpoint 次数 | 长时间不增长且 WAL pending 增长时告警 |
| `mts_wal_checkpoint_errors_total` | counter | checkpoint 失败数 | 大于 0 告警 |
| `mts_wal_replay_records_total` | counter | replay 记录数 | 重启后用于恢复量评估 |
| `mts_wal_replay_errors_total` | counter | replay 失败数 | 大于 0 立即告警 |
| `mts_wal_segments` | gauge | 当前 WAL segment 数 | 持续增长排查 checkpoint |
| `mts_wal_pending_records` | gauge | 未 fsync 记录数 | 接近批量阈值长期不下降时告警 |
| `mts_wal_pending_bytes` | gauge | 未 fsync 字节数 | 接近内存预算时告警 |

## MemTable 和 SSTable

| 指标 | 类型 | 含义 | 告警建议 |
| --- | --- | --- | --- |
| `mts_memtable_samples` | gauge | 当前 MemTable sample 数 | 接近硬阈值告警 |
| `mts_memtable_estimated_bytes` | gauge | 当前 MemTable 估算内存 | 接近内存预算告警 |
| `mts_memtable_series` | gauge | 当前 MemTable series 数 | 基数快速增长时排查 tag 维度 |
| `mts_memtable_fields` | gauge | 当前 MemTable field 数 | 字段数异常增长时排查 schema |
| `mts_memtable_columns` | gauge | 当前 MemTable typed column 数 | column 数持续增长时排查 series/field 基数 |
| `mts_memtable_flush_triggered_total` | counter | 内存压力触发 flush 次数 | 快速增长说明写入内存预算偏小 |
| `mts_sstable_parts` | gauge | manifest 引用的 SSTable 数 | 超过读放大预算告警 |
| `mts_sstable_rows` | gauge | SSTable 行数 | 容量趋势 |
| `mts_sstable_series` | gauge | SSTable series 数 | 基数趋势 |
| `mts_sstable_blocks` | gauge | SSTable block 数 | 读放大趋势 |
| `mts_sstable_max_level` | gauge | 当前最高 compaction level | 层级异常停留排查 compaction |
| `mts_sstable_level0_parts` | gauge | L0 part 数 | 超过 L0 阈值告警 |
| `mts_sstable_max_write_seq` | gauge | SSTable 最大写序号 | 恢复和持久化进度辅助判断 |
| `mts_sstable_data_bytes` | gauge | SSTable data 组件字节数 | 结合 logical bytes 判断压缩效果 |
| `mts_sstable_index_bytes` | gauge | SSTable index/metadata 组件字节数 | 过高时排查 block/page 切分 |
| `mts_sstable_total_bytes` | gauge | SSTable 总组件字节数 | 容量趋势 |
| `mts_sstable_compression_ratio` | gauge | 估算逻辑数据字节 / data 组件字节 | 低于预期时排查压缩策略 |

## Query、Compaction、Retention、Recovery

| 指标 | 类型 | 含义 | 告警建议 |
| --- | --- | --- | --- |
| `mts_query_samples_returned_total` | counter | 最近查询返回样本数 | 与查询流量结合判断 |
| `mts_query_samples_read_total` | counter | 最近查询读取样本数 | 与 returned 差距大时排查过滤和读放大 |
| `mts_query_parts_scanned_total` | counter | 最近查询扫描 part 数 | 超过预算告警 |
| `mts_query_parts_skipped_total` | counter | 最近查询跳过 part 数 | 用于确认索引过滤效果 |
| `mts_query_errors_total` | counter | 最近查询错误数 | 大于 0 持续增长告警 |
| `mts_query_duration_seconds_sum/count` | counter | 最近查询累计耗时和次数 | 延迟升高时结合 stats 定位读放大 |
| `mts_query_budget_errors_total` | counter | 查询预算错误数 | 大于 0 说明查询预算触顶 |
| `mts_query_cancellations_total` | counter | 查询取消和 deadline 次数 | 持续增长排查客户端或超时配置 |
| `mts_compaction_active` | gauge | 活跃 compaction 任务数 | 长期为 0 且 backlog 增长时告警 |
| `mts_compaction_backlog` | gauge | 待 compaction 计划数 | 超过 degraded 阈值告警 |
| `mts_compaction_errors_total` | counter | compaction 失败数 | 大于 0 告警 |
| `mts_retention_expired_parts_total` | counter | retention 删除的 part 数 | 删除失败需结合日志和 recovery 指标 |
| `mts_retention_deleted_bytes_total` | counter | retention 删除字节数 | 容量回收趋势 |
| `mts_retention_delete_errors_total` | counter | retention 删除失败数 | 大于 0 告警 |
| `mts_recovery_issues_total` | counter | recovery 发现的问题数 | 大于 0 排查启动恢复日志 |
| `mts_recovery_errors_total` | counter | 带底层错误的 recovery 问题数 | 大于 0 告警 |
| `mts_recovery_fatal_errors_total` | counter | 致命 recovery 问题数 | 大于 0 立即告警 |

## Memory 和 Runtime

| 指标 | 类型 | 含义 | 告警建议 |
| --- | --- | --- | --- |
| `mts_storage_memory_current_bytes` | gauge | 存储层估算当前内存 | 接近 hard limit 告警 |
| `mts_storage_memory_peak_bytes` | gauge | 存储层估算峰值内存 | 容量规划 |
| `mts_storage_memory_runtime_rss_bytes` | gauge | 进程 RSS | 接近容器/机器限制告警 |
| `mts_storage_memory_runtime_gap_bytes` | gauge | RSS 与存储层估算差值 | 持续扩大排查非存储分配 |
| `mts_storage_memory_rejected_writes_total` | counter | 内存预算拒绝写入次数 | 大于 0 告警 |
| `mts_runtime_heap_alloc_bytes` | gauge | Go heap alloc | GC 和内存趋势 |
| `mts_runtime_heap_inuse_bytes` | gauge | Go heap in-use | 堆占用趋势 |
| `mts_runtime_gc_total` | counter | GC 次数 | 与延迟结合判断 GC 压力 |
| `mts_runtime_gc_pause_total_seconds` | counter | GC 累计暂停秒数 | pause 增速异常时排查分配热点 |
| `mts_runtime_goroutines` | gauge | goroutine 数 | 持续增长排查泄漏 |
| `mts_runtime_fd_open` | gauge | 打开文件描述符数 | 接近系统限制告警 |
