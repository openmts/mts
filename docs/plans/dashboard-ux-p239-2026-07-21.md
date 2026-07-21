# Dashboard UX P239（2026-07-21）

## 目标
去掉查询结果 fields 的 `as any`，依赖 `QueryResultRow.fields` 类型。

## EARS
- [x] EARS-FE-P239-01 `formatFieldsMap(row.fields)` 无 any 断言

## 非目标
- 宣称可商用完成
