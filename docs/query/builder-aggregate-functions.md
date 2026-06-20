# Builder Aggregate Function Matrix

MTS 当前只支持 Builder/API 查询入口，不支持 SQL、InfluxQL 或 PromQL parser。聚合函数通过 `mts.NewQuery().Aggregate(function, field)` 或 `querylang.NewBuilder().Aggregate(function, field)` 构造。

## 已支持函数

| 函数 | 类型要求 | 输出 | 窗口支持 | 语义 |
| --- | --- | --- | --- | --- |
| `count` | 任意类型 | `int64` | 支持 | 返回样本数 |
| `sum` | `float64`/`int64` | 同输入或 `float64` | 支持 | 返回数值和 |
| `avg`/`mean` | `float64`/`int64` | `float64` | 支持 | 返回算术平均值 |
| `min` | `float64`/`int64` | 同输入 | 支持 | 返回最小值 |
| `max` | `float64`/`int64` | 同输入 | 支持 | 返回最大值 |
| `first` | 任意类型 | 同输入 | 支持 | 返回窗口内最早 timestamp 的值 |
| `last` | 任意类型 | 同输入 | 支持 | 返回窗口内最晚 timestamp 的值 |
| `difference` | `float64`/`int64` | 同输入或 `float64` | 不聚合窗口 | 返回相邻样本差值序列 |
| `derivative` | `float64`/`int64` | `float64` | 不聚合窗口 | 返回相邻样本按秒归一化的变化率 |
| `rate` | `float64`/`int64` | `float64` | 支持 | counter reset 后按 reset 后当前值计入增量 |
| `irate` | `float64`/`int64` | `float64` | 支持 | 使用最后两个样本计算瞬时 rate |
| `increase` | `float64`/`int64` | `float64` | 支持 | counter reset 友好的累计增量 |
| `delta` | `float64`/`int64` | 同输入或 `float64` | 支持 | 返回首尾差值，不做 counter reset 修正 |
| `spread` | `float64`/`int64` | 同输入或 `float64` | 支持 | 返回 max - min |
| `median` | `float64`/`int64` | `float64` | 支持 | 返回排序后的中位数 |
| `mode` | 任意类型 | 同输入 | 支持 | 返回出现次数最多的值 |
| `stddev` | `float64`/`int64` | `float64` | 支持 | 返回总体标准差 |
| `stdvar` | `float64`/`int64` | `float64` | 支持 | 返回总体方差 |
| `top` | `float64`/`int64` | 同输入 | 支持 | 当前 Builder 无 k 参数，语义为 top1 |
| `bottom` | `float64`/`int64` | 同输入 | 支持 | 当前 Builder 无 k 参数，语义为 bottom1 |

## 稳定拒绝函数

以下函数当前不属于 Builder 主链路支持范围：`fill`、`interpolation`、`align`、`downsample`、`histogram`、`approx_quantile`、`moving_average`、`moving_sum`、`moving_stddev`。

当 Builder 查询包含这些函数时，Analyzer 必须返回稳定 `unsupported-function` 错误，不能进入执行期后再失败，也不能输出不完整结果。

## 边界规则

- 输入类型不匹配时，Analyzer 返回 `function-type-mismatch`。
- 窗口按左闭右开 `[start, start+window)` 归桶。
- 重复 timestamp 在列聚合归一化时保留最新值。
- 乱序输入在聚合前按 timestamp 稳定排序。
- `NaN`/`Inf` 当前按 Go `float64` 原样参与数值计算，不做 PromQL 风格特殊传播语义。
