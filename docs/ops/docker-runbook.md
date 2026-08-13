# mts-server Docker 运行手册

镜像由 CI 构建并推送到 GitHub Container Registry（GHCR），二进制已内嵌 mts-dashboard，容器启动后访问 HTTP 端口即可打开管理页面。

## 1. 镜像来源

- `ghcr.io/<owner>/mts-server:dev`：pre-release 版本，每次 main 分支 push 后更新，用于验证/试运行。
- `ghcr.io/<owner>/mts-server:<tag>`：正式版本，与 `v*` git tag 对应（如 `v1.0.0`）。
- 本仓库默认 owner 为 `openmts`，即 `ghcr.io/openmts/mts-server:dev`。

镜像支持 `linux/amd64` 与 `linux/arm64` 双平台，Docker 会自动选择匹配当前主机的平台。

## 2. 快速开始

```bash
docker pull ghcr.io/openmts/mts-server:dev

docker run -d --name mts-server \
  -p 8086:8086 -p 9096:9096 \
  -v mts-data:/data \
  --restart unless-stopped \
  ghcr.io/openmts/mts-server:dev
```

- HTTP API 与 Dashboard：`http://localhost:8086`（浏览器打开即管理页面）。
- gRPC：`localhost:9096`。
- 数据持久化在 named volume `mts-data`（容器内 `/data`），删除容器不会丢失数据。

## 3. 健康检查与日志

```bash
curl -fsS http://127.0.0.1:8086/healthz   # 存活
curl -fsS http://127.0.0.1:8086/readyz    # 就绪
curl -fsS http://127.0.0.1:8086/metrics   # Prometheus 指标

docker logs -f mts-server                 # 跟踪日志
docker exec mts-server mts-server version # 查看镜像内版本
```

## 4. 自定义配置

容器默认配置见 `deploy/docker/mts-server.yaml`（监听 `[::]` 全零 IPv4/IPv6 双栈、数据目录 `/data`）。生产部署将自定义配置挂载到 `/etc/mts-server/config.yaml` 覆盖默认值：

```bash
docker run -d --name mts-server \
  -p 8086:8086 -p 9096:9096 \
  -v mts-data:/data \
  -v "$PWD/mts-server.yaml:/etc/mts-server/config.yaml:ro" \
  ghcr.io/openmts/mts-server:v1.0.0
```

首次生产部署应配置强随机 `auth.admin_token`，再通过用户 API 创建管理员；开启 `auth.require_user` 后数据面必须携带用户 token。配置字段说明见 `configs/mts-server.yaml` 内注释。

## 5. Docker Compose 示例

```yaml
services:
  mts-server:
    image: ghcr.io/openmts/mts-server:dev
    restart: unless-stopped
    ports:
      - "8086:8086"
      - "9096:9096"
    volumes:
      - mts-data:/data
      - ./mts-server.yaml:/etc/mts-server/config.yaml:ro
    healthcheck:
      test: ["CMD", "curl", "-fsS", "http://127.0.0.1:8086/healthz"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 5s

volumes:
  mts-data:
```

## 6. 数据与备份

- `/data` 是唯一持久化数据目录，包含 WAL、SSTable、manifest、metadata 与 `backups/` 子目录。
- 建议对 `/data` 做独立持久化（volume/宿主机目录），并通过管理 API `POST /api/v1/admin/storage/snapshot` 定期快照到 `backups/`，备份编排参考 `docs/ops/backup-orchestration.md`。

## 7. 平台与安全注意

- 容器以非 root 用户（uid 1001）运行；挂载宿主目录作为 `/data` 时需保证该 uid 有写权限（如 `chown -R 1001:1001`）。
- 容器默认监听 `[::]` 全零地址（IPv4/IPv6 双栈），暴露到公网前应放在反向代理后，并启用 TLS/鉴权（见 `docs/ops/dashboard-production-runbook.md`）。

## 8. 相关文件

- `Dockerfile`：多阶段构建（Dashboard → mts-server → 精简运行时）。
- `deploy/docker/mts-server.yaml`：容器默认配置。
- `.github/workflows/pre-release.yml`、`release.yml`：镜像构建与推送。
