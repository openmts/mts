# Dashboard UX P477（2026-07-23）

## 目标
对齐 query/stats 的 path meta，并补齐范围删除结果导出，继续前后端数据面契约一致。

## 范围
- Server：`queryStatsResponse.path`、契约 `query_stats_path`、单测
- Dashboard：引擎 stats path 徽章、删除结果 JSON 导出、清单/命令面板/e2e
- 不做：对象存储冷层、SQL parser、分布式、refresh token

## 验收
- [x] GET query/stats 含 path
- [x] 数据面契约含 query_stats_path
- [x] Stats 区展示 path 徽章
- [x] 删除结果可导出 mts.delete.result v1
- [x] npm test / build / commercial-smoke / Go 定向测试

## 实现备注
- POC 不兼容旧客户端；path 为 omitempty 友好字段
- 深链只读；ForceChange 不禁用 submit
