# Dashboard 体验增强 EARS 清单（2026-07-19 P8）

## 范围
- 查询/写入表单脏状态提示与 beforeunload 防护
- 查询耗时水线（fast/ok/slow/critical）
- Write / Audit 页 EmptyState 复用

## EARS
- [x] EARS-FE-P8-01 WHEN 用户修改查询条件且未以当前条件成功查询 THE SYSTEM SHALL 展示「未查询变更」标识
- [x] EARS-FE-P8-02 WHEN 查询表单存在未查询变更且用户关闭标签页 THE SYSTEM SHALL 触发 beforeunload 确认
- [x] EARS-FE-P8-03 WHEN 查询返回 stats.duration_nanos THE SYSTEM SHALL 展示耗时水线（≤50 快 / ≤200 正常 / ≤1000 偏慢）
- [x] EARS-FE-P8-04 WHEN 写入表单相对上次成功提交有变更 THE SYSTEM SHALL 展示「未提交变更」并可 beforeunload 提示
- [x] EARS-FE-P8-05 WHEN 审计列表为空 THE SYSTEM SHALL 使用 EmptyState 展示引导与刷新动作
- [x] EARS-FE-P8-06 WHEN 写入页尚无结果 THE SYSTEM SHALL 使用 EmptyState 提示推荐 TypedBatch

## 实现备注
- `utils/formDirty.ts`：stableStringify / isDirty / snapshotForm
- `utils/queryLatency.ts`：classifyLatency / latencyFromNanos
- QueryPage：formDirty + latency bar
- WritePage：formDirty + EmptyState
- AuditPage：EmptyState

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`
