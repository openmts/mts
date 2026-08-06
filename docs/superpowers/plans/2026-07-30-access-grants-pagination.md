# Access Grants 聚合分页 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用一次管理员聚合请求加载一页用户及其授权，消除 Dashboard 的 `U+1` 网络请求，并以分页上限、快照一致性和性能证据控制扩展风险。

**Architecture:** `internal/user.Manager` 在一次读锁中生成按用户名排序的授权页，runtime 与公共 Engine 只做类型转换；`mts-server` 暴露 additive HTTP GET contract；Dashboard 只持有当前页 rows 与 cursor 历史。既有用户、单用户授权、gRPC 与存储格式保持不变。

**Tech Stack:** Go 1.24+、`net/http`、现有 operation registry、Vue 3 Composition API、TypeScript、Node test runner、Playwright、Go benchmark/benchstat。

## Global Constraints

- 接口固定为 `GET /api/v1/users/access-grants?limit=100&cursor=<username>`，默认 limit 100，范围 1..200。
- 匿名/普通用户分别得到 401/403；只有管理员可读取。
- 单次响应内 users/grants 必须来自同一 `RLock` 快照。
- Dashboard 每次加载或翻页只允许一个聚合请求；筛选、选择、统计和导出只作用于当前页。
- 既有 HTTP/gRPC contract 与存储格式不得改变；不新增 gRPC RPC。
- 新目录权限 `0700`，新文件权限 `0600`；不在 for 循环中 defer；所有 error 必须显式处理。
- 生产 bundle gzip 增长不得超过 5%；相关 Go benchmark 不得出现无说明的 `ns/op`、`B/op`、`allocs/op` 劣化。
- 本会话不执行 commit、push 或远端操作。

---

### Task 1: 锁内授权分页快照与基线 benchmark

**Files:**
- Modify: `internal/user/types.go`
- Modify: `internal/user/contracts.go`
- Create: `internal/user/local_user_grant_page.go`
- Create: `internal/user/local_user_grant_page_test.go`
- Create: `internal/user/local_user_grant_page_benchmark_test.go`

**Interfaces:**
- Produces: `type UserGrantBundle struct { User User; Grants []Grant }`
- Produces: `type UserGrantPage struct { Items []UserGrantBundle; TotalUsers int; NextCursor string }`
- Produces: `ListUserGrantPage(context.Context, string, int) (UserGrantPage, error)` on a new focused `GrantPageStore` interface and `*Manager`.

- [x] **Step 1: 写旧读取模型 benchmark 并采集基线**

  使用同一 fixture 构造 10/100/1000 用户，每人三项授权；benchmark `BenchmarkUserGrantRead/legacy-users=N` 每轮执行一次 `ListUsers` 加 N 次 `ListPermissions`，报告 `requests/op = N+1`。运行：

  ```bash
  timeout 3m go test ./internal/user -run '^$' -bench '^BenchmarkUserGrantRead/legacy' -benchmem -benchtime=500ms -count=10 > /tmp/mts-access-grants-before.txt
  ```

- [x] **Step 2: 写失败的分页、删除 cursor、取消和快照测试**

  覆盖默认调用方参数已归一化为 limit、静态数据分页不重不漏、无授权用户保留、已删除 cursor 使用严格字典序后继、取消 context 返回错误；并发测试循环读取与 grant/revoke，在 `-race` 下不得产生数据竞争或同一 item 用户缺失。

- [x] **Step 3: 运行测试确认 RED**

  ```bash
  timeout 3m go test ./internal/user -run 'TestManagerListUserGrantPage' -count=1
  ```

  预期因 `ListUserGrantPage` 尚不存在而编译失败。

- [x] **Step 4: 实现最小锁内分页快照**

  `ListUserGrantPage` 先检查 `ctx.Err()`，要求 `limit > 0`；在一次 `m.mu.RLock()` 下调用 `sortedUserNames`，用 `sort.Search` 定位严格大于 cursor 的起点，复制页内 `User` 和 `sortedGrants`。当 `end < len(names)` 时，`NextCursor` 为本页最后一个用户名；否则为空。

