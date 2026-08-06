# MTS 全项目检视整改实施计划

> **执行前置：** 只有用户明确回复“进入实现阶段”或“implement tasks”后才能修改业务代码。涉及 shared API/schema、根配置、依赖和 CI 的任务仍按仓库规则确认边界。

**Goal:** 闭环主库、`mts-server`、`mts-dashboard` 的全部已证实问题，并用自动化门禁证明无正确性、安全性或性能回归。

**Architecture:** 先恢复可信测试基线并封堵 P0 安全问题，再修失败原子性、并发与协议一致性；随后建立真实性能门禁、升级依赖、修前端问题、补覆盖率，最后分批拆分超大文件。

**Tech Stack:** Go 1.26.5、gRPC、Vue 3、TypeScript 5.9、Vite 7、Playwright、golangci-lint、goimports-reviser、benchstat、govulncheck、npm audit。

## 全局约束

- 所有生产包代码行覆盖率必须 `>=90%`。
- Go 修改后执行 `goimports-reviser` 和 `golangci-lint`。
- 所有错误返回必须显式处理；禁止在 for 循环中 defer。
- 新目录权限 0700，新文件权限 0600。
- 性能 `ns/op` median 不得劣化超过 10%；`B/op`、`allocs/op` 不得统计显著劣化。
- Dashboard 主入口和总 bundle gzip 不得无说明增长超过 5%。
- 不改变存储格式；shared API/schema 变更必须先获得明确确认。
- 每完成一个 Task，更新本文件勾选状态、实现备注及检视报告问题状态。

## 当前状态

- [x] 完成项目边界确认、三部分独立检视和主代理复核。
- [x] 建立检视报告、EARS 规格、性能和 bundle 基线。
- [x] 获得用户“进入实现阶段”授权。

---

### Task 1：恢复可信测试基线

**预算：** 预计 30 分钟，硬超时 60 分钟；单包测试 3 分钟，全量测试 10 分钟。

**Files:**
- Modify: `cmd/mts-server/coverage_closure_test.go`
- Modify: `tests/e2e/mts_server_protocols/main.go`
- Update: `docs/review/code-review-2026-07-30-0708.md`

- [x] 先运行两个定向用例，确认仍分别因 400/404 和短密码失败。
- [x] 将缺失 downsample policy 预期对齐为 404。
- [x] 将协议 E2E 密码改为满足公开策略的非默认密码。
- [x] 运行定向测试和 `go test ./... -count=1 -timeout=10m`。
- [x] 更新 `REV-20260730-P1-01` 状态和证据。

**实现备注（2026-07-30）：** RED 分别复现 404/400 漂移与 8 位密码策略失败；GREEN 定向测试和全仓 `go test ./... -count=1 -timeout=10m` 均通过。仅修改测试 fixture，生产契约未变化。

**验收：** 两处失败消失，未修改生产契约，全量测试恢复为可用基线。

### Task 2：封堵管理员 gRPC 绕过和固定默认管理员

**预算：** 预计 2 小时，硬超时 4 小时；定向测试每次 3 分钟。

**Files:**
- Modify: `cmd/mts-server/grpc.go`
- Modify: `cmd/mts-server/operation_registry.go`
- Modify: `cmd/mts-server/runtime.go`
- Modify: server auth/config tests and docs

- [x] 写失败的 Registry 权限矩阵测试，覆盖匿名、普通用户、管理员。
- [x] 将 Registry `Auth` 元数据接入统一 gRPC 鉴权层，消除 handler 漏加风险。
- [x] 写失败测试证明全新目录不会创建公开固定凭据，禁用/降权管理员不会被重启恢复。
- [x] 设计一次性 bootstrap 方案；若涉及配置/schema，先请求用户确认。
- [x] 运行 HTTP/gRPC auth 测试、协议 E2E、`govulncheck` 和 server race。
- [x] 更新 `REV-20260730-P0-03/P0-04`。

