# Dashboard / Query 范围删除确认修复（2026-07-21 P149）

## 问题
- `doRangeDelete` 依赖未绑定的 `deleteConfirmText`，导致 ConfirmDialog 输入 DELETE 后仍无法提交

## 范围
- 删除死代码检查，改由 ConfirmDialog `requireText` 门禁
- 确认框展示删除范围摘要（db/rp/measurement/tags/time）
- 无时间范围警告

## EARS
- [x] EARS-FE-P149-01 WHEN 用户在确认框输入 DELETE THE SYSTEM SHALL 允许提交范围删除
- [x] EARS-FE-P149-02 WHEN 打开范围删除确认 THE SYSTEM SHALL 展示当前查询范围摘要
- [x] EARS-FE-P149-03 WHEN 未设置时间范围 THE SYSTEM SHALL 显示警告
- [x] EARS-FE-P149-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖确认输入启用路径
- [x] EARS-DOC-P149-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P149

## 验证
- [x] `npm test`
- [x] `npm run build`
- [x] `npm run test:e2e`
- [x] `make e2e`
- [x] `go test -count=1 -timeout 120s ./...`
