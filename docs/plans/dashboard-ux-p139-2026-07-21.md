# Dashboard / Query predicates EARS（2026-07-21 P139）

## 范围
- 查询表单 `predicates` DSL → API `Query.predicates`
- 支持 tag/field 比较；校验错误可读 i18n
- 历史表单可持久化 predicates 字段

## 边界
- 不实现完整 expr 树（AND/OR/NOT UI）
- 不做 SQL/PromQL parser

## EARS
- [x] EARS-FE-P139-01 WHEN 用户填写 predicates 文本 THE SYSTEM SHALL 映射到 Query.predicates
- [x] EARS-FE-P139-02 WHEN 谓词格式非法 THE SYSTEM SHALL 返回可读错误
- [x] EARS-FE-P139-03 WHEN 打开查询页 THE SYSTEM SHALL 暴露 query-predicates 输入
- [x] EARS-DOC-P139-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P139 实现

## DSL 示例
- `tag:host=a`
- `tag_ne:env=prod`
- `tag_exists:region`
- `tag_in:zone=z1|z2`
- `usage>0.5` / `field_gt:usage=0.5`

## 验证
- [x] `npm test`（含 parsePredicates / buildQueryFromForm）
- [x] `npm run build`（i18n 无重复 key）
- [x] `npm run test:e2e`（commercial-smoke 覆盖 `query-predicates`）
- [x] `make e2e`
- [x] `go test -count=1 -timeout 120s ./...`

## 实现备注
- DSL 支持 tag/tag_ne/tag_exists/tag_in 与 field 比较；kind 与 `query_types.go` iota+1 对齐
- 非法谓词走 formT i18n（queryErrPred*）
- 历史 `push({ form: { ...queryForm } })` 自动持久化 predicates
- **不做** expr 树 UI