**实现备注（2026-07-30）：** Registry 统一为所有 `authAdmin` gRPC 方法注入鉴权拦截器，并用私有 context 标记避免现有 handler 重复校验 token；自动权限矩阵覆盖全部管理员 RPC。移除生产启动时的 `admin/admin` 创建、提权和启用逻辑，复用已有 `auth.admin_token` + 受保护用户 API 作为显式、可审计的首次管理员创建路径，未增加配置/schema。RED 复现匿名 RPC 成功、全新目录隐式建号和处置状态被覆盖；GREEN 覆盖 HTTP/gRPC auth、协议 E2E、全仓测试、server race 与 lint。`govulncheck ./...` 仍命中既有 `google.golang.org/grpc@v1.81.1` 的 `GO-2026-6061`，留待 Task 8 升级到 `v1.82.1+`。

**验收：** 管理员 RPC 不可匿名调用；不存在公开固定管理员；处置后的账号状态跨重启保持。

### Task 3：修复 Retention 与 Downsample 失败原子性

**预算：** 预计 3 小时，硬超时 5 小时；单包测试 3 分钟，race 5 分钟。

**Files:**
- Modify: `internal/engine/lifecycle.go`
- Modify: `internal/catalog/downsample.go`
- Test: `internal/engine/*_test.go`, `internal/catalog/*_test.go`

- [x] 为 close 成功 + remove 失败、部分删除、重试和重启写故障注入测试。
- [x] 让 Retention 删除具有明确提交点，失败后不暴露已关闭 shard。
- [x] 为 Upsert/Drop/Update 的 write/rename/fsync 失败写内存与磁盘一致性测试。
- [x] 采用“持久化成功后提交内存”或等价事务语义。
- [x] 运行 engine/catalog 定向测试、race、compaction/retention/downsample E2E。
- [x] 更新 `REV-20260730-P1-04/P1-05`。

**实现备注（2026-07-30）：** Retention 在 shard 成功关闭后立即移出活动路由并登记待清理项；目录删除可在下一次 retention 或同 shard 写入前幂等重试，部分删除和重启均可恢复，删除失败不再触发 nil WAL panic。Downsample 使用 O(1) undo journal：写盘失败先恢复单个内存旧值，再恢复调用前磁盘快照，覆盖临时文件 write/fsync、rename 和 rename 后目录 fsync。`go test ./internal/catalog ./internal/engine`、两包 race、相关 E2E/fault 与定向 lint 均通过；仓库暂无 Retention/Downsample 专项 benchmark，正常成功路径未引入全 map 复制，统一性能比较留在 Task 7。

**验收：** 所有注入失败返回错误但运行态可继续使用，重试/重启状态一致，无 panic。

### Task 4：修复热重载和审计并发治理

**预算：** 预计 4 小时，硬超时 6 小时；race 命令 5 分钟。

**Files:**
- Modify: `cmd/mts-server/runtime.go`
- Modify: `cmd/mts-server/config_admin.go`
- Modify: `cmd/mts-server/middleware.go`
- Modify: `cmd/mts-server/grpc.go`, `grpc_stream.go`
- Modify: `cmd/mts-server/audit.go`

- [x] 将现有 semaphore race 探针固化为仓库测试。
- [x] 设计稳定 limiter 更新语义，验证降低上限时旧新请求总量不穿透。
- [x] 对不支持热更新的 `user/log` 字段拒绝或真正应用，响应只列出已提交字段。
- [x] 写 `audit.record + close` 并发、排空、持久化失败和队列满测试。
- [x] 修复审计关闭 panic/丢事件，并统一 HTTP/gRPC 审计矩阵。
- [x] 运行 server 定向测试与 `go test -race ./cmd/mts-server -count=1 -timeout=10m`。
- [x] 更新 `REV-20260730-P1-06/P1-07/P1-09`。

