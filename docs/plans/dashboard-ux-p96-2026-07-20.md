# Dashboard / 通知历史时间范围 EARS（2026-07-20 P96）

## 范围
- 通知历史：快捷时间范围（1h/24h/7d/30d/全部）
- 自定义 since/until（datetime-local）
- 与类型过滤、文本搜索组合；一键清除筛选
- 商业冒烟覆盖相关 testid

## 边界
- 时间过滤基于本会话通知历史的 `at` 毫秒戳
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P96-01 WHEN 用户选择快捷时间范围 THE SYSTEM SHALL 仅展示区间内通知
- [x] EARS-FE-P96-02 WHEN 用户设置自定义起止时间 THE SYSTEM SHALL 按闭区间过滤
- [x] EARS-FE-P96-03 WHEN 用户清除筛选 THE SYSTEM SHALL 重置类型/搜索/时间条件
- [x] EARS-FE-P96-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 time-range/since/until/clear
- [x] EARS-DOC-P96-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P96

## 实现备注
- `filterNotifyHistoryByTime` / `notifyHistoryRangeBounds` 纯函数 + 单测
- testid：`notify-history-time-range`、`notify-history-since`、`notify-history-until`、`notify-history-time-clear`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