- [x] **Step 5: 运行正确性、race 与新 benchmark**

  ```bash
  timeout 3m go test ./internal/user -count=1
  timeout 5m go test -race ./internal/user -run 'TestManagerListUserGrantPage' -count=1
  timeout 3m go test ./internal/user -run '^$' -bench '^BenchmarkUserGrantRead/(legacy|page)' -benchmem -benchtime=500ms -count=10 > /tmp/mts-access-grants-core-after.txt
  ```

  验收：page 子项 `requests/op=1`；1000 用户首屏只复制 100 个 bundle；对 legacy 共存基准无显著劣化。

### Task 2: 公共 Engine 与 HTTP additive contract

**Files:**
- Modify: `user_types.go`
- Modify: `engine.go`
- Modify: `internal/runtime/user_manager.go`
- Modify: `cmd/mts-server/constants.go`
- Modify: `cmd/mts-server/protocol_types.go`
- Create: `cmd/mts-server/http_access_grants.go`
- Create: `cmd/mts-server/access_grants_contract_test.go`
- Modify: `cmd/mts-server/operation_registry.go`
- Modify: `cmd/mts-server/operation_registry_test.go`

**Interfaces:**
- Consumes: `internal/user.GrantPageStore.ListUserGrantPage` from Task 1.
- Produces: `mts.UserGrantBundle`, `mts.UserGrantPage`, `(*Engine).ListUserGrantPage(ctx, cursor, limit)`.
- Produces: HTTP response `{items,total_users,next_cursor,path,admin_op_busy,op,started_at_unix,last}`.

- [x] **Step 1: 写失败的 Engine 转换和 HTTP contract 测试**

  覆盖 items/grants/metadata 深复制、默认 limit=100、limit 1/200、limit 0/201/非数字 `bad_request`、cursor 稳定分页、无授权用户、path/admin-op 元数据、匿名 401、普通用户 403、管理员 200、旧 `/users` 与单用户授权接口仍可用。

- [x] **Step 2: 运行定向测试确认 RED**

  ```bash
  timeout 3m go test ./... -run 'TestEngineListUserGrantPage|TestAccessGrantsContract' -count=1
  ```

- [x] **Step 3: 实现类型转换、query 解析与 handler**

  新增常量 `routeUsersAccessGrants`。`parseAccessGrantsPageQuery` 只接受 GET、解析 limit 并拒绝范围外值；cursor 保留内部空格但拒绝首尾空白。handler 首先 `requireHTTPAdmin`，随后调用 Engine 一次并附加 admin-op 元数据。operation registry 将该固定路径作为独立 `authAdmin` HTTP operation 注册，不新增 gRPC method。

- [x] **Step 4: 运行 server 契约、全包测试和 race**

  ```bash
  timeout 5m go test ./cmd/mts-server -run 'TestAccessGrantsContract|TestOperationRegistry' -count=1
  timeout 5m go test ./cmd/mts-server -count=1
  timeout 8m go test -race ./cmd/mts-server -run 'TestAccessGrantsContract' -count=1
  ```

### Task 3: Dashboard 当前页模型与分页交互

**Files:**
- Create: `cmd/mts-dashboard/src/utils/accessGrantsPagination.ts`
- Create: `cmd/mts-dashboard/src/utils/accessGrantsPagination.test.ts`
- Modify: `cmd/mts-dashboard/src/pages/AccessGrantsPage.vue`
- Modify: `cmd/mts-dashboard/src/i18n/messages.ts`
- Modify: `cmd/mts-dashboard/e2e/commercial-smoke.spec.ts`

**Interfaces:**
- Produces: `ACCESS_GRANTS_PAGE_LIMIT = 100`.
- Produces: `buildAccessGrantsPagePath(cursor?: string, limit?: number): string`.
- Produces: response types `AccessGrantItem` and `AccessGrantsPageResponse` plus `accessGrantItemsToBundles(items)`.

