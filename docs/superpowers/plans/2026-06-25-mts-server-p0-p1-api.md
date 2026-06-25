# mts-server P0/P1 API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补齐 `mts-server` P0/P1 HTTP 与 gRPC 外部访问接口，覆盖数据、查询、用户权限、元数据、配置、运维观测和降采样。

**Architecture:** 保持单机 Engine 和 LocalMetadataStore 边界，复用当前 HTTP JSON 与 gRPC JSON codec。按 handler 责任拆分 `cmd/mts-server` 文件，新增轻量 auth 和统一错误映射，不引入 SQL parser、外部权限系统或 protobuf 生成链。

**Tech Stack:** Go、net/http、google.golang.org/grpc、goccy/go-yaml、MTS public Engine API、httptest、bufconn、Makefile e2e。

---

### Task 1: 协议类型、认证与测试基线

**Files:**
- Create: `cmd/mts-server/protocol_types.go`
- Create: `cmd/mts-server/auth.go`
- Modify: `cmd/mts-server/config.go`
- Modify: `cmd/mts-server/http_test.go`
- Modify: `cmd/mts-server/grpc_test.go`

- [x] Step 1: 新增 HTTP/gRPC 目标行为测试，覆盖 admin token、typed write、columns/explain、用户接口、元数据接口。
- [x] Step 2: 运行 `timeout 180s go test ./cmd/mts-server -run 'TestHTTP.*P0P1|TestGRPC.*P0P1|TestHTTPAdminAuth' -count=1 -timeout 3m`，确认新增测试因接口缺失失败。
- [x] Step 3: 实现 DTO、auth 配置、统一错误响应基础。
- [x] Step 4: 运行同一测试，进入下一批缺口。

**实现备注:** 已新增 admin token 配置、HTTP/gRPC auth helper、统一 `apiError` 响应、P0/P1 HTTP 和 gRPC 目标测试。RED 验证显示测试因缺失 API 返回 404 或 gRPC unknown method 失败，符合预期。

### Task 2: 数据写入与查询 P0 接口

**Files:**
- Create: `cmd/mts-server/http_data.go`
- Modify: `cmd/mts-server/http.go`
- Modify: `cmd/mts-server/grpc.go`
- Modify: `cmd/mts-server/runtime.go`
- Test: `cmd/mts-server/http_test.go`
- Test: `cmd/mts-server/grpc_test.go`

- [x] Step 1: 实现 typed batch 写入 HTTP/gRPC。
- [x] Step 2: 实现 query columns、query explain、query stats、HTTP NDJSON stream、gRPC 查询扩展。
- [x] Step 3: 接入可选用户 DB read/write 权限校验。
- [x] Step 4: 运行 `timeout 180s go test ./cmd/mts-server -run 'TestHTTP.*P0P1|TestGRPC.*P0P1|TestHTTPAdminAuth' -count=1 -timeout 3m`。

**实现备注:** HTTP 数据面已补点写入、typed batch、rows、columns、explain、NDJSON stream 和 stats；gRPC 已补 typed batch、columns、explain、stats。数据面支持 `X-MTS-User`/gRPC metadata 用户身份的 DB read/write 权限校验，未携带用户时保持开发兼容模式。

### Task 3: 用户权限与元数据接口

**Files:**
- Create: `cmd/mts-server/http_users.go`
- Create: `cmd/mts-server/http_meta.go`
- Modify: `cmd/mts-server/grpc.go`
- Modify: `cmd/mts-server/runtime.go`
- Test: `cmd/mts-server/http_test.go`
- Test: `cmd/mts-server/grpc_test.go`

- [x] Step 1: 实现用户 CRUD、权限 grant/revoke/list/check。
- [x] Step 2: 实现 database、retention policy、measurement、fields、series 接口。
- [x] Step 3: 接入 admin/read 权限校验。
- [x] Step 4: 运行 `timeout 180s go test ./cmd/mts-server -run 'TestHTTP.*P0P1|TestGRPC.*P0P1|TestHTTPAdminAuth' -count=1 -timeout 3m`。

**实现备注:** HTTP/gRPC 已补用户 CRUD、DB 权限 grant/revoke/list/check、database/retention 管理和 measurement/field/series 发现接口。用户面和管理面受 admin token 保护，数据发现接口支持 DB read 权限校验。

### Task 4: 配置、运维观测与 P1 降采样接口

**Files:**
- Create: `cmd/mts-server/http_admin.go`
- Modify: `cmd/mts-server/http.go`
- Modify: `cmd/mts-server/grpc.go`
- Modify: `cmd/mts-server/runtime.go`
- Test: `cmd/mts-server/http_test.go`
- Test: `cmd/mts-server/grpc_test.go`

- [x] Step 1: 实现 config、effective config、config schema。
- [x] Step 2: 实现 metrics、retention apply、maintenance errors、storage memory、compaction stats、admin health。
- [x] Step 3: 实现降采样 policy CRUD、enable/disable/reset/status/run/run-range/repair/dry-run。
- [x] Step 4: 运行 `timeout 180s go test ./cmd/mts-server -run 'TestHTTP.*P0P1|TestGRPC.*P0P1|TestHTTPAdminAuth' -count=1 -timeout 3m`。

**实现备注:** 已补配置只读、配置 schema、Prometheus metrics、retention apply、maintenance errors、storage memory、compaction stats、admin health，以及降采样策略管理和手动执行接口。`/metrics` 复用 Engine 指标并补 server health ready/healthy 指标。

### Task 5: e2e、文档和质量门禁

**Files:**
- Modify: `tests/e2e/mts_server_protocols/*`
- Modify: `tests/e2e/README.md`
- Modify: `README.md`
- Modify: `configs/mts-server.yaml`
- Modify: `docs/superpowers/plans/2026-06-25-mts-server-p0-p1-api.md`

- [x] Step 1: 扩展 mts-server e2e，覆盖 HTTP/gRPC P0/P1 smoke。
- [x] Step 2: 更新 README 和配置注释，保持简洁。
- [x] Step 3: 运行 `timeout 300s go test ./cmd/mts-server -count=1 -timeout 5m`。
- [x] Step 4: 运行 `timeout 300s go test ./tests/e2e/mts_server_protocols -count=1 -timeout 5m`。
- [x] Step 5: 运行 `timeout 600s make ci`。
- [x] Step 6: 运行 `timeout 600s govulncheck ./...`。
- [x] Step 7: 运行 `timeout 60s git diff --check` 并扫描临时产物。

**实现备注:** 已扩展 `mts_server_protocols` e2e 覆盖 HTTP/gRPC P0/P1 smoke；README 与 YAML 配置说明保持简洁更新。最终验证通过：`go test ./cmd/mts-server`、`go test ./tests/e2e/mts_server_protocols`、`make ci`。`make ci` 中 `cmd/mts-server` 覆盖率为 90.3%，满足 >=90% 门禁。已安装并运行官方 `govulncheck`，结果为调用路径 0 漏洞；`git diff --check` 通过，工作区临时测试产物扫描无输出。

## Self-review

- Spec 覆盖：P0/P1 数据、查询、用户、元数据、配置、运维观测、降采样均映射到 Task 2-4。
- Scope 控制：storage repair/restore/migrate、SQL parser、外部 IAM、protobuf 生成链明确不在本计划内。
- 验证：每个 task 都有定向测试，最终有 e2e、make ci、govulncheck 和 diff 检查。