**实现备注（2026-07-30）：** 用稳定的带锁 `requestLimiter` 替代热重载时替换 channel，零限制仍统计全部在途请求，降低上限后旧请求继续占用配额；HTTP、unary gRPC、stream gRPC 共用对应 limiter。reload 检测 `user/log` 变化并原子拒绝，成功响应仅列 `auth/limits/observability`。audit 在同一锁内入队/关闭，关闭时按 context 排空，队列满、关闭后拒绝和持久化失败均计数并返回；Registry 为变更类 gRPC 成功响应统一记录与 HTTP 等价的审计事件。完整 server 测试、race、lint 与协议 E2E 通过。

**验收：** race detector 无报告；限流不穿透；reload 无虚假成功；审计正常关闭且错误可观测。

### Task 5：统一 gRPC TTL、stream、shutdown 和错误语义

**预算：** 预计 3 小时，硬超时 5 小时。

**Files:**
- Modify: `cmd/mts-server/http_auth.go`, `grpc.go`, `grpc_stream.go`
- Modify: `cmd/mts-server/app.go`, `runtime.go`, `http.go`

- [x] 提取 HTTP/gRPC 共享 TTL 规范化函数，覆盖边界和溢出。
- [x] 增加 stream interceptor，统一 timeout、request ID、metrics、access log 和限流。
- [x] 让信号、Serve 异常和启动回滚共享 shutdown timeout。
- [x] 正确映射 canceled/deadline，内部错误对外脱敏。
- [x] 用慢读 stream、阻塞 HTTP、listener 失败和底层路径错误验证。
- [x] 更新 `REV-20260730-P1-08/P2-07`。

**实现备注（2026-07-30）：** HTTP/gRPC 登录统一经 `normalizeAuthTTL` 在 duration 乘法前按 30 天秒数截断，`math.MaxInt64` 不再溢出。新增 stream interceptor，与 unary 统一 request ID/header、配置 timeout、稳定 limiter、metrics、access log 及 canceled/deadline 状态；受控 `RecvMsg` 包装让未发送首帧的慢读 stream 在服务端预算内返回。信号、Serve 异常、HTTP/gRPC listener 启动失败均通过同一 `shutdownWithinTimeout` 入口；真实阻塞 HTTP 测试证明 30ms 配置预算生效。内部 gRPC 错误只返回 `Internal/internal`，真实 StorageSnapshot 路径错误不再泄露绝对路径。`goimports-reviser`、完整 server test、完整 server race（68.063s）、`golangci-lint`（0 issues）和协议 E2E 均通过；Task 5 相关函数覆盖率为 94.4%–100%，server 全包当前为 83.8%，全局 90% 门禁继续由 Task 11 闭环。

**验收：** HTTP/gRPC 等价能力具有一致安全和可观测语义，异常退出在预算内完成。

### Task 6：修复 Dashboard 凭据生命周期和 CSV 导出

**预算：** 预计 2 小时，硬超时 4 小时。

**Files:**
- Modify: `cmd/mts-dashboard/src/api/client.ts`
- Modify: `cmd/mts-dashboard/src/composables/useAuth.ts`
- Modify: `cmd/mts-dashboard/src/utils/*Export.ts`, `utils/csv.ts`

- [x] 写服务 token 跨登出/改密/过期残留的失败测试。
- [x] 清理所有会话关联凭据，并明确连接级凭据产品语义。
- [x] 让 logout 使用统一 API base/client，区分本地退出与远端撤销失败。
- [x] 合并 CSV escape helper 并中和公式前缀及前导控制字符。
- [x] 运行 unit、typecheck、build、非空 API base Playwright 场景和 bundle 对比。
- [x] 更新 `REV-20260730-P1-10/P1-11/P1-12`。

