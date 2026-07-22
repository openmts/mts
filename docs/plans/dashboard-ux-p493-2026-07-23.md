# Dashboard UX P493 — Metrics 健康信号 + 运维深链

## 目标
从 /metrics 提取 healthy/ready/积压/错误计数扫视卡，并深链 Operations/Readiness；修复降采样摘要误挂在 loadError 下的展示 bug。

## 验收
- [x] extractMetricsHealthSignals 单测
- [x] metrics-health-signals / jump-ops / jump-readiness testid
- [x] downsample summary 与 loadError 解耦
- [x] 清单 metrics-health-signals
- [x] npm test / build / commercial-smoke
