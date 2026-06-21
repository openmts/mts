# Downsample Commercial Closure Design

## 背景

降采样首版已经支持单机规则物化：创建 `DownsamplePolicy`，按窗口聚合源 retention policy，并写入目标 retention policy。检视报告 `docs/review/code-review-2026-06-20-2258.md` 指出当前链路仍缺少可取消调度、增量 checkpoint、历史回填语义、policy 变更一致性、分批执行、per-policy 观测、故障矩阵和规模压测。

本专项只补齐单机商用主链路，不引入 SQL、分布式调度、外部元数据中心或 compaction rollup。

## 目标

- 让 scheduler 随 Engine 生命周期可取消，关闭不被长降采样任务无限阻塞。
- 让降采样执行按窗口、按列流、按 batch 写入，避免整次运行全量 materialize。
- 让 watermark 能记录连续成功进度，失败后不会重跑大量已经成功的新窗口。
- 让首次回填、显式 backfill、repair、dry-run 和 reset watermark 有稳定 API。
- 让 policy 非兼容变更被拒绝或显式 reset，避免目标 RP 混入不一致 schema。
- 让 per-policy status/metrics/runbook/fault/scale 覆盖能支撑生产排障。

## 非目标

- 不实现 SQL/InfluxQL/PromQL parser。
- 不实现自动查询路由。
- 不实现分布式 leader election。
- 不接入 etcd、ZooKeeper 或外部 metadata store。
- 不在 compaction 中生成 rollup SSTable。
- 不实现 page-level 预聚合 stats。

## 设计方案

### Policy 配置

`DownsamplePolicy` 增加商用控制字段：

- `InitialStartTime int64`：首次没有 watermark 时的历史回填起点，按窗口对齐。未设置时自动调度从 lookback 起点或最后一个完整窗口开始，完整历史回填必须使用显式 range backfill，避免 wall-clock 场景从 Unix 纪元规划海量窗口。
- `RunTimeout time.Duration`：单次自动调度运行的超时。未设置时使用引擎默认值。
- `BatchSize int`：目标点分批写入大小。未设置时使用默认值。
- `CheckpointInterval int`：处理多少个新窗口后 checkpoint 一次 watermark。未设置时每个新窗口都可 checkpoint。
- `PolicyTagName string`：写入目标 tags 的 policy 标记名。未设置时使用 `mts_downsample_policy`。

同名 policy 更新时，以下字段变化属于非兼容变更：source、target、interval、functions、group tags、policy tag name、initial start。系统应拒绝非兼容覆盖，用户必须显式 reset 后再保存。

### Scheduler 生命周期

Engine 启动 scheduler root context。Close 时先 cancel root context，再关闭 stop channel，运行中的 policy run 使用 root context 派生 context，因此 Query/Write 能收到取消信号。自动调度使用 `RunTimeout` 或默认超时保护长任务。

### 执行器

执行器不再调用 `QueryColumns` 收集整个窗口结果，而是通过 `QueryColumnIterator` 逐列读取聚合结果。转换目标点时复用 per-window point buffer，达到 `BatchSize` 后立即写入目标 RP。

现有 query aggregate stream 对单个窗口仍会 materialize 当前窗口内的 group accumulator；本专项通过“小窗口 + 列流消费 + 分批写入 + query memory budget + run timeout”控制内存，不宣称无限窗口零 materialize。

### Watermark 与进度

窗口分为两类：

- refresh window：由 lookback 触发的历史回刷窗口，不推进主 watermark。
- advance window：从当前 watermark 或 initial start 到 eligibleUntil 的连续新窗口，成功后推进 watermark。

每个 advance window 成功后可按 `CheckpointInterval` 更新 watermark。失败时保留最近 checkpoint，记录 `LastError`，下次从 checkpoint 后续跑。

### Backfill/Repair/Dry-run/Reset

新增 API：

- `RunDownsamplePolicyRange(ctx, name, start, end, opts)`：显式处理 `[start,end)`。
- `DryRunDownsamplePolicy(ctx, name, start, end)`：只返回窗口数和预估范围，不写目标数据。
- `RepairDownsamplePolicy(ctx, name, start, end)`：等价于显式 range 重算，但不推进 watermark。
- `ResetDownsamplePolicy(ctx, name, reset)`：设置 watermark，并可选择允许下一次非兼容 policy 替换。

### Observability

新增 per-policy status：

- policy name、active、last run/success/error、watermark、lag、next run、windows/points 累计、last duration、current window。

Metrics 保留全局指标，同时输出 per-policy gauge/counter 文本中可表达的维度信息。项目当前 registry 不支持 label，因此 per-policy 指标使用 policy 名归一化后编码到 metric name，例如 `mts_downsample_policy_cpu_1m_lag_seconds`。

### Fault 与 Scale

Fault 覆盖：

- 目标写失败。
- metadata 保存失败。
- context cancellation。
- policy 非兼容变更拒绝。
- checkpoint 后失败可续跑。

Scale 用例增加：

- flags 控制 points、series、policy 数、batch size、checkpoint interval、run timeout、初始起点。
- 输出 per-stage duration、RSS、windows、points、watermark、policy status。

## EARS 清单

- When Engine 关闭时，系统应取消 scheduler root context，使运行中的 downsample query/write 在 context cancellation 下退出。
- When 自动 scheduler 触发 policy run 时，系统应使用 policy `RunTimeout` 或默认超时创建 run context。
- When policy 首次运行且没有 watermark 且设置了 `InitialStartTime` 时，系统应从该时间对齐后的窗口开始；未设置时应从 lookback 起点或最后一个完整窗口开始，避免无界历史扫描。
- When policy lookback 覆盖已完成窗口时，系统应回刷 refresh window，但不倒退主 watermark。
- When advance window 成功处理时，系统应按 `CheckpointInterval` checkpoint watermark。
- When 某个窗口失败时，系统应保留最近成功 checkpoint，记录 LastError，并允许下次从 checkpoint 后续跑。
- When executor 读取聚合结果时，系统应通过 `QueryColumnIterator` 流式消费列结果，并按 `BatchSize` 分批写入目标点。
- When 目标点写入时，系统应写入 `mts_downsample_policy=<policy>` 或配置的 policy tag。
- When 同名 policy 发生非兼容变更时，系统应拒绝覆盖并返回明确错误。
- When 用户显式 reset policy 时，系统应更新 watermark，并允许下一次非兼容 policy 替换。
- When 用户执行 dry-run 时，系统应返回窗口数和范围，不写目标数据、不推进 watermark。
- When 用户执行 repair/range backfill 时，系统应只处理指定范围，并由选项控制是否推进 watermark。
- When 用户查看 status 或 metrics 时，系统应能看到 per-policy lag、active、last run、last success、last error、watermark、next run、last duration、windows 和 points。
- When query/write/metadata/cancel 等异常发生时，系统应有 fault 测试证明 watermark 不越过失败窗口。
- When scale workload 运行时，系统应输出降采样写入、运行、查询、RSS 和 status 报告。

## 自检

- Placeholder scan：本文没有未决占位。
- Scope check：本文只覆盖单机规则物化降采样闭环，不扩展到 SQL、分布式或 compaction rollup。
- Consistency check：所有新增 API 均走现有 Engine/LocalMetadataStore/Query iterator/Write 路径。
