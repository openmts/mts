# MTS Dashboard / mts-server 生产 Runbook

> 单机部署基线。TLS 终止优先放在反向代理；本 runbook 覆盖上线前检查、日常运维与应急。

## 1. 部署拓扑建议

```
[客户端]
   │ HTTPS
   ▼
[反向代理 / LB]  ── TLS 终止、HSTS、限流、访问日志
   │ HTTP (内网)
   ▼
[mts-server]     ── 嵌入 Dashboard + REST/gRPC + 本地存储
   │
   ▼
[本地数据目录]   ── 0700 目录 / 0600 文件
```

- Dashboard 静态资源由 `mts-server` 嵌入（`dashboard-dist`）。
- 子路径部署设置 `http.dashboard_base`（例如 `/mts/`），前端构建使用匹配的 `VITE_BASE`。
- API 前缀默认 `/api/v1/...`；前端使用站点根或 `VITE_API_BASE`，不要把 `VITE_BASE` 拼进 API。
- 普通 API 请求默认超时 **30s**（`VITE_API_TIMEOUT_MS` 可覆盖，单位毫秒，上限 600000）；`/readyz` 探测 5s；NDJSON 流式查询不套默认超时，依赖用户取消。

## 2. 上线前检查清单

建议先跑：`mts-server doctor --config /path/to/mts-server.yaml`（会检查 data/backup 目录、TLS 与生产鉴权提示）。


| 项 | 必须 | 做法 |
|---|---|---|
| 边缘 HTTPS / TLS | 是 | 反向代理证书；可选 mts-server 自带 TLS |
| HSTS | 推荐 | 仅在确认全站 HTTPS 后启用 |
| 安全响应头 | 是 | mts-server `wrapHTTP` 默认写入；冒烟 `TestCommercialDashboardSmoke` |
| 修改默认 admin 密码 | 是 | 禁止长期 `admin/admin` |
| 健康与指标 | 是 | `/healthz` `/readyz` `/metrics` 接入监控 |
| 备份/快照演练 | 推荐 | `admin/storage/snapshot` + 恢复演练 |
| 登录-写-查-运维冒烟 | 是 | 服务侧 smoke + 浏览器人工/Playwright |
| 权限矩阵复核 | 推荐 | Dashboard「权限矩阵」页 + 非 admin 账号抽检 |

自动化入口：

```bash
go test ./cmd/mts-server -run TestCommercialDashboardSmoke -count=1
cd cmd/mts-dashboard && npm run test && npm run build
# 首次需要：npm run test:e2e:install
cd cmd/mts-dashboard && npm run test:e2e
# 或：make dashboard-test-e2e-install && make dashboard-test-e2e
make test && make e2e && make lint
```



## 2.0 可商用就绪中心

打开 Dashboard `/ops/readiness`：聚合生产清单、边缘 HTTPS 验收、备份编排指引与 doctor 状态；勾选状态保存在浏览器 localStorage。就绪评分融合清单完成度与 doctor warn/TLS。Overview 提供入口。支持就绪状态 JSON 导出/导入与演练归档（JSON/Markdown）下载；版本信息见 `/about` 与 `GET /api/v1/admin/version`。


Dashboard 就绪中心提供「部署 Runbook 联调清单」导出（Markdown），覆盖边缘 HTTPS/HSTS、cron/systemd、异地备份与告警的人工步骤与证据占位；**不计就绪评分**，本地勾选/下载不代表生产验收完成。

## 2.1 边缘 HTTPS / HSTS 验收（人工）

1. 反向代理/LB 配置有效证书并对浏览器暴露 HTTPS。
2. 确认明文 HTTP 跳转到 HTTPS（301/308）。
3. 在确认全站 HTTPS 后启用 HSTS；若 `mts-server` 开启 HTTP TLS，服务会自动发送 `Strict-Transport-Security`。
4. 调用 `GET /api/v1/admin/doctor` 或 `mts-server doctor`，处理 warn 项。
5. 经 HTTPS 完成登录 → 查询 → 运维冒烟。

Dashboard 存储页提供同口径勾选清单（`edgeHttpsAcceptance.ts`）。

## 3. 首次登录与账号

1. 服务启动且密码认证开启时，会 bootstrap 默认管理员 `admin`（默认密码 `admin`，以配置为准）。
2. 首次登录若仍为 bootstrap 默认密码，服务端返回 `must_change_password=true` 并拦截业务 API；Dashboard 强制改密页引导完成后需重新登录。
3. 为业务账号创建 `user` 角色，并按库授予 `read` / `write`。
4. 在「权限矩阵」页对照 admin/user 能力，用非 admin 账号验证查询/写入降级路径。

## 4. 反向代理示例（nginx）

```nginx
server {
  listen 443 ssl http2;
  server_name mts.example.com;
  ssl_certificate     /etc/ssl/mts.crt;
  ssl_certificate_key /etc/ssl/mts.key;
  add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

  location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Request-ID $request_id;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
  }
}
```

## 5. 日常运维

| 场景 | 动作 |
|---|---|
| 健康巡检 | 抓取 `/healthz` `/readyz`；Dashboard 概览页 |
| 内存/刷盘压力 | 运维页 Flush；观察 maintenance/compaction 统计 |
| 压缩 | Compact（按运维策略，避免高峰全量） |
| 保留策略 | Retention 立即执行或确认周期任务 |
| 降采样 | 策略 enable/disable/run/reset/dry-run |
| 配置热更 | Config validate → reload（变更前备份配置文件） |
| 快照 | Storage 创建快照并异地拷贝 |

## 6. 应急

1. **服务不可用**：检查进程、数据目录权限、磁盘空间、`/readyz` 原因。
2. **登录失败**：确认密码认证配置、时钟偏移、token TTL；必要时用服务管理 token 救急（若已配置）。
3. **写放大 / 磁盘满**：停写入流量 → Flush → 评估 retention/compact → 扩容磁盘。
4. **误删 / 数据可疑**：停止写入 → 用最近快照恢复到旁路目录验证 → 再切换。
5. **权限事故**：禁用相关用户 → 审计页检索 → 修正库级授权。

## 7. 安全注意

- 生产关闭或限制 pprof（`/debug/pprof`）对外暴露。
- CSP / X-Frame-Options / nosniff 由服务默认设置；若前置 CDN 改写头，需保持等价强度。
- 审计日志与 `_internal.audit_log` 需纳入备份范围（若启用持久化审计）。
- 会话 token 视为密钥；Dashboard 使用本地存储，公共终端需强制退出。

## 8. 相关文档与代码

- 可商用基线：`docs/plans/dashboard-commercial-baseline-2026-07-19.md`
- P13 冒烟：`cmd/mts-server/dashboard_commercial_smoke_test.go`
- 生产清单数据：`cmd/mts-dashboard/src/utils/productionChecklist.ts`
- 权限矩阵数据：`cmd/mts-dashboard/src/utils/rbacMatrix.ts`


## 9. 备份演练（最短路径）

主机侧推荐脚本：`scripts/mts-backup.sh`（说明见 `docs/ops/backup-orchestration.md`）。


1. Dashboard → 存储：执行验证 → 创建快照 → 导出配置。
2. 将快照目录拷贝到旁路介质。
3. Dashboard 存储页执行「创建 data_dir 快照」→「执行旁路恢复演练」（`POST /api/v1/admin/storage/data-snapshot` + `restore-drill`），或 CLI/测试 `TestDataDirSidePathRestoreDrill`；也可旁路 data_dir 启动临时 mts-server 做查询比对。
4. 在存储页备份演练清单勾选主机侧步骤，保留演练记录。
