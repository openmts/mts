# Time Precision API 设计

## 背景

当前 mts 公开 API 与内部存储均默认使用 Unix nanosecond：

- `Point.Timestamp` 表示写入时间戳。
- `TypedBatch.Timestamps` 表示批量写入时间戳。
- `Query.StartTime`、`Query.EndTime` 和 `QueryPredicateTimeRange.Start/End` 表示查询范围。
- `Row.Timestamp` 与 `ColumnSeries.Timestamps` 直接返回内部纳秒时间戳。

商用调用方可能使用秒、毫秒或微秒时间戳。为保持 API 简洁且不改变内部存储格式，本设计在 public package 增加时间精度声明，由公开转换层把写入和查询时间统一归一化为纳秒，并按查询精度转换返回结果。

## 范围

本次包含：

- 新增 public `TimePrecision` 类型和 `ns/us/ms/s` 常量。
- `Point` 增加 `Precision` 字段。
- `TypedBatch` 增加 `Precision` 字段，保证高吞吐写入路径和 `Point` 一致。
- `Query` 增加 `Precision` 字段。
- `QueryBuilder` 增加 `Precision(precision TimePrecision)` 方法。
- 写入时将 public 时间戳转换为内部纳秒。
- 查询时将 public 查询范围转换为内部纳秒，并将行式、列式、迭代器和 explain 结果中的数据时间戳转换为查询精度。

本次不包含：

- 改变 `internal/model`、WAL、SSTable、downsample 或 query stats 的时间单位。
- 增加 SQL、InfluxQL、PromQL 解析。
- 支持小数时间戳或非整数精度。
- 对 `time.Duration`、`QueryStats.DurationNanos`、`QueryStats.StartedUnixNanos` 做精度转换。

## 设计决策

内部存储继续使用纳秒。原因是现有 shard、WAL、SSTable、query pruning、downsample 和聚合窗口均已围绕纳秒实现，改内部格式会扩大风险且没有必要。

`Precision` 的零值表示纳秒，保持已有调用方兼容。合法值为：

- `PrecisionNanosecond` / `"ns"`
- `PrecisionMicrosecond` / `"us"`
- `PrecisionMillisecond` / `"ms"`
- `PrecisionSecond` / `"s"`

`Query.Precision` 同时用于解释 `StartTime`、`EndTime`、time range predicate 的输入单位，并用于返回 `Row.Timestamp`、`ColumnSeries.Timestamps`、`QueryResult.Columns` 和 iterator 当前项的输出单位。`TimeRangeTime(start, end)` 使用 `time.Time.UnixNano()` 写入精确纳秒，同时把查询精度设置为纳秒，避免被后续默认精度误解释。

当输入精度非法或转换成纳秒时发生 `int64` 溢出，公开 API 应返回 `ErrInvalidPrecision`。写入失败时不得部分写入；查询失败时不得进入内部扫描。

## EARS 清单

- When 调用方不设置 `Point.Precision` 时，系统应按 Unix nanosecond 解释 `Point.Timestamp`。
- When 调用方设置 `Point.Precision` 为秒、毫秒、微秒或纳秒时，系统应在写入前把 `Point.Timestamp` 转换为纳秒存储。
- When 调用方不设置 `TypedBatch.Precision` 时，系统应按 Unix nanosecond 解释 `TypedBatch.Timestamps`。
- When 调用方设置 `TypedBatch.Precision` 为秒、毫秒、微秒或纳秒时，系统应在写入前把所有 batch timestamps 转换为纳秒存储。
- When 写入 precision 非法时，系统应返回 `ErrInvalidPrecision`，且不写入任何 point 或 typed batch。
- When 写入时间戳按 precision 转换为纳秒会溢出 int64 时，系统应返回 `ErrInvalidPrecision`，且不写入任何数据。
- When 调用方不设置 `Query.Precision` 时，系统应按 Unix nanosecond 解释 `StartTime`、`EndTime` 和 time range predicate，并按纳秒返回结果时间戳。
- When 调用方设置 `Query.Precision` 为秒、毫秒、微秒或纳秒时，系统应把查询范围转换为纳秒执行，并把返回数据时间戳转换为该 precision。
- When 查询包含 `QueryPredicateTimeRange` 或 `Query.Expr` 中的 time range predicate 时，系统应按 `Query.Precision` 转换 predicate 的 `Start/End`。
- When 查询 precision 非法或查询范围转换溢出时，系统应返回 `ErrInvalidPrecision`，且不进入内部查询。
- When 调用方使用 `TimeRangeTime(start, end)` 时，系统应使用 `time.Time` 的纳秒值查询，并把 `Query.Precision` 设置为纳秒。
- When 调用方使用 `QueryBuilder.Precision(precision)` 时，系统应在 `Build()` 结果中保留该 precision。
- When 使用 `QueryRows`、`QueryColumns`、`QueryRowIterator`、`QueryColumnIterator` 或 `QueryWithExplain` 时，系统应按 `Query.Precision` 转换返回数据时间戳。
- When 查询返回运行统计时，系统应保持 `DurationNanos` 和 `StartedUnixNanos` 的纳秒语义不变。

## 验收标准

- 单元测试覆盖 `Point`、`TypedBatch`、`QueryRows`、`QueryColumns`、两个 iterator、`QueryWithExplain`、Builder、默认纳秒兼容、非法 precision 和溢出。
- public e2e 覆盖秒或毫秒写入、查询范围转换和返回时间戳转换。
- README、doc.go 或 llms.txt 中公开 API 示例说明 precision 用法。
- `make fmt`、定向测试、`make ci`、`make e2e-public-api`、`git diff --check` 通过。
- 临时测试产物检查无残留。
