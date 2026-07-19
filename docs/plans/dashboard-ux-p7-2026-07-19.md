# Dashboard 体验增强 EARS 清单（2026-07-19 P7）

## 范围
- 全局错误边界（ErrorBoundary）
- 统一空状态组件（EmptyState）
- 查询结果列可见性记忆

## EARS
- [x] EARS-FE-P7-01 WHEN 路由页面组件渲染抛错 THE SYSTEM SHALL 捕获错误并展示可重试/刷新的兜底 UI，而非白屏
- [x] EARS-FE-P7-02 WHEN 查询成功但无行/列/原始输出 THE SYSTEM SHALL 展示统一空状态提示
- [x] EARS-FE-P7-03 WHEN 查询历史为空 THE SYSTEM SHALL 使用 EmptyState 展示引导文案
- [x] EARS-FE-P7-04 WHEN 用户切换结果列可见性 THE SYSTEM SHALL 至少保留一列，并将配置写入 localStorage
- [x] EARS-FE-P7-05 WHEN 用户再次打开查询页 THE SYSTEM SHALL 恢复上次的结果列可见性

## 实现备注
- `components/ErrorBoundary.vue` + `App.vue` 包裹 RouterView
- `components/EmptyState.vue`：查询空结果 / 历史空列表
- `utils/resultColumns.ts`：列 key/解析/切换/grid class
- `queryPrefs.resultColumns` 扩展持久化

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`
