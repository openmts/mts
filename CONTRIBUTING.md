# Contributing

## 环境

- Go 1.26.4 或更高兼容版本
- `goimports-reviser`
- `golangci-lint`

本项目使用 Go 自动工具链下载时需要启用官方 checksum DB。仓库 `Makefile` 和 `make ci` 默认使用 `GOSUMDB=sum.golang.org`，避免本机全局 `GOSUMDB=off` 阻止 `go1.26.4` 工具链校验下载。确需覆盖时使用 `MTS_GOSUMDB=<value>`。

## 本地验证

```bash
make fmt
make test
make lint
make coverage
make e2e
timeout 60s git diff --check
```

完整商用门禁执行 `make ci`，单场景验证可通过 `make help` 查看。

生产包覆盖率未达到 90% 时，不应声明商用达标。`tests/**` 下的 e2e、fault、scale 和 pprof harness 做行为验证，不作为商用库行覆盖率分母。

## 发布前 API 兼容检查

首个 release tag 之后，每次发布前对根包执行 API 兼容检查：

```bash
go run golang.org/x/exp/cmd/apidiff@latest github.com/openmts/mts@<previous-tag> .
```

当前仓库尚无 release tag，因此本轮无法对历史基线执行 `apidiff`。

## 开发边界

- 只支持单机本地目录版本。
- 不实现分布式查询、分布式存储、跨节点副本和故障转移。
- 不接入 ETCD、ZooKeeper、Consul 等外部元数据系统。
- 不提供 SQL、InfluxQL、PromQL 或 MetricsQL parser；查询入口以 Builder/API 为主。

## 提交信息

使用 Conventional Commits：

```text
feat(scope): 动词开头的中文摘要
fix(scope): 动词开头的中文摘要
docs(scope): 动词开头的中文摘要
test(scope): 动词开头的中文摘要
```
