# Downsample Commercial Gap Closure Design

## 背景

`docs/review/code-review-2026-06-21-1502.md` 指出降采样已经具备单机 beta/RC 主链路，但距离商用仍有内存边界、dry-run 估算、metrics、fault、rollup 数据治理、函数集合、规模压测和用户查询体验等差距。

本专项只覆盖单机 MTS、本地 metadata、Builder/API 使用方式。不实现分布式调度、外部配置中心、SQL parser，也不在本轮改 SSTable 物理格式。

## 目标

- 降低降采样窗口输出阶段的内存峰值，使 `BatchSize` 同时约束目标点构造和写入。
- 提供可解释的 dry-run 成本估算，避免高基数任务被低估。
- 将降采样观测升级为固定 metric name + label，并暴露采集错误。
- 补齐可商用故障矩阵、目标 rollup 清理 API、常见聚合函数、规模压测报告入口。
- 对不属于当前项目边界的自动路由和 page-level rollup，提供显式 Builder/文档化闭环。

## EARS 清单

### EARS-1: 窗口输出内存控制

- When 降采样窗口产生目标点时，系统应按 `BatchSize` 分批 flush 已完整构造的目标点，避免窗口结束前保存全部输出点。
- When 同一 timestamp/tags 的多个聚合字段跨列到达时，系统应在字段完整后再写出，保证目标点字段不丢失。
- When 输出点 buffer 写入成功后，系统应复用或释放内部 map/slice，避免长期持有高基数窗口内存。

### EARS-2: Group aggregate 内存边界

- When group aggregate 处理窗口化查询时，系统应优先使用增量 accumulator，仅对不能增量计算的函数保留原始点。
- When 降采样使用 `rate/irate/increase/delta/difference/derivative` 等序列函数时，系统应按有序点计算，并在函数不支持时返回明确错误。
- When 当前存储顺序无法保证全局有序时，系统应保持现有排序语义，避免为了流式化牺牲正确性。

### EARS-3: Dry-run 成本估算

- When 用户 dry-run 一个 policy 范围时，系统应返回窗口数、刷新窗口数、推进窗口数、估算 group 数、估算源样本数、估算目标点数。
- When group cardinality 可通过源列扫描估算时，系统应使用 source query 读取 group tags 并去重。
- When 没有 group tag 时，系统应将非空源数据估算为 1 个 group。

### EARS-4: 标准 metrics label

- When metrics 导出 per-policy 降采样指标时，系统应使用固定指标名和 `policy` label。
- When registry 同时存在同名不同 label 指标时，系统应分别保存和按 Prometheus 文本格式输出。
- When status 采集失败时，系统应导出 `mts_downsample_status_collection_errors_total`。

### EARS-5: Fault 矩阵

- When 目标写入失败时，watermark 不应越过失败窗口，health 应降级。
- When metadata 保存失败时，watermark 不应被错误推进。
- When context cancellation 或 engine close 发生时，运行应退出并保留最近 checkpoint。
- When policy reset 后替换非兼容 policy，系统应只允许一次替换并清理替换许可。

### EARS-6: 目标 rollup 数据治理

- When 用户 drop policy 且要求 cleanup 时，系统应按 policy tag 对目标 rollup 数据写入 tombstone。
- When 用户 reset policy 且要求 cleanup 时，系统应先清理目标 rollup 数据，再更新 watermark。
- When cleanup 范围非法或 policy 不存在时，系统应返回明确错误。

### EARS-7: 降采样函数集合

- When policy 使用 `mean` 时，系统应归一化为 `avg`。
- When policy 使用 `rate/irate/increase/delta/difference/derivative/spread/mode/stddev/stdvar/top/bottom/median` 时，系统应通过校验并复用查询聚合语义。
- When 函数需要数值类型而源字段非数值时，系统应返回聚合错误，不应写入错误目标点。

### EARS-8: 规模压测报告

- When scale workload 完成时，报告应输出写入、降采样、查询、RSS、GC、磁盘占用、SSTable 数和参数。
- When 用户配置 points/series/policy/batch/checkpoint/run-timeout/initial-start 时，workload 应稳定解析并写出 JSON。
- When 小规模 smoke 测试运行时，应验证报告字段被填充。

### EARS-9: 显式 rollup 查询入口

- When 用户使用 Builder 构造查询时，系统应允许显式选择 downsample policy，将查询 source 切到 policy target。
- When policy 不存在时，Builder/Engine 应返回明确错误。
- When 项目不支持 SQL parser 和自动路由时，文档应明确推荐显式查询目标 RP 或 Builder 选择 policy。

### EARS-10: Page-level rollup 边界闭环

- When 用户需要长期历史聚合时，当前版本应通过 policy 物化结果服务查询。
- When 讨论 page-level rollup 或 compaction rollup 时，文档应标记为非当前单机降采样专项范围，避免误判为已实现。
- When 后续要进入存储格式级 rollup，应单独开持久化格式专项。

## 验收标准

- 新增和修改的核心行为都有单元或 e2e/fault/scale smoke 覆盖。
- `go test ./... -count=1 -timeout 10m` 通过。
- `golangci-lint run ./...` 通过。
- `goimports-reviser -rm-unused -format ./...` 已执行。
- 文档说明能力边界和运维方式。
