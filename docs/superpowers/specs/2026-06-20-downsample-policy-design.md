# Downsample Policy Design

## 背景

MTS 当前已经支持 Builder/API 查询、窗口聚合、tag 分组、retention policy 和本地元数据管理。`GroupByTime(window)` 能在查询时计算窗口聚合，但每次查询仍需要扫描原始样本，无法满足长时间范围 dashboard、历史报表和低成本长期保留的需求。

本设计选择“规则驱动物化降采样”方案：通过本地 `DownsamplePolicy` 周期性扫描原始数据，将固定窗口聚合结果写入目标 retention policy。第一阶段不做自动查询路由，用户显式查询目标 retention policy。

## 开源实现参考

- InfluxDB 的 Continuous Query / Task 常将原始 measurement 聚合写入低精度 retention policy，用 retention policy 管理不同精度数据生命周期。
- Prometheus 的 recording rules 周期性执行表达式，把结果作为新的 time series 写回 TSDB，规则失败时保留错误状态并等待下一次评估。
- TimescaleDB 的 continuous aggregates 使用物化结果、后台刷新和 invalidation 管理迟到数据一致性。
- VictoriaMetrics 和 Prometheus 生态系统常结合 rollup 查询、recording rules、result cache 或历史 block downsampling，在查询性能和写入复杂度之间取舍。

MTS 采用更保守的单机实现：规则调度和物化写入在 Engine 层完成，目标数据仍走普通 WAL、MemTable、SSTable 主链路，避免引入新的 SSTable 格式和 compaction 耦合。

## 范围

### 包含

- 新增 `DownsamplePolicy` 配置模型和稳定校验。
- LocalMetadataStore 持久化 policy 和 watermark。
- Engine 暴露 policy 管理 API。
- 后台 scheduler 按完整窗口执行降采样。
- 支持 `avg`、`min`、`max`、`sum`、`count`、`first`、`last`。
- 支持按 tag 分组保留源 series 的指定 tag。
- 支持 `delay`、`refresh_interval`、`lookback`。
- 支持失败不推进 watermark、重复执行幂等写入。
- 支持显式查询目标 retention policy。
- 提供 metrics、runbook、单元测试、e2e 和 fault 测试。

### 不包含

- 不实现 SQL、InfluxQL、PromQL parser。
- 不实现自动查询路由和 rollup 选择器。
- 不在 compaction 阶段生成 rollup SSTable。
- 不实现 page-level 预聚合 stats。
- 不做分布式调度、leader election 或外部元数据中心。

## 数据模型

```go
type DownsamplePolicy struct {
    Name              string
    SourceDatabase    string
    SourceRetention   string
    SourceMeasurement string
    TargetDatabase    string
    TargetRetention   string
    TargetMeasurement string
    Interval          time.Duration
    Functions         []DownsampleFunction
    GroupByTags       []string
    Delay             time.Duration
    RefreshInterval   time.Duration
    Lookback          time.Duration
    Enabled           bool
}

type DownsampleFunction struct {
    Function string
    Field    string
    As       string
}

type DownsampleWatermark struct {
    PolicyName         string
    CompletedUntilUnix int64
    LastRunUnix        int64
    LastSuccessUnix    int64
    LastError          string
}
```

字段命名规则：

- `As` 非空时使用 `As`。
- `As` 为空时使用 `<function>_<field>`，例如 `avg_usage`。
- `count` 输出 `int64`，其他聚合按现有 queryexec 聚合语义输出。

窗口规则：

- 窗口为左闭右开 `[windowStart, windowStart+interval)`。
- 输出 timestamp 固定为 `windowStart`。
- 只处理 `now - delay` 之前的完整窗口。
- 首次执行没有 watermark 时，从源数据查询范围的最早完整窗口开始；若无法推导起点，则从 `now - delay - lookback` 对齐到窗口开始。

## 架构

### Metadata

`MetadataStore` 增加 downsample policy 和 watermark 子接口。`LocalMetadataStore` 将其作为本地实现，持久化到 catalog 目录下的二进制 metadata 文件，WAL 或临时文件原子替换遵循现有 catalog 模式。

接口职责：

- 创建、更新、删除、启用、禁用 policy。
- 列出 policy。
- 读取和更新 watermark。
- 校验同名 policy 唯一。

### Engine API

Engine 提供：

- `CreateDownsamplePolicy(ctx, policy)`
- `ListDownsamplePolicies(ctx)`
- `DropDownsamplePolicy(ctx, name)`
- `EnableDownsamplePolicy(ctx, name)`
- `DisableDownsamplePolicy(ctx, name)`
- `RunDownsamplePolicy(ctx, name, now)`，用于测试、手动触发和 scheduler 复用。

