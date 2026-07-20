# Dashboard / Query series 筛选（2026-07-21 P146）

## 范围
- series 选择器客户端筛选（标签/自由文本）
- 计数反映筛选结果；无匹配空 option 文案
- 不改后端分页（仍上限 200）

## EARS
- [x] EARS-FE-P146-01 WHEN 输入 series 筛选 THE SYSTEM SHALL 过滤下拉选项
- [x] EARS-FE-P146-02 WHEN 无匹配 THE SYSTEM SHALL 提示空
- [x] EARS-FE-P146-03 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 series filter testid
- [x] EARS-DOC-P146-04 WHEN 更新基线 THE SYSTEM SHALL 记录 P146

## 验证
- [x] `npm test`（filterSeriesList）
- [x] `npm run build`
- [x] `npm run test:e2e`
- [x] `make e2e`
- [x] `go test -count=1 -timeout 120s ./...`（偶发 compaction_integrity flaky，重跑通过）