**实现备注（2026-07-30）：** `clearAuth` 统一清除 bearer、用户/角色/过期/强改密标记，以及当前会话的 admin/data service token（内存、localStorage、sessionStorage）；登录新账号、改密、过期、401 与主动退出均复用该清理入口。`apiLogout` 改走统一 API base/client，远端撤销失败仍完成本地退出并显示明确警告；非空 `VITE_API_BASE` Playwright 捕获旧 bearer，验证登出后三类本地凭据为空且旧 bearer 调用 session 返回 401。9 个 CSV 导出统一使用 `escapeCSVCell`，公式前缀与前导控制字符先中和、再按 RFC 4180 quoting；真实 `usersToCSV` mutation 测试覆盖接线。调试 Playwright 时同时修复两处既有基线漂移：E2E 改用每次随机管理 token 显式创建必须改密的管理员（不恢复生产默认账号），并阻止旧的已取消导出覆盖新任务错误状态。最终 unit 690/690、typecheck、默认/非空 API base build、非空 API base Playwright 1/1 均通过。默认 bundle 相对基线：总 raw +0.06%、总 gzip +0.08%、主入口 raw +0.26%、主入口 gzip +0.34%，低于 5% 门禁。

**验收：** 旧会话凭据不跨用户残留，远端 token 确实撤销，CSV 不可执行公式。

### Task 7：建立真实 benchmark gate

**预算：** 预计 2 小时，硬超时 4 小时；每轮 benchmark 5 分钟。

**Files:**
- Modify: `scripts/storage_benchmark_gate.sh`
- Create: `docs/benchmarks/storage-engine-baseline.txt`
- Test: benchmark gate shell tests
- Modify: `docs/ops/nightly-gates.md`

- [x] 测试缺失基线默认失败、显式更新创建 0600 文件、工具缺失失败。
- [x] 选择可执行的阈值判定实现；`benchstat` 用于统计，回归阈值必须转换为非零退出。
- [x] 同机采样 10 次并版本化基线，记录 CPU、Go、governor 和参数。
- [x] 人工注入 >10% 回归，证明 gate 会失败。
- [x] 更新 `REV-20260730-P1-03`。

**实现备注（2026-07-30）：** benchmark gate 现在缺基线、缺 `go`/`timeout`/`benchstat`、四项样本不是各 10 次、Go/CPU/goos/goarch/governor 不一致时均 fail closed。官方 `benchstat` 同时输出人读报告与 CSV；脚本直接从 CSV baseline/current median 计算 `sec/op`，任一 benchmark 劣化 >10% 即非零退出，不受统计不显著的 `~` 隐藏；`B/op`/`allocs/op` 在 current median 增大且 `p < 0.05` 时失败。shell 测试用确定 fake Go 样本证明缺基线、0600 原子更新、缺工具、确定/噪声型 >10% 时间回归和显著 allocation 回归语义。真实基线在 Ryzen 7 7840HS、Go 1.26.5、powersave governor 下各采样 10 次并版本化；正式 `make bench` 再采 10 次通过，`sec/op` median 相对基线为 WriteBatch +0.08%、WriteWideBatch +0.45%、RowIterator -2.31%、ColumnIterator -2.64%，无显著 allocation 劣化。`bash -n`、脚本测试、`git diff --check` 通过；环境未安装 `shellcheck`，未声称该项通过。

**验收：** `make bench` 的成功可作为“当前样本未超过阈值”的有效证据。

### Task 8：升级 Go 与 Dashboard 漏洞依赖

**预算：** 预计 2 小时，硬超时 4 小时；需要用户确认依赖变更。

**Files:**
- Modify: `go.mod`, `go.sum`
- Modify: `cmd/mts-dashboard/package.json`, `package-lock.json`

- [x] 保存升级前 benchmark 和 bundle 基线。
- [x] gRPC 升级到不低于 `v1.82.1` 的最小兼容修复版本。
- [x] Playwright/PostCSS 升级到安全稳定版本。
- [x] 运行 `govulncheck ./...` 和官方 registry `npm audit --audit-level=high`。
- [x] 运行全量 test/race/E2E/build，并用 benchstat/bundle 证明无劣化。
- [x] 更新 `REV-20260730-P0-01/P0-02`。

