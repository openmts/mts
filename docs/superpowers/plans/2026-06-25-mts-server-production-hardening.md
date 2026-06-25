# mts-server Production Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `mts-server` 补齐为可安全部署、可观测、可治理的单机服务端启动程序，但不包含发布制品/流水线。

**Architecture:** 保持单机 Engine 和现有 HTTP/gRPC JSON codec。新增 TLS、auth、limits、observability、contract、operations、CLI command 等小模块，并通过 HTTP middleware 与 gRPC unary interceptor 接入横切能力。

**Tech Stack:** Go、net/http、crypto/tls、google.golang.org/grpc、goccy/go-yaml、urfave/cli、httptest、bufconn。

---

### Task 1: 配置模型、TLS 与运行时基础

**Files:**
- Modify: `cmd/mts-server/config.go`
- Create: `cmd/mts-server/tls.go`
- Modify: `cmd/mts-server/runtime.go`
- Test: `cmd/mts-server/config_test.go`

- [x] Step 1: 扩展配置结构，覆盖 TLS、limits、observability、backup、log、config file path。
- [x] Step 2: 实现 HTTP/gRPC TLS 配置构造，支持 TLS 与 mTLS。
- [x] Step 3: 将 HTTP timeout、TLSConfig、gRPC message size 应用到 runtime。
- [x] Step 4: 运行 `GOSUMDB=sum.golang.org timeout 180s go test ./cmd/mts-server -run 'TestLoadConfig|TestTLS|TestRuntime' -count=1 -timeout 3m`。

**实现备注:** 已在配置模型中补齐 TLS、超时、消息大小、limits、observability、backup、log 和配置路径；HTTP 与 gRPC 启动时都会构造 TLS 配置，证书、私钥、CA 无效时启动前失败，TLS 最低版本为 1.2。最终使用完整 `cmd/mts-server` 测试和 `make ci` 覆盖该任务。

### Task 2: 认证增强、请求身份与用户禁用管控

**Files:**
- Modify: `cmd/mts-server/auth.go`
- Create: `cmd/mts-server/security_context.go`
- Modify: `cmd/mts-server/http_data.go`
- Modify: `cmd/mts-server/grpc.go`
- Test: `cmd/mts-server/http_test.go`
- Test: `cmd/mts-server/grpc_test.go`

- [x] Step 1: 新增 data token 校验，支持 HTTP header 和 gRPC metadata。
- [x] Step 2: 支持 `auth.require_user`，未携带用户身份时拒绝数据面请求。
- [x] Step 3: 数据面权限校验时拒绝 Disabled 用户。
- [x] Step 4: 运行 `GOSUMDB=sum.golang.org timeout 180s go test ./cmd/mts-server -run 'TestHTTP.*Auth|TestGRPC.*Auth|Test.*Disabled' -count=1 -timeout 3m`。

**实现备注:** 数据面支持 `X-MTS-Data-Token`、Bearer token、gRPC `x-mts-data-token` 和 `authorization` metadata；`auth.require_user` 开启后读写必须携带用户身份，且会检查 DB 级权限与用户禁用状态。最终使用完整 `cmd/mts-server` 测试和协议 e2e 覆盖该任务。

### Task 3: HTTP/gRPC 治理与观测横切层

**Files:**
- Create: `cmd/mts-server/middleware.go`
- Create: `cmd/mts-server/metrics.go`
- Modify: `cmd/mts-server/http.go`
- Modify: `cmd/mts-server/grpc.go`
- Modify: `cmd/mts-server/http_admin.go`
- Test: `cmd/mts-server/http_test.go`
- Test: `cmd/mts-server/grpc_test.go`

- [x] Step 1: 实现 request id、HTTP access log、metrics 和 security headers middleware。
- [x] Step 2: 实现 HTTP body limit、concurrency limit、查询 limit、写入点数限制。
- [x] Step 3: 实现 gRPC unary interceptor 的 request id、metrics、access log 和并发限制。
- [x] Step 4: 实现受 admin token 保护的 pprof endpoint。
- [x] Step 5: 运行 `GOSUMDB=sum.golang.org timeout 180s go test ./cmd/mts-server -run 'TestHTTP.*Limit|TestHTTP.*Metrics|TestGRPC.*Limit|Test.*Pprof' -count=1 -timeout 3m`。

**实现备注:** HTTP middleware 和 gRPC unary interceptor 已统一接入 request id、访问日志、metrics、请求超时和并发限制；HTTP 请求体、写入点数、查询 limit、gRPC 消息大小均由配置治理。pprof 仅在启用后挂载，并要求 admin token。

### Task 4: API 契约、错误码和 CLI 运维命令

**Files:**
- Create: `cmd/mts-server/api_contract.go`
- Modify: `cmd/mts-server/http_admin.go`
- Modify: `cmd/mts-server/grpc.go`
- Modify: `cmd/mts-server/app.go`
- Test: `cmd/mts-server/config_test.go`
- Test: `cmd/mts-server/grpc_test.go`

- [x] Step 1: 实现 `/api/v1/admin/api-spec`、`/api/v1/admin/error-codes` 与 gRPC RPC。
- [x] Step 2: 实现 `version`、`validate-config`、`doctor`、`init-config` CLI 命令。
- [x] Step 3: 确保 `init-config` 写文件权限 0600，已有文件未 `--force` 时拒绝覆盖。
- [x] Step 4: 运行 `GOSUMDB=sum.golang.org timeout 180s go test ./cmd/mts-server -run 'TestCLI|TestAPIContract|TestGRPC.*Spec' -count=1 -timeout 3m`。