- [x] **Step 1: 写失败的 URL、转换和 cursor 历史单测**

  覆盖空 cursor、用户名编码、limit=100、无授权 item、用户 role/disabled 映射，以及前进后退时 cursor 栈不丢失。

- [x] **Step 2: 运行单测确认 RED**

  ```bash
  cd cmd/mts-dashboard && timeout 2m npm test -- --test-name-pattern='access grants pagination'
  ```

- [x] **Step 3: 实现当前页加载**

  删除 `/api/v1/users` 加 worker 的 `U+1` 逻辑；`load(cursor)` 仅调用一次聚合路径。增加 `pageCursor`、`cursorHistory`、`nextCursor`、`pageUserCount`、`totalUsers`，翻页成功后才提交 cursor 状态；失败时保留当前页与 cursor。每次新页清理选择并按当前 rows prune。

- [x] **Step 4: 增加当前页导航与明确文案**

  增加带图标或清晰命令的上一页/下一页按钮、当前页用户数/总用户数、`筛选与导出仅作用于当前页` 中英文说明。按钮有本地化 accessible name，loading 时禁用，第一页禁用上一页，无 `next_cursor` 时禁用下一页。

- [x] **Step 5: 更新 Playwright 请求计数断言**

  在商业冒烟中监听 Access Grants 页面请求，断言首屏恰好调用一次 `/api/v1/users/access-grants`，且没有逐用户 `/database-permissions` GET；创建超过一页 fixture 时验证下一页仍只新增一次聚合请求，并验证当前页提示与导航状态。

- [x] **Step 6: 运行 Dashboard unit/typecheck/build**

  ```bash
  cd cmd/mts-dashboard && timeout 3m npm test
  cd cmd/mts-dashboard && timeout 3m npm run typecheck
  cd cmd/mts-dashboard && timeout 5m npm run build
  ```

  实现备注：新增聚合分页 helper 与不可变 cursor 历史；Access Grants 页面改为每页单次聚合请求，并明确筛选、选择、统计和导出仅作用于当前页。定向 unit、typecheck、production build 及 `commercial browser smoke path` 均已通过；Playwright GREEN 为 1/1，用时 1.2 分钟。

### Task 4: 10/100/1000 性能、请求数和端到端证据

**Files:**
- Create: `cmd/mts-server/access_grants_benchmark_test.go`
- Create: `docs/benchmarks/access-grants-task10.txt`

**Interfaces:**
- Consumes: Task 1–3 完成的聚合页与 Dashboard 请求模型。
- Produces: 可复核的 old/new 请求数、server latency、response bytes、Dashboard 首屏时间和 bundle 对比。

- [x] **Step 1: 写 server HTTP benchmark**

  对 10/100/1000 用户 fixture 分别测默认首屏聚合 handler，报告 `response-B/op` 与 `users/op`；fixture 构造和 token 创建移出计时区。

- [x] **Step 2: 采集 10 次实现后样本并用 benchstat 比较**

  ```bash
  timeout 5m go test ./internal/user ./cmd/mts-server -run '^$' -bench 'Benchmark(UserGrantRead|AccessGrantsHTTP)' -benchmem -benchtime=500ms -count=10 > /tmp/mts-access-grants-after.txt
  timeout 2m benchstat /tmp/mts-access-grants-before.txt /tmp/mts-access-grants-after.txt
  ```

  不把不同语义 benchmark 的绝对值误作回归；比较同名 legacy 项确认新增代码没有拖慢既有读取，page/HTTP 项记录新路径预算。

- [x] **Step 3: 运行真实 Playwright commercial smoke**

  ```bash
  cd cmd/mts-dashboard && timeout 10m npm run test:e2e
  ```

  记录 10/100/1000 fixture 的聚合请求数均为 1，以及首屏交互时间。

