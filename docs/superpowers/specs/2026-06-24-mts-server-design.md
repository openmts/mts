# mts-server Design

## Goal

新增 `cmd/mts-server` 原生可执行文件，用配置文件启动 MTS 单机引擎，并同时提供 HTTP 与 gRPC 访问。

## EARS Requirements

- When 用户执行 `mts-server serve --config <path>` 时，系统应读取 YAML 配置文件、校验参数、打开 MTS Engine，并启动启用的 HTTP/gRPC listener。
- When 配置文件缺失、格式错误或关键字段非法时，系统应返回明确错误且不启动服务。
- When HTTP 客户端访问 `/healthz` 或 `/readyz` 时，系统应返回 Engine 健康状态 JSON。
- When HTTP 客户端 POST `/api/v1/write` 时，系统应按请求中的 points 和 write options 写入数据。
- When HTTP 客户端 POST `/api/v1/query/rows` 时，系统应按结构化 Query 返回 rows，不引入 SQL/PromQL/InfluxQL parser。
- When HTTP 客户端 POST `/api/v1/flush` 或 `/api/v1/compact` 时，系统应执行对应维护动作并返回 JSON 结果。
- When gRPC 客户端调用 Health、Write、QueryRows、Flush、Compact 时，系统应提供与 HTTP 等价的能力。
- When 进程收到 SIGINT/SIGTERM 时，系统应优雅关闭 HTTP/gRPC listener 和 Engine。

## Design

- CLI 使用 `github.com/urfave/cli/v2`，默认命令为 `serve`，支持 `--config` 和 `--print-config`。
- 配置文件使用 YAML，并通过 `github.com/goccy/go-yaml` 做严格字段校验；示例配置写入 `configs/mts-server.yaml`，文件内保留完整注释说明。
- HTTP 使用标准库 `net/http`，只暴露结构化 JSON API，不实现 parser。
- gRPC 使用 `google.golang.org/grpc` 和自定义 JSON codec，避免引入 proto 生成工具链；服务描述在代码中注册。
- Engine 使用 public `github.com/openmts/mts` API，避免跨越内部边界。

## Verification

- `go test ./cmd/mts-server -count=1 -timeout 180s`
- `make ci`
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`
