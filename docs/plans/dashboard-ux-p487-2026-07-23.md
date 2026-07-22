# Dashboard UX P487 — ops/config/stats path

## 目标
管理面 stats/config/ops/spec 响应统一补 `path`，Dashboard Operations/Config/ApiSpec 可观测。

## 范围
- Server：`maintenanceStats`/`opsStatus`/`storageMemory`/`compactionStats`/`maintenanceErrors`/`config`/`configSchema`/`apiSpec`/`errorCodes`/`restoreDrill`/`downsampleDryRun`
- Dashboard：Operations/Config/ApiSpec path 徽章
- 清单/命令面板/e2e：`ops-config-stats-path`

## 验收
- [x] 响应 JSON 含 path
- [x] Go `TestHTTPOpsConfigStatsReportPath`
- [x] Dashboard 徽章 data-testid
- [x] npm test / build / commercial-smoke
- [x] P3-01 对象存储冷层仍不在本轮
