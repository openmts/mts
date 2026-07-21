# Dashboard UX P243（2026-07-21）

## 目标
Overview 分项失败标签化；Query series 加载失败可重试。

## EARS
- [x] EARS-FE-P243-01 admin 部分失败文案带分区标签（内存/compaction/维护/doctor/版本）
- [x] EARS-FE-P243-02 `query-series-error` 提供 `query-series-retry` 调用 `refreshSeriesWithTags`
- [x] EARS-FE-P243-03 series 错误 live region alert

## 非目标
- AccessMatrix（静态矩阵无服务端 load）
- 宣称可商用完成
