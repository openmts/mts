# 降采样运维手册

## 检查策略

1. 使用 `ListDownsamplePolicies` 确认策略存在且 `Enabled=true`。
2. 确认 `SourceDatabase/SourceRetention/SourceMeasurement` 与原始写入路径一致。
3. 确认 `TargetDatabase/TargetRetention/TargetMeasurement` 与查询目标一致，且不能与 source 完全相同。
4. 确认 `Interval`、`Delay`、`RefreshInterval`、`Lookback` 均为预期值。

## 检查水位

`CompletedUntilUnix` 表示已完成的窗口右边界，窗口语义为左闭右开 `[start,end)`。

- `CompletedUntilUnix` 不推进：检查 source 时间范围内是否有完整窗口，以及最近一次 `LastError`。
- `LastRunUnix` 持续更新但 `LastSuccessUnix` 不更新：说明执行失败，优先查看 `LastError`。
- late data 未反映到目标 RP：确认 `Lookback` 是否覆盖 late data 所在窗口。
- 使用 `DownsamplePolicyStatuses` 查看每个 policy 的 `LagSeconds`、`NextRunUnix`、`LastDuration`、`WindowsProcessed` 和 `PointsWritten`。

## Delay 与 Lookback

- `Delay` 用于避免处理未完整到达的半窗口，系统只处理 `now-delay` 之前的完整窗口。
- `Lookback` 用于回刷已完成窗口，适合处理延迟到达数据。
- `Lookback` 过小会漏掉 late data，过大会增加查询与写入开销。

## 指标

关键指标：

- `mts_downsample_runs_total`
- `mts_downsample_success_total`
- `mts_downsample_failures_total`
- `mts_downsample_windows_processed_total`
- `mts_downsample_points_written_total`
- `mts_downsample_last_watermark_unix`
- `mts_downsample_last_duration_seconds`
- `mts_downsample_policy_lag_seconds{policy="<policy>"}`
- `mts_downsample_policy_last_watermark_unix{policy="<policy>"}`
- `mts_downsample_policy_windows_processed_total{policy="<policy>"}`
- `mts_downsample_policy_points_written_total{policy="<policy>"}`
- `mts_downsample_status_collection_errors_total`

若 `HealthSnapshot` 中 `downsample` 为 `degraded`，优先查看最后一次错误和对应 policy 配置。

## 手动回填与修复

- 使用 `DryRunDownsamplePolicy` 预估指定时间范围内的窗口数量、group 数、源样本数和目标点数，不写目标数据，不推进水位。
- 使用 `RunDownsamplePolicyRange` 执行连续范围回填。只有确认范围从当前 watermark 连续开始时，才设置 `AdvanceWatermark=true`。
- 使用 `RepairDownsamplePolicy` 重算某段历史窗口，默认不推进 watermark，适合修复 late data 或目标数据异常。
- 使用 `ResetDownsamplePolicy` 重设 watermark；当 policy 函数、group tag、target 或 interval 发生非兼容变化时，需要显式设置 `AllowPolicyReplace=true` 后再替换 policy。若需要同步清理旧 rollup 数据，设置 `CleanupTarget=true` 并提供 `[CleanupStartUnix,CleanupEndUnix)`。
- 使用 `DropDownsamplePolicyWithOptions` 且设置 `CleanupTarget=true` 时，系统会按 policy tag 对目标数据写 tombstone，再删除 policy metadata。

## 查询降采样结果

当前项目不支持 SQL parser 和自动 rollup 路由。查询降采样结果有两种方式：

- 显式查询目标 `TargetDatabase/TargetRetention/TargetMeasurement`，并带上 `PolicyTagName=<policy>` tag。
- 使用 public Builder 的 `FromDownsamplePolicy(policy)`，由 Builder 填充目标 RP 和 policy tag。

## 存储层 rollup 边界

当前降采样是规则驱动的后台物化，不在 SSTable page 或 compaction 阶段生成内建 rollup。若后续需要 page-level rollup，需要单独设计持久化格式和兼容性策略。
