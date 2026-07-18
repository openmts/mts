# 删除（Tombstone）物理回收说明

## 语义

1. `Delete` / 范围删除写入 **逻辑 tombstone**（WAL + shard 内存列表）。
2. 查询路径在读时过滤 tombstone，保证读侧正确性。
3. **物理回收**发生在 compaction：合并时丢弃被删除样本，并安全删除旧 part。

## 期望 SLA（单机本地）

| 阶段 | 期望 |
| --- | --- |
| 逻辑删除生效 | 写入 tombstone 成功后，后续查询立即不可见 |
| 物理空间回收 | 依赖后台/触发 compaction；无全局固定时限 |
| 可观测 | 见下方 metrics |

生产侧若需加速回收，可主动触发 `Compact` / 等待 `BackgroundInterval`。

## 关键 Metrics

| Metric | 含义 |
| --- | --- |
| `mts_tombstones_pending` | 当前 pending 逻辑 tombstone 条数 |
| `mts_compaction_dropped_rows_total` | compaction 丢弃行（含 tombstone 回收） |
| `mts_compaction_safe_delete_parts_total` | manifest 提交后安全删除的输入 part 数 |
| `mts_compaction_backlog` / `mts_maintenance_compaction_*` | 维护积压与并发 |

## 告警建议

- `mts_tombstones_pending` 长时间持续升高且 compaction backlog 不降：检查磁盘/内存 soft limit 是否跳过 compact。
- `mts_compaction_errors_total` 上升：检查维护错误与恢复审计。
