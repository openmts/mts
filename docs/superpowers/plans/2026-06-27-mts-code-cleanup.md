# MTS Code Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 基于重复代码检视报告完成代码精简和重复逻辑收敛。

**Architecture:** 新增职责明确的 `internal/collections` 和 `internal/httpjson` 小包；`cmd/mts-server` 通过统一错误分类、credential source 和协议常量收敛 HTTP/gRPC 重复。

**Tech Stack:** Go 1.26.4、泛型、`cmp`、`slices`、标准库 `net/http`、gRPC。

---

## EARS 任务清单

### Task 1：泛型集合工具

**EARS:** When 多个包需要浅拷贝 map/slice 或排序 map key 时，系统应复用 `internal/collections`。

- [x] 新增 `internal/collections` 测试。
- [x] 实现 `CloneMap`、`CloneMapNilIfEmpty`、`CloneSlice`、`SortedKeys`。
- [x] 替换报告中列出的 clone map 和 sorted key 机械重复。
- [x] 运行 `timeout 180s go test ./internal/collections ./internal/catalog ./internal/queryexec ./internal/queryservice ./internal/user ./internal/observability ./internal/engine ./internal/wal -count=1 -timeout 180s`。

实现备注：新增职责明确的 `internal/collections`，只承载泛型集合纯函数；业务深拷贝未迁移，避免错误抽象。

### Task 2：HTTP JSON 工具

**EARS:** When HTTP handler 需要严格 decode 或 JSON write 时，系统应复用 `internal/httpjson`。

- [x] 新增 `internal/httpjson` 测试。
- [x] 实现 `DecodeStrict`、`Write`、`WriteRaw`。
- [x] 替换 `cmd/mts-server`、`internal/queryservice`、`internal/service` 的重复 JSON helpers。
- [x] 运行 `timeout 180s go test ./internal/httpjson ./cmd/mts-server ./internal/queryservice ./internal/service -count=1 -timeout 180s`。

实现备注：HTTP JSON 基础能力已抽为内部小包，业务错误响应结构仍由各协议层控制。

### Task 3：mts-server 错误分类和鉴权收敛

**EARS:** When HTTP/gRPC 需要错误响应或认证授权时，系统应使用统一分类和 credential source，协议层只做转换。

- [x] 补充 HTTP/gRPC 未知错误映射测试。
- [x] 实现 `classifyAPIError` 和协议转换。
- [x] 实现 credential source 并合并 admin/data 鉴权逻辑。
- [x] 运行 `timeout 180s go test ./cmd/mts-server -run 'Test.*Error|Test.*Auth|Test.*Credential|Test.*Protocol' -count=1 -timeout 180s`。

实现备注：HTTP/gRPC 已共享错误分类和鉴权主体逻辑，未知错误默认映射为 HTTP 500 / gRPC Internal。

### Task 4：协议常量化和薄包装收敛

**EARS:** When 注册 route/method 或写 stream record 时，系统应使用常量和小 wrapper，避免魔法字符串。

- [x] 新增 `cmd/mts-server/constants.go`。
- [x] 常量化 HTTP route、gRPC method、stream type、bearer prefix、通用错误消息。
- [x] 用 admin/data wrapper 收敛重复的 method/admin/data guard。
- [x] 运行 `timeout 180s go test ./cmd/mts-server -count=1 -timeout 180s`。

实现备注：本轮以常量化和小 wrapper 完成低风险收敛；大型 operation registry 评估为当前收益低于行为回归风险，明确不纳入本轮实现项。

### Task 5：测试 helper 和报告闭环

**EARS:** When 测试存在局部高频 open/close 重复时，系统应在对应包内收敛 helper，并更新检视报告状态。

- [x] 在根包和 internal/engine 局部补充/复用 open/close helper。
- [x] 更新 `docs/review/code-review-2026-06-27-1540.md` 状态。
- [x] 保持“不建议处理”的持久化 reader/writer 和顶层 utils 结论不变。

实现备注：新增包内测试 helper，并只替换低风险 open/close 模板；复杂错误路径保留显式关闭和错误报告。

### Task 6：最终验证

**EARS:** When 清理完成时，系统应通过格式化、lint、全量测试和 diff 检查。

- [x] 运行 `timeout 300s /root/go/bin/goimports-reviser -rm-unused -format ./...`。
- [x] 运行 `timeout 600s /root/go/bin/golangci-lint run ./...`。
- [x] 运行 `timeout 600s go test ./... -count=1 -timeout 10m`。
- [x] 运行 `timeout 120s git diff --check`。

实现备注：格式化、lint、全量测试和 diff 空白检查均已通过。
