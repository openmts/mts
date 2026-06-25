# mts-server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 `cmd/mts-server` 可执行文件，通过配置文件启动 MTS Engine 并提供 HTTP/gRPC 访问。

**Architecture:** `cmd/mts-server` 内部按配置、CLI、运行时、HTTP、gRPC、测试拆分。HTTP/gRPC 均调用同一个 `serverRuntime`，避免业务逻辑重复；配置文件使用 YAML，gRPC 使用官方 gRPC transport 加 JSON codec。

**Tech Stack:** Go 1.26.4、`urfave/cli/v2`、`github.com/goccy/go-yaml`、`google.golang.org/grpc`、标准库 `net/http`、public `github.com/openmts/mts` API。

---

## 文件结构

- Create: `cmd/mts-server/main.go`：进程入口。
- Create: `cmd/mts-server/app.go`：urfave/cli app、serve action 和 print-config。
- Create: `cmd/mts-server/config.go`：YAML 配置结构、默认值、加载和校验。
- Create: `cmd/mts-server/runtime.go`：Engine 生命周期、HTTP/gRPC listener 启停和优雅关闭。
- Create: `cmd/mts-server/http.go`：HTTP JSON API。
- Create: `cmd/mts-server/grpc.go`：gRPC JSON codec、服务描述和方法。
- Create: `cmd/mts-server/*_test.go`：配置、CLI、HTTP、gRPC 行为测试。
- Create: `configs/mts-server.yaml`：带完整注释说明的示例配置文件。
- Modify: `go.mod` / `go.sum`：新增 `urfave/cli/v2`、`github.com/goccy/go-yaml` 和 gRPC 必要依赖。
- Modify: `Makefile`：新增 `mts-server` 定向验证目标。
- Modify: `README.md` / `llms.txt`：补充 server 启动说明。

## Task 1: 配置与 CLI

**状态:** 已完成。

**EARS:** When 用户提供配置文件时，系统应能加载、默认化、校验并传给 serve action。

- [x] 写失败测试：配置缺失、默认配置、非法配置、`--print-config`。
- [x] 实现 `config.go`、`app.go`、`main.go`。
- [x] 运行定向包测试覆盖 CLI 与配置路径。

**实现备注:** CLI 使用 `urfave/cli/v2`，默认命令为 `serve`；配置使用 `github.com/goccy/go-yaml` 严格解码、默认值合并和 `mts.Options.Validate()` 复用。

## Task 2: HTTP 服务

**状态:** 已完成。

**EARS:** When HTTP 客户端访问结构化 API 时，系统应执行健康检查、写入、查询、flush 和 compact。

- [x] 写失败测试：health、write/query rows、错误 JSON、维护接口和方法错误。
- [x] 实现 `runtime.go` 和 `http.go`。
- [x] 运行定向包测试覆盖 HTTP 与 runtime 生命周期。

**实现备注:** HTTP 暴露 `/healthz`、`/readyz`、`/api/v1/data/write`、`/api/v1/data/query/rows`、`/api/v1/admin/flush`、`/api/v1/admin/compact`，所有业务入口统一经过 `serverRuntime`。

## Task 3: gRPC 服务

**状态:** 已完成。

**EARS:** When gRPC 客户端调用服务方法时，系统应提供与 HTTP 等价能力并返回正确 status code。

- [x] 写失败测试：bufconn Health、Write、QueryRows、Flush、Compact、status code 和 interceptor。
- [x] 实现 `grpc.go`。
- [x] 运行定向包测试覆盖 gRPC 行为。

**实现备注:** gRPC 使用官方 `google.golang.org/grpc`，注册 `mts.v1.MTSServer` 服务和 JSON codec，不引入 proto 生成链。

## Task 4: 文档、示例配置与门禁

**状态:** 已完成。

**EARS:** When 开发者需要启动 mts-server 时，系统应提供完整配置文件和 README 说明。

- [x] 新增 `configs/mts-server.yaml`，并将旧 JSON 示例替换为带注释 YAML 示例。
- [x] 更新 README、llms.txt、Makefile，并将 `cmd/mts-server` 加入 CI 覆盖率门禁。
- [x] 运行 `make ci`、`govulncheck`、`git diff --check` 和产物扫描。

**实现备注:** `make mts-server-test` 提供新服务入口的定向验证；`make ci` 中 `cmd/mts-server` 覆盖率为 91.4%。

**验证结果:**

- `make ci`：通过，包含格式化、全量测试、golangci-lint、核心包覆盖率、回归 smoke 和产物扫描。
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`：通过，代码可达漏洞为 0。
- `git diff --check`：通过。
- 测试产物扫描：未发现遗留 `.test`、profile 或 coverage 文件。