**实现备注:** 管理面新增 API contract 与错误码契约 HTTP/gRPC 接口；CLI 新增 `version`、`validate-config`、`doctor`、`init-config`，默认配置输出为 YAML，文件写入权限为 0600，目录创建权限为 0700。

### Task 5: 配置校验、热重载、数据运维和用户审计

**Files:**
- Create: `cmd/mts-server/config_admin.go`
- Create: `cmd/mts-server/operations.go`
- Create: `cmd/mts-server/audit.go`
- Modify: `cmd/mts-server/http_admin.go`
- Modify: `cmd/mts-server/http_users.go`
- Modify: `cmd/mts-server/grpc.go`
- Test: `cmd/mts-server/http_test.go`
- Test: `cmd/mts-server/grpc_test.go`

- [x] Step 1: 实现配置 validate/reload HTTP 与 gRPC 接口，只热更新 auth、limits、observability、log。
- [x] Step 2: 实现 SIGHUP 触发同样的 reload 路径。
- [x] Step 3: 实现 storage validate、snapshot、export 接口，目录 0700、文件 0600。
- [x] Step 4: 实现用户审计事件记录和 `/api/v1/users/{name}/audit`。
- [x] Step 5: 运行 `GOSUMDB=sum.golang.org timeout 180s go test ./cmd/mts-server -run 'TestHTTP.*Config|TestHTTP.*Storage|TestHTTP.*Audit|TestGRPC.*Config' -count=1 -timeout 3m`。

**实现备注:** 配置 validate/reload 已提供 HTTP 与 gRPC 接口；SIGHUP 复用 reload 路径，且只热更新 auth、limits、observability 和 log，不重载 listener、TLS、engine 或 data_dir。数据运维已补 storage validate、snapshot、export；snapshot 目录和文件权限分别为 0700 和 0600。用户创建、更新、删除、授权、撤销会写入内存审计日志，并通过用户 audit 接口查询。

### Task 6: 文档、e2e 和最终门禁

**Files:**
- Modify: `configs/mts-server.yaml`
- Modify: `README.md`
- Modify: `tests/e2e/mts_server_protocols/main.go`
- Modify: `tests/e2e/README.md`
- Modify: `docs/superpowers/plans/2026-06-25-mts-server-production-hardening.md`

- [x] Step 1: 更新 YAML 示例和 README，说明服务端生产增强能力。
- [x] Step 2: 扩展 e2e smoke 覆盖 API spec、data token、metrics 和 storage validate。
- [x] Step 3: 运行 `GOSUMDB=sum.golang.org timeout 300s go test ./cmd/mts-server -count=1 -timeout 5m`。
- [x] Step 4: 运行 `GOSUMDB=sum.golang.org timeout 300s go test ./tests/e2e/mts_server_protocols -count=1 -timeout 5m`。
- [x] Step 5: 运行 `GOSUMDB=sum.golang.org timeout 600s make ci`。
- [x] Step 6: 运行 `GOPATH_VALUE=$(GOSUMDB=sum.golang.org go env GOPATH) && GOSUMDB=sum.golang.org timeout 600s "$GOPATH_VALUE/bin/govulncheck" ./...`。
- [x] Step 7: 运行 `timeout 60s git diff --check` 和临时产物扫描。

**实现备注:** `configs/mts-server.yaml` 和 README 已补生产增强配置说明；协议 e2e 已覆盖 data token、HTTP/gRPC 管理接口、API spec、metrics 和 storage validate smoke。

**最终验证记录:**

- `GOSUMDB=sum.golang.org timeout 300s go test ./cmd/mts-server -coverprofile=/tmp/mts-server-hardening.out -count=1 -timeout 5m`：通过，`cmd/mts-server` 覆盖率 90.0%。
- `GOSUMDB=sum.golang.org timeout 300s go test ./cmd/mts-server -count=1 -timeout 5m`：通过。
- `GOSUMDB=sum.golang.org timeout 300s go test ./tests/e2e/mts_server_protocols -count=1 -timeout 5m`：通过。
- `GOSUMDB=sum.golang.org timeout 600s make ci`：通过，lint 0 issues，核心包覆盖率均 >= 90%。
- `GOPATH_VALUE=$(GOSUMDB=sum.golang.org go env GOPATH) && GOSUMDB=sum.golang.org timeout 600s "$GOPATH_VALUE/bin/govulncheck" ./...`：通过，当前代码路径 0 个可达漏洞。
- `timeout 60s git diff --check`：通过。
- `timeout 60s find . -type f \( -name '*.test' -o -name '*.prof' -o -name '*.pprof' -o -name 'coverage.out' -o -name '*.coverprofile' \) -not -path './.git/*' -print`：无输出，未发现遗留测试产物。

## Self-review

- Spec 覆盖：1-9 项均映射到 Task 1-6。
- Scope 控制：未包含第 10 点发布制品/流水线，未引入外部 IAM、分布式配置或 parser。
- 验证：每个 task 有定向 Go 测试，最终有 e2e、make ci、govulncheck 和产物扫描。
