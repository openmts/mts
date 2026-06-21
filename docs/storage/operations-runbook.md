# MTS 单机运维 Runbook

## 安全默认

- `/metrics`、`/healthz`、`/readyz` 可直接暴露给本机或受控内网采集面。
- `/debug/pprof/*` 默认关闭，只有显式设置 `EnablePprof=true` 时注册。
- `/admin/compact` 默认关闭，只有显式设置 `EnableAdmin=true` 且配置 `AdminToken` 时注册。
- 管理请求必须携带 `Authorization: Bearer <token>` 或 `X-MTS-Admin-Token: <token>`。
- `/admin/compact` 必须配置 `AdminTimeout` 或请求上下文 deadline，避免长时间占用管理链路。

## Metrics 与 Alert

建议至少采集以下指标并设置 alert：

- `mts_storage_memory_*`：存储层内存预算、已用量、拒绝次数。
- `mts_compaction_backlog`：compaction backlog，持续高于阈值说明写放大或读放大开始累积。
- `mts_compaction_failure_total`：compaction 失败次数，非零需要结合日志和 `storagecheck` 排查。
- `mts_wal_sync_errors_total`：WAL fsync 错误，出现后应立即停止写入并检查磁盘。
- `mts_sstable_parts`、`mts_sstable_level0_parts`：SSTable 数和 L0 数，持续增长需要手动 compact 或调小 flush 频率。
- `mts_downsample_*`：降采样运行次数、失败次数、处理窗口数、写入点数和完成水位；失败时结合 `docs/storage/downsample-runbook.md` 排查 policy 与 watermark。
- query profile 中的 part/page/sample 读取数：用于定位 slow query 和读放大。

## Health 与 Ready

- `/healthz` 表示进程基础健康。
- `/readyz` 表示是否适合承载读写流量。
- 当 memory hard limit、compaction backlog、maintenance error 或恢复问题出现时，`readyz` 应返回 `503` 并包含结构化 checks。
- 当 `downsample` check 为 `degraded` 时，说明最近一次降采样运行失败；优先确认 source/target retention policy、delay、lookback 和目标写入错误。

## Backup 与 Restore

本地快照：

```bash
timeout 600s go run ./cmd/mts-storage snapshot /var/lib/mts /backup/mts-snapshot-001
```

恢复到新目录：

```bash
timeout 600s go run ./cmd/mts-storage restore /backup/mts-snapshot-001 /var/lib/mts-restore
timeout 600s go run ./cmd/mts-storage check /var/lib/mts-restore
```

恢复验证完成前不要覆盖原目录。发现 fatal issue 时先执行 dry-run：

```bash
timeout 600s go run ./cmd/mts-storage repair --dry-run /var/lib/mts
timeout 600s go run ./cmd/mts-storage repair --apply /var/lib/mts
```

## Compaction

- 优先依赖后台 level compaction。
- 当 `compaction backlog` 持续升高或 L0 parts 超阈值时，可通过启用 admin endpoint 后触发手动 compact。
- 手动 compact 必须设置 `AdminTimeout`，并通过 audit 记录 `task_id`、状态、耗时和错误。
- compact 后检查 `SSTableCountBeforeCompaction`、`SSTableCountAfterCompaction`、level distribution、read/write/space amplification。

## Memory

- 小内存环境建议配置 `StorageMemoryOptions.HardBytesLimit` 和 query/result cache 上限。
- 当写入被内存预算拒绝时，优先降低 batch size、提高 flush 频率或降低并发查询。
- standard gate 可用以下命令复核 512MiB 阈值：

```bash
timeout 900s go run ./tests/scale/storage_10m \
  -profile=standard \
  -points=1000000 \
  -max-rss-bytes=536870912 \
  -verify=true
```

## Slow Query

- Builder 查询应显式设置 time range、projection、limit 和 budget。
- 对 `ORDER BY time DESC LIMIT N` 的查询，检查 explain/profile 中是否出现早停和 page pruning。
- 当 slow query 出现时，记录 query spec、tenant、deadline、read amplification、parts/pages/samples 读取数和 post-filter 数。
- 对需要全量扫描的离线任务，使用流式 API 或分页 cursor，避免一次性 materialization。
