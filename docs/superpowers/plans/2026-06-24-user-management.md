# User Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 MTS public API 增加接口化用户管理和默认本地 DB 级权限管理实现。

**Architecture:** public package 定义 `UserManager` 接口和用户/权限 DTO；默认 `LocalUserManager` 使用二进制 envelope 持久化 `users.bin`；`Engine` 持有注入或默认用户管理器，并暴露用户 CRUD、DB 权限管理和权限校验方法。现有读写查询 API 不被默认破坏，调用方可在入口层显式调用权限校验。

**Tech Stack:** Go、public `mts` package、`internal/codec`、`internal/storagefs`、`go test`、`goimports-reviser`、`golangci-lint`。

**预计耗时与硬超时:** 规格和计划 10 分钟；实现 75-120 分钟；定向 Go 测试 `-timeout 180s`；全量 `make ci` 10-15 分钟；e2e public/no-json `-timeout 300s`。

---

## 文件结构

- Create: `user_types.go`：public 用户、权限 DTO、`UserManager` 接口和错误。
- Create: `local_user_manager.go`：默认本地用户管理器、CRUD、权限管理、持久化入口。
- Create: `local_user_encoding.go`：二进制 envelope 编解码。
- Create: `local_user_permissions.go`：DB 级授权、撤销、列表和校验。
- Create: `local_user_helpers.go`：排序、clone 和状态复制辅助函数。
- Create: `user_management_test.go`、`user_permissions_test.go`、`user_engine_test.go`：public API 行为测试。
- Modify: `options.go`：增加 `UserManager UserManager` 配置字段。
- Modify: `engine_types.go`：Engine 保存用户管理器和默认关闭逻辑。
- Modify: `engine.go`：暴露用户 CRUD 和 DB 权限方法。
- Modify: `tests/e2e/public_api_workflow/*`：覆盖公开用户管理 workflow。
- Modify: `README.md`、`doc.go`、`llms.txt`：补充用户管理和权限说明。
- Modify: `cmd/mts-storage/main.go`、`cmd/mts-storage/main_test.go`：抽取可测试入口以满足覆盖率门禁。

## Task 1: Public UserManager 合约

**状态:** 已完成。

**EARS:** When 调用方使用 public API 管理用户时，系统应提供稳定接口和明确错误。

- [x] Step 1: 写失败测试，覆盖用户 CRUD、列表排序、metadata clone、错误类型。
  - Run: `timeout 180s go test . -run 'TestLocalUserManager.*User' -count=1 -timeout 180s`
  - Expected: FAIL，原因是类型和方法不存在。
- [x] Step 2: 新增 `user_types.go` 和 `local_user_manager.go` 的最小 CRUD 实现。
- [x] Step 3: 运行同一测试通过，并记录实现备注。

**实现备注:** 已提供 `UserManager` public 接口、`User`/`DatabaseGrant` DTO 和稳定错误；`LocalUserManager` 完成 Create/Update/Get/List/Delete，列表按用户名排序，返回值复制 metadata，避免调用方修改内部状态。

## Task 2: DB 级权限管理

**状态:** 已完成。

**EARS:** When 调用方授予、撤销或校验 DB 权限时，系统应按 read/write/admin 语义返回结果。

- [x] Step 1: 写失败测试，覆盖 grant/revoke/list/check、admin 隐含 read/write、disabled 用户拒绝、非法 permission。
  - Run: `timeout 180s go test . -run 'TestLocalUserManager.*Permission' -count=1 -timeout 180s`
  - Expected: FAIL，原因是权限方法不存在。
- [x] Step 2: 完成本地权限 map、稳定排序、权限校验和错误处理。
- [x] Step 3: 运行同一测试通过，并记录实现备注。

**实现备注:** DB 权限粒度为 `read`、`write`、`admin`；`admin` 隐含读写和管理权限；禁用用户与无授权用户均拒绝访问；grant 列表按 database 和权限顺序稳定输出。

## Task 3: 二进制持久化与 Engine 集成

**状态:** 已完成。

**EARS:** When Engine 打开时，系统应默认使用本地用户管理器；When 注入第三方 UserManager 时，系统应使用注入实现。

- [x] Step 1: 写失败测试，覆盖本地重启恢复、损坏文件打开失败、Engine 默认用户管理、Engine 注入自定义 UserManager。
  - Run: `timeout 180s go test . -run 'TestUserManagement.*Persistence|TestEngine.*UserManager' -count=1 -timeout 180s`
  - Expected: FAIL，原因是持久化和 Engine 方法不存在。
- [x] Step 2: 新增 `local_user_encoding.go`，修改 `Options`、`Open`、`Close`、Engine 用户方法。
- [x] Step 3: 运行同一测试通过，并记录实现备注。

**实现备注:** 默认本地实现持久化到引擎目录下 `users.bin`，使用二进制 envelope；`Options.UserManager` 注入第三方实现时不创建本地 `users.bin`；`Engine.Close` 会关闭默认本地用户管理器。

## Task 4: Public E2E 与文档

**状态:** 已完成。

**EARS:** When 外部用户使用默认用户管理模块时，public workflow 应能完成用户创建、授权、重启恢复和权限校验。

- [x] Step 1: 扩展 `tests/e2e/public_api_workflow`，覆盖用户创建、DB read/write/admin 授权和重启后校验。
- [x] Step 2: 更新 README、doc.go、llms.txt。
- [x] Step 3: 运行 `timeout 300s go test ./tests/e2e/public_api_workflow -count=1 -timeout 300s` 和 `timeout 300s go test ./tests/e2e/no_json_storage -count=1 -timeout 300s`。

**实现备注:** public workflow 已覆盖创建用户、授予 `admin`、重启后读取用户和权限、`admin` 隐含 `write`、其他 database 拒绝访问；no-json e2e 保持通过，确认本地用户元数据未引入 JSON 存储。

## Task 5: 质量门禁与收尾

**状态:** 已完成，`govulncheck` 存在 Go 1.26.2 标准库发布阻塞。

**EARS:** When 用户管理模块完成时，系统应通过格式化、测试、lint、e2e 和产物清理检查。

- [x] Step 1: 运行 `timeout 300s make fmt`。
- [x] Step 2: 运行 `timeout 300s go test . -run 'TestLocalUserManager|TestUserManagement|TestEngine.*UserManager|TestEngineUserManagementWrappers' -count=1 -timeout 180s`。
- [x] Step 3: 运行 `timeout 900s make ci`。
- [x] Step 4: 运行 `timeout 300s make e2e-public-api`。
- [x] Step 5: 运行 `timeout 300s make e2e-no-json`。
- [x] Step 6: 运行 `timeout 600s go run golang.org/x/vuln/cmd/govulncheck@latest ./...` 并记录结果。
- [x] Step 7: 运行 `timeout 60s git diff --check` 和临时产物扫描。
- [x] Step 8: 更新本计划每个 Task 状态和实现备注。

**实现备注:** `make ci` 通过，根包覆盖率为 `90.5%`，`cmd/mts-storage` 覆盖率为 `92.0%`；单独 e2e public/no-json 通过；`git diff --check` 通过；临时产物扫描未发现 `.test`、`coverage.out`、`.prof` 残留。`govulncheck` 失败于 Go 1.26.2 标准库可达漏洞 `GO-2026-5039`、`GO-2026-5037`、`GO-2026-4971`，需要升级发布工具链到 Go 1.26.4 或更高后再正式发布。