**实现备注（2026-07-30）：** 用户确认依赖边界后，将 `google.golang.org/grpc` 从 `v1.81.1` 定向升级到最小修复版 `v1.82.1`，并接受其 MVS 所需的 `x/net v0.53.0`、`x/sys v0.43.0`、`x/text v0.36.0` 和 genproto 更新；Dashboard 将 Playwright 链从 `1.55.0` 升到 `1.55.1`，将传递依赖 PostCSS 从 `8.5.15` 升到 `8.5.25`，未升级 Vite/Vue。升级后 `go mod verify`、`govulncheck ./...` 和官方 registry `npm audit --audit-level=high` 均以 0 退出，项目实际可达 Go 漏洞与 npm high/critical 漏洞均为 0。`go test -timeout 9m ./...`、全仓 race、15 个 `tests/e2e` 可执行场景、Dashboard 690/690 unit、typecheck、默认/非空 API base build、Playwright commercial smoke 1/1、`goimports-reviser` 和 `golangci-lint` 均通过；`npm ci` 验证 lockfile 可重现。默认 bundle 升级前后字节一致：总 raw `1,399,534 B`、总 gzip `421,342 B`、主入口 raw `333,588 B`、gzip `110,828 B`。正式 storage benchmark gate 通过；升级前后同机 10 次样本直接比较时，四项 `sec/op` 均无劣化（RowIterator `-2.71%`），`B/op` 与 `allocs/op` 均无显著增加。额外执行的 `make coverage` 仍暴露多个既有包低于 90% 及无语句包误判，作为 Task 11 的未完成项保留，未据此宣称全项目门禁完成。

**验收：** 无实际可达 Go 漏洞和 high/critical npm 漏洞，协议、性能和 bundle 门禁通过。

### Task 9：修复 Dashboard 异步竞态、子路径和无障碍

**预算：** 预计 3 小时，硬超时 5 小时。

**Files:**
- Modify: `QueryPage.vue`, `useQueryWorkbench.ts`
- Modify: route/share URL helpers and 19 consumers
- Modify: `VirtualTable.vue` and icon-button consumers

- [x] 用可控 Promise 写 A/B 旧响应交错测试。
- [x] 采用 AbortController 或序号提交机制，旧请求不得改变新状态。
- [x] 统一通过 router/BASE_URL 生成分享 URL，覆盖 `/` 和 `/mts/`。
- [x] 补 list/grid/row/cell/columnheader 语义和本地化 accessible name。
- [x] 运行 unit、typecheck、build、Playwright route/name/a11y 检查和 bundle 对比。
- [x] 更新 `REV-20260730-P2-03/P2-04/P2-06`。

**实现备注（2026-07-30）：** 新增最新请求提交门卫，`loadDbChildren` 与 `loadMeasurementMeta` 的旧请求不能再提交 measurements/RP/fields/series、错误、全局管理状态或 loading；可控 Promise 覆盖 A/B 交错和清空目标时立即结束 loading。新增 `BASE_URL` 感知的绝对分享 URL helper，19 处页面/组件统一调用，单测覆盖 `/`、`/mts/`、已有前缀和 query/hash，真实 Playwright 分别在根路径与 `/mts/` 服务前缀下核对复制结果。`VirtualTable` 默认输出合法 `list/listitem` 与虚拟位置/总数，并支持 `rowgroup/row`；Users/Audit 补齐 `grid/row/columnheader/gridcell`、行列总数和正确的 `aria-sort` 所属关系，删除用户/写入行/降采样函数与权限面板关闭按钮补中英文 accessible name，危险操作包含目标。最终 `npm test` 694/694、typecheck、默认和 `/mts/` build、commercial smoke 1/1、根路径分享/危险按钮 E2E 2/2、`/mts/` 分享 E2E 1/1 均通过；删除用户名称断言经临时移除属性验证会真实 RED。`goimports-reviser` 与全仓 `golangci-lint`（0 issues）通过。最终 bundle 为总 raw `1,400,585 B`、总 gzip `421,766 B`、主入口 raw `333,874 B`、gzip `110,983 B`，相对 Task 8 基线分别增加 `0.0751%/0.1006%/0.0857%/0.1399%`，低于 5% 门禁。