- [x] **Step 4: 对比 bundle**

  记录总 raw/gzip 与主入口 raw/gzip，并与 Task 9 基线 `1,400,585/421,766/333,874/110,983 B` 比较；任一 gzip 增长超过 5% 时停止验收并重新设计。

- [x] **Step 5: 写性能证据文档**

  `docs/benchmarks/access-grants-task10.txt` 写明硬件、Go/Node 版本、命令、请求数、p50 样本摘要、响应体积、benchstat 与 bundle 差异，不能只写结论。

  实现备注：legacy 10/100/1000 用户读取的 `B/op`、`allocs/op` 与请求数均不变，100/1000 用户耗时无统计显著变化，10 用户耗时改善 1.54%。新 HTTP p50 为 `12.01/90.78/170.5 us`，响应为 `1,927/18,668/18,695 B`；Playwright 三档均为一次聚合请求、零次旧 GET，完整套件首屏为 `184.38/178.53/180.62 ms`。bundle 总 gzip 和主入口 gzip 分别增长 `0.2250%/0.2388%`，低于 5% 门禁。完整证据见 `docs/benchmarks/access-grants-task10.txt`。

### Task 5: 全量门禁与整改文档闭环

**Files:**
- Modify: `docs/review/code-review-2026-07-30-0708.md`
- Modify: `docs/superpowers/plans/2026-07-30-full-project-review-remediation.md`
- Modify: `.superpowers/sdd/2026-07-30-full-project-review-remediation/progress.md`
- Modify: `README.md` only if public API listing needs the new read endpoint.

- [x] **Step 1: 格式化与 lint**

  ```bash
  timeout 5m goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .
  timeout 12m golangci-lint run --timeout 12m ./...
  ```

- [x] **Step 2: 全量 Go、Dashboard 与 E2E 门禁**

  运行 `go test -timeout 9m ./...`、受影响包 race、Dashboard unit/typecheck/build/commercial smoke，并逐个执行 `tests/e2e` 可执行目录；每个 E2E 独立 timeout，结束后清理本轮二进制和测试数据。

- [x] **Step 3: 检查权限、diff 和运行态残留**

  所有新文件为 `0600`；`git diff --check` 通过；本轮端口无监听；`.e2e-runtime` 与 Dashboard `test-results` 不存在；不得删除用户原有根目录 `data/`、`test-results/`。

  实现备注：所有新增非用户数据文件均为 `0600`；明确需要执行的 `scripts/storage_benchmark_gate_test.sh` 保持 `0700`。`git diff --check` 通过，15 个 `.task10-e2e-bin` 已逐项清理，Dashboard `.e2e-runtime`/`test-results` 与测试端口监听均不存在；根目录用户已有 `data/`、`test-results/` 未触碰。

- [x] **Step 4: 更新状态与证据**

  将 `REV-20260730-P2-05` 标为已处理，Task 10 全部勾选并写实现备注、性能数字与验证命令；SDD ledger 追加 Task 10 complete。Task 11 保持 pending，根配置/CI 仍需单独批准。

  实现备注：`goimports-reviser` 与 `golangci-lint`（0 issues）通过；全仓 Go test、受影响包 race、Dashboard 697/697 unit、typecheck/build、Playwright 3/3 和 15/15 Go E2E 均通过。完整 Playwright 首次运行发现 share-base 登录依赖前序密码状态，已按 RED/GREEN 修复测试夹具，并验证独立 2/2 与全套 3/3。`REV-20260730-P2-05`、总计划、README、性能证据和 SDD ledger 已同步；Task 11 保持 pending。

## Self-Review

- 规格 6.1 的 8 条 EARS 要求均映射到 Task 1–4 的测试或性能证据。
- 类型名在 internal/runtime/public/server/Dashboard 层保持 `UserGrantBundle`、`UserGrantPage`、`AccessGrantsPageResponse` 一致。
- 计划没有未决占位符；所有实现步骤均指定文件、接口与验证命令。
- 只有 Task 10 additive HTTP GET contract 在授权范围内；未扩展 gRPC、写入 schema、根配置或 CI。