Public package `mts` 暴露等价类型和方法。

### Scheduler

`DownsampleScheduler` 随 Engine 启动，随 Engine 关闭停止。它每隔一个短 tick 读取 enabled policy，根据 `RefreshInterval` 和 watermark 决定是否运行。每个 policy 同一时刻最多一个执行任务，避免重复刷新同一窗口。

调度策略：

- disabled policy 不运行。
- `LastRunUnix + RefreshInterval > now` 时跳过。
- 当前可处理边界为 `floor((now-delay)/interval)*interval`。
- 只处理 watermark 到可处理边界之间的窗口。
- 失败时记录 `LastError` 和 `LastRunUnix`，不推进 `CompletedUntilUnix`。
- 成功时推进 watermark 到最后成功窗口末尾。

### Executor

Executor 对每个窗口构造 QuerySpec：

- From source。
- TimeRange 为当前窗口。
- Aggregate 为 policy functions。
- GroupByTags 为 policy tags。
- GroupByTime 为 policy interval。

执行结果转换为普通 `model.Point` 写入 target：

- database/retention/measurement 使用 target。
- tags 为 group 结果 tags，并额外写入 `mts_downsample_policy=<name>`。
- timestamp 为窗口开始。
- fields 为聚合输出字段。

幂等性依赖现有 LSM write sequence 和查询最新值语义：相同 series、field、timestamp 的重跑会写入更高 write sequence，读取时保留最新值。为了减少无效重写，scheduler 只对 lookback 范围内已完成窗口进行可配置 refresh。

### Late Data 与 Refresh

迟到数据通过 `lookback` 处理。每次运行除了 watermark 后的新窗口，也重新计算 `[now-delay-lookback, now-delay)` 内的完整窗口。refresh 写入相同 timestamp 和字段名，最终查询取最新 write sequence。

第一阶段不删除旧降采样值；如果函数集合或字段集合变更，要求用户创建新 policy 名称或清空目标 retention policy。

### Observability

新增指标：

- `mts_downsample_runs_total`
- `mts_downsample_failures_total`
- `mts_downsample_windows_total`
- `mts_downsample_points_written_total`
- `mts_downsample_last_duration_seconds`
- `mts_downsample_watermark_unix`

`HealthSnapshot` 包含 downsample scheduler 是否运行、policy 数、最近错误数。

## EARS

- When 用户创建 downsample policy 时，系统应校验 policy 名称、source、target、interval、functions、delay、refresh interval、lookback 和 group tags。
- When policy interval 小于等于 0 时，系统应拒绝创建并返回明确错误。
- When policy functions 为空或函数不在支持列表中时，系统应拒绝创建并返回明确错误。
- When source 和 target 完全相同时，系统应拒绝创建，避免降采样写回原始序列。
- When scheduler 运行时，系统应只处理 `now-delay` 之前的完整窗口。
- When 某个窗口执行失败时，系统应记录错误且不推进 watermark。
- When 同一窗口被重复执行时，系统应写入相同 timestamp、tags 和 fields，使结果可由 write sequence 幂等覆盖。
- When late data 落入 lookback 范围时，系统应重新计算对应完整窗口。
- When policy disabled 时，scheduler 应停止自动运行该 policy，但保留 policy 和 watermark。
- When Engine 重启时，系统应从本地 metadata 恢复 policy 和 watermark。
- When 查询目标 retention policy 时，用户应能读取已物化的降采样结果。
- When Engine 关闭时，scheduler 应停止接收新任务并等待运行中的任务退出或响应 context cancellation。

## 测试策略

- Unit：policy validation、field naming、window planning、watermark advance、refresh range、disabled skip。
- Metadata：policy 和 watermark 创建、更新、删除、重启恢复、损坏文件错误。
- Engine：手动运行 policy、后台 scheduler、失败不推进 watermark、重跑幂等。
- E2E：写入 raw 数据，创建 1m 降采样 policy，运行后查询 target retention policy 校验 `avg/min/max/count/last`。
- Fault：写 target 失败、query 失败、metadata save 失败时 watermark 不推进。
- Scale smoke：100K raw 点降采样到 1m 窗口，输出耗时、写入点数和 RSS。

## 自检

- Placeholder scan：本文不使用未决占位标记。
- Scope check：本专项只做单机规则驱动物化降采样，不做自动查询路由和 compaction rollup。
- Consistency check：所有写入目标都走现有 Engine Write 路径，降采样数据不是新的事实源格式。