**验收：** 无旧响应覆盖；子路径链接可用；关键操作可由 role/name 唯一访问。

### Task 10：消除 Access Grants N+1 请求

**预算：** 预计 3 小时，硬超时 5 小时；shared API contract 修改前必须单独确认。

- [x] 记录 10/100/1000 用户下请求数和耗时基线。
- [x] 设计分页/聚合授权 API，明确权限、分页上限和响应体积。
- [x] 获得用户对 shared contract/schema 的明确确认。
- [x] 实现 server + Dashboard + 契约/E2E 测试。
- [x] 对比 server benchmark、API 延迟、请求数和 Dashboard bundle。
- [x] 更新 `REV-20260730-P2-05`。

**实现备注（2026-07-30）：** 新增管理员只读聚合接口 `GET /api/v1/users/access-grants`，默认 `limit=100`、范围 `1..200`，按用户名 cursor 分页；内部在一次 `RLock` 中复制用户与授权，Engine/runtime 只做深复制转换，既有 HTTP/gRPC 和存储格式未改变。Dashboard 改为每页单次聚合请求，翻页成功后才提交 cursor，失败保留当前页，筛选、选择、统计和导出仅作用于当前页。10/100/1000 用户请求数由 `11/101/1001` 降为 `1/1/1`；HTTP p50 `12.01/90.78/170.5 us`，响应 `1,927/18,668/18,695 B`。legacy benchmark 内存、分配和请求数不变，100/1000 用户耗时无统计显著变化；完整 Playwright 三档首屏为 `184.38/178.53/180.62 ms`，总 gzip 和主入口 gzip 增长 `0.2250%/0.2388%`。最终 `go test -timeout 9m ./...`、受影响包 race、Dashboard 697/697 unit、typecheck/build、Playwright 3/3、15 个 Go E2E、goimports-reviser 与 golangci-lint 均通过。全套 Playwright 首次运行还暴露了 share-base 固定使用初始密码的顺序依赖，已用可适应干净/已改密状态的测试登录夹具修复，并分别验证独立 2/2 与全套 3/3。性能原始证据见 `docs/benchmarks/access-grants-task10.txt`。Task 11 的覆盖率与根 CI 门禁仍待单独批准。

**验收：** 单页请求数不随总用户数线性增长，1000 用户场景在明确预算内可交互。

### Task 11：补齐覆盖率和工程门禁

**预算：** 预计 4-8 小时，硬超时 12 小时；按包拆分，每包测试 3 分钟。

**Files:**
- Test: `cmd/mts-server`, `internal/runtime`, `internal/sstable`, `internal/wal`, `internal/catalog`, `internal/engine`
- Modify/Create: Dashboard lint/coverage config, `Makefile`, `scripts/ci_gate.sh`, README

- [x] 按最低覆盖函数生成逐包缺口，优先错误、生命周期、取消和并发路径。
- [x] 每完成一个包即运行定向覆盖率，所有生产包达到 90%。
- [x] Dashboard 增加 lint、coverage、audit gate，并禁止危险 DOM sink。
- [x] 将 Go/Dashboard 门禁接入统一 CI；根配置/CI 修改前请求确认。
- [x] 更新 `REV-20260730-P1-02/P2-02`。

**实现备注（2026-07-30）：** 新增动态逐包 coverage gate 和无语句包 contract test，全部生产 Go 包达到 `>=90.0%`，其中 `cmd/mts-server` 精确为 `3249/3609 = 90.0249%`。Dashboard 新增 lint、lines/functions/branches=`90/90/70` 覆盖率、官方 audit 与危险 DOM sink 门禁，实测 697/697、`96.65%/95.01%/73.35%`、0 漏洞、0 sink；根 `Makefile` 和 `scripts/ci_gate.sh` 复用统一入口。最终 bundle 相对 Task 10 基线总 raw `-639 B`、总 gzip `0 B`、主入口 raw `-1,419 B`、gzip `0 B`。

