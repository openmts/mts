# 生产查询保护与告警模板

## 默认策略

`DefaultOptions(path)` 启用：

- `QueryProtection.DefaultMaxSamples = 1_000_000`
- 请求未设置 `Budget.MaxSamples` / `Limit` 时注入默认保护
- 显式设置的 budget/limit **不会被覆盖**

推荐生产再叠加：

```go
opts := mts.DefaultOptions("/var/lib/mts")
opts.QueryProtection.DefaultMaxSamples = 500_000
opts.QueryProtection.DefaultLimit = 100_000 // 可选
```

查询侧：

```go
query.Budget = mts.QueryBudget{MaxSamples: 100_000, MaxParts: 64, MaxShards: 32}
query.Limit = 10_000
```

## 错误语义

- `ErrReadBudgetExceeded`：触达 MaxSamples/MaxParts/MaxShards
- `ErrResourceExhausted`：资源/限制类耗尽（若映射）

可用 `errors.Is` 判定。

## 告警模板（PromQL 风格示例）

```text
# 查询预算错误激增
increase(mts_query_budget_errors_total[5m]) > 10

# 查询取消/超时
increase(mts_query_cancellations_total[5m]) > 20

# 后台 compaction 被并发/内存跳过过多
increase(mts_maintenance_compaction_skipped_total[15m]) > 50

# tombstone 堆积
mts_tombstones_pending > 1000
```

具体指标名以 `MetricsSnapshot` 实际输出为准；若某 counter 未导出，优先使用 query stats / maintenance stats 等价字段。
