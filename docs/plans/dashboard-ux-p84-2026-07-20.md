# Dashboard / Ops·Account·404 可商用 EARS（2026-07-20 P84）

## 范围
- Operations：维护/压缩统计 JSON 导出与复制；统计区 testid
- Account：账户快照（无密钥）导出与复制
- NotFound：页面与导航按钮 testid
- 商业冒烟覆盖相关 testid

## 边界
- 不导出密码/Token
- 不改运维执行契约
- 不宣称部署侧验收完成

## EARS
- [x] EARS-FE-P84-01 WHEN 运维页已加载 THE SYSTEM SHALL 支持导出与复制维护/压缩统计快照
- [x] EARS-FE-P84-02 WHEN 账户页已加载 THE SYSTEM SHALL 支持导出与复制账户会话快照（不含密钥）
- [x] EARS-FE-P84-03 WHEN 访问不存在路由 THE SYSTEM SHALL 暴露 404 页与返回概览 testid
- [x] EARS-FE-P84-04 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 ops/account/404 导出与导航 testid
- [x] EARS-DOC-P84-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P84

## 实现备注
- 纯函数：`buildOpsStatsExport` / `buildAccountExport`
- testid：`ops-export-stats`、`ops-copy-stats`、`ops-maint-stats`、`ops-compact-stats`、`account-export-json`、`account-copy-snapshot`、`not-found-page`、`not-found-go-overview`

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e`
- 仓库根：`make e2e` / `make lint` / `go test ./...`