**验收：** `make coverage` 逐包通过，Dashboard 门禁可量化且不增加生产 bundle。

### Task 12：分批拆分超大文件

**预算：** 每批预计 2-4 小时，单批硬超时 6 小时；不得一次性全量重构。

- [x] 每次只选择一个职责边界，先确认现有契约测试。
- [x] 仅移动代码并保持符号/API 不变，不混入功能变更。
- [x] 每批运行定向测试、全量 test/lint、race/E2E 和性能/bundle 对比。
- [x] 文件仍超过 300 行时记录下一批边界，不强行合并大改。
- [x] 更新 `REV-20260730-P2-01`。

**实现备注（2026-07-30）：** 首批仅拆 `cmd/mts-server/runtime.go` 的响应装配职责，将管理员响应移入 `runtime_admin_payload.go`（242 行）、数据面响应移入 `runtime_data_payload.go`（274 行），原文件从 992 行降至 480 行；方法签名、函数体、API/schema 未变化，新文件权限为 0600。定向 test/coverage、server race（74.650s）、lint、15/15 Go E2E、根 CI、正式 benchmark 与 Dashboard bundle 均通过。剩余边界按批次继续：runtime 生命周期/管理互斥、`grpc.go` handler、`operation_registry.go` catalog，以及 Dashboard 页面/i18n/E2E；`REV-20260730-P2-01` 保留为“建议观察”。

**验收：** 每批行为不变、可独立回退且无性能劣化。

### Task 13：最终统一门禁与收尾

**预算：** 预计 4-10 小时，硬超时 16 小时；E2E 每个目录单独限时，禁止无边界总命令。

- [x] 运行 `goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .`。
- [x] 运行 `golangci-lint run ./...`、全量测试、逐包覆盖率和核心/server race。
- [x] 逐个执行 `tests/e2e`、fault、scale smoke 和 Playwright commercial smoke。
- [x] 运行 `govulncheck`、npm official audit、benchmark gate 和 bundle 对比。
- [x] 清理本轮临时构建产物，不删除用户现有 `data/`、`test-results/`。
- [x] 更新报告中所有问题状态，无法关闭的项必须有复现证据、风险和后续责任边界。

**实现备注（2026-07-30）：** 最终递归 `goimports-reviser` 和修正后根 `scripts/ci_gate.sh` 均以 0 退出；全量 Go test、全仓 lint（0 issues）、核心/server race、逐包 coverage 和 Dashboard gate 均通过，全部生产 Go 包 `>=90%`，`cmd/mts-server` 精确为 `3249/3609 = 90.0249%`。Dashboard 为 697/697，lines `96.65%`、branches `73.35%`、functions `95.01%`，官方 npm audit 为 0 漏洞且危险 DOM sink 为 0；Playwright 串行 3/3、15/15 Go E2E 及 fault/scale smoke 全部通过。`govulncheck ./...` 为 0 个可达漏洞；独占运行 `make bench` 通过，四项时间变化约为 `0%/0%/-11.43%/0%`，RowIterator 的 `B/op`/`allocs/op` 为 `-19.29%/-19.40%`。首次 benchmark 与 Dashboard 重负载并行时因 CPU/磁盘争用出现 `+16.14%/+13.04%`，已记录并改为独占重跑，不作为产品回归。最终 bundle 为总 raw `1,403,096 B`、总 gzip `422,715 B`、主入口 raw `334,442 B`、gzip `111,248 B`，相对 Task 10 为 `-639 B/0 B/-1,419 B/0 B`。统一 CI artifact scan 和限定路径清理审计均无残留，用户根 `data/`、`test-results/` 保留。报告中功能性问题均为“已处理”；`REV-20260730-P2-01` 因仍有历史超大文件待后续分批拆分而保留“建议观察”。

**验收：** 全部硬门禁通过；报告、规格、计划和 README 与实现一致；输出最终 merge plan。
