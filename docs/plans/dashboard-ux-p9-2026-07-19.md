# Dashboard 体验增强 EARS 清单（2026-07-19 P9）

## 范围
- 运维页（Operations）空状态与动作结果统一
- 降采样页（Downsample）空状态与动作结果统一
- 共享 ActionResultBanner / actionResult 工具

## EARS
- [x] EARS-FE-P9-01 WHEN 运维动作成功或失败 THE SYSTEM SHALL 以可关闭的统一结果条展示（ok/error）
- [x] EARS-FE-P9-02 WHEN 运维统计/维护错误为空 THE SYSTEM SHALL 使用 EmptyState 而非纯文本占位
- [x] EARS-FE-P9-03 WHEN 降采样策略列表为空 THE SYSTEM SHALL 展示 EmptyState 并提供创建入口
- [x] EARS-FE-P9-04 WHEN 降采样 run/reset/dry-run/启停/删除成功或失败 THE SYSTEM SHALL 写入统一 ActionResult 结果条
- [x] EARS-FE-P9-05 WHEN 用户关闭结果条 THE SYSTEM SHALL 清除对应结果状态

## 实现备注
- `utils/actionResult.ts` + 单测
- `components/ActionResultBanner.vue`
- `index.css` 增加 `mts-alert-info`
- OperationsPage / DownsamplePage 接入

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`
