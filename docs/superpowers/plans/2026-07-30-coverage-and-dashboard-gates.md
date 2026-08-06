# MTS 覆盖率与 Dashboard 工程门禁实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让全部有生产语句的 Go 包达到逐包行覆盖率 `>=90%`，并为 Dashboard 建立可量化、可阻塞、不会进入生产 bundle 的 lint、测试覆盖率、依赖审计和危险 DOM sink 门禁。

**Architecture:** Go 侧只补行为与错误路径测试，不为覆盖率增加生产 test hook；将重复的逐包覆盖率逻辑抽成一个可独立测试的 shell gate，Makefile 与统一 CI 共用。Dashboard 保留 Node 原生测试运行器，利用 Node 22 原生覆盖率阈值，使用 ESLint flat config 检查 Vue/TypeScript，并增加只扫描源码的危险 DOM sink 工具；这些依赖和工具均为开发期资产，不进入 Vite 生产依赖图。

**Tech Stack:** Go 1.26.5、Go coverage、bash、Node 22.22.2 原生 test coverage、ESLint 10、typescript-eslint 8、eslint-plugin-vue 10、eslint-plugin-no-unsanitized 4、Vue 3、TypeScript 5.9。

## Global Constraints

- 当前连续工作区为 `/root/projects/v2/mts`，保留 Task 1-10 的全部未提交修改。
- 用户已批准修改根 `Makefile`、`scripts/ci_gate.sh`、Dashboard 依赖/lockfile和 lint/coverage 配置。
- 不修改存储格式、shared API/schema 和生产行为；测试暴露真实缺陷时另行按 RED/GREEN 修复。
- 所有有生产语句的 Go 包覆盖率必须 `>=90.0%`；无生产语句包明确输出 skipped，不能伪装为达标。
- Dashboard 覆盖率门禁排除 `*.test.ts`，最低 lines `90%`、functions `90%`、branches `70%`。
- Dashboard lint 覆盖 `src/**/*.ts`、`src/**/*.vue`、`e2e/**/*.ts`、`scripts/**/*.mjs`；危险 DOM sink 扫描覆盖 `src`。
- Dashboard 高危/严重依赖漏洞必须为 0，固定使用 npm 官方 registry 执行审计。
- 新目录权限 `0700`，新普通文件 `0600`，仅可执行脚本使用 `0700`。
- Go 文件修改后运行 `goimports-reviser` 和 `golangci-lint`；禁止在 for 循环中 defer；所有错误显式处理。
- 不删除用户已有根目录 `data/`、`test-results/`，不执行 commit/push。
- 单包 Go 测试硬超时 3 分钟；全量 Go 测试 10 分钟；lint 12 分钟；CI 30 分钟；每个 E2E 独立限时。
- 生产性能和 Dashboard bundle 不得劣化；纯测试/工具改动预期 bundle byte-for-byte 不变，最终仍需实测。

## Coverage Baseline

| Package | Covered / Total | Current | Statements Needed For 90% |
| --- | ---: | ---: | ---: |
| `cmd/mts-server` | 3024 / 3609 | 83.7905% | 225 |
| `internal/runtime` | 34 / 70 | 48.5714% | 29 |
| `internal/catalog` | 1106 / 1248 | 88.6218% | 18 |
| `internal/engine` | 3895 / 4349 | 89.5608% | 20 |
| `internal/wal` | 1032 / 1169 | 88.2806% | 21 |
| `internal/sstable` | 3812 / 4411 | 86.4203% | 158 |

---

### Task 1: 统一且可测试的 Go 覆盖率 Gate

**预算：** 预计 30 分钟，硬超时 60 分钟。

**Files:**
- Create: `scripts/coverage_gate.sh`
- Create: `scripts/coverage_gate_test.sh`
- Modify: `Makefile`
- Modify: `scripts/ci_gate.sh`

**Interfaces:**
- Consumes: `MTS_COVERAGE_MIN`、`MTS_COVERAGE_PACKAGE_TIMEOUT`、可选 package arguments。
- Produces: `bash scripts/coverage_gate.sh [packages...]`；无参数时动态枚举生产包，排除 `/tests/` 与 `/internal/bench`。

- [x] **Step 1: 写失败的 shell contract test**

  测试在 `mktemp -d` 的独立临时 Go module 中创建：一个无语句包、一个 100% 包、一个低覆盖包。断言无语句包输出 `coverage skipped` 且成功，100% 包成功，低覆盖包输出 `coverage below threshold` 且失败；退出 trap 清理临时目录。

- [x] **Step 2: 验证 RED**

  ```bash
  timeout 60s bash scripts/coverage_gate_test.sh
  ```

  Expected: FAIL，因为 `scripts/coverage_gate.sh` 尚不存在。

- [x] **Step 3: 实现最小 gate**

  gate 为每个包创建 `0600` profile；测试失败立即返回非零；profile 只有 `mode:` 行时输出 skipped；其他包用 `go tool cover -func` 提取 total 并与阈值比较。临时目录用 `mktemp -d`、`chmod 0700` 和 `trap` 清理。

- [x] **Step 4: 验证 GREEN 并接入现有入口**

  ```bash
  timeout 60s bash scripts/coverage_gate_test.sh
  timeout 180s MTS_COVERAGE_MIN=0 bash scripts/coverage_gate.sh ./internal/archtest ./internal/runtime
  ```

  将 `make coverage` 和 `scripts/ci_gate.sh` 的重复实现替换为对该脚本的调用。

**验收：** 无语句包不会误报 0%，Makefile 和 CI 不再维护两份包列表/计算逻辑。

**实现备注（2026-07-30）：** `coverage_gate_test.sh` 首次按预期因 gate 缺失退出 1；新增 gate 后 contract test 通过。真实探针中 `internal/archtest` 输出 `coverage skipped: ... reason=no-statements`，`internal/runtime` 可正常计算 48.6%。Makefile 与 `scripts/ci_gate.sh` 已复用同一脚本，消除静态包列表和覆盖率计算重复。

### Task 2: 补齐 Runtime、Catalog 与 Engine 覆盖率

**预算：** 预计 45 分钟，硬超时 90 分钟；每包测试 3 分钟。

**Files:**
- Create: `internal/runtime/user_manager_coverage_test.go`
- Create: `internal/catalog/management_coverage_test.go`
- Create: `internal/engine/metadata_coverage_test.go`

**Interfaces:** 只调用既有公开/包内接口，不修改生产签名。

- [x] **Step 1: Runtime RED/GREEN**

  新测试通过真实本地 user manager 覆盖 `UpdateUser`、`ListUsers`、`DeleteUser`、`RevokeDatabasePermission`、`ListDatabasePermissions`、`ListUserGrantPage`、`ChangePassword`、`RevokeToken`，同时验证返回用户/授权为深复制且撤销后认证失败；通过 runtime engine 验证 `Storage()` 返回实际 storage engine。

  ```bash
  timeout 180s go test ./internal/runtime -count=1 -coverprofile=/tmp/mts-runtime-task11.cover -timeout 3m
  go tool cover -func=/tmp/mts-runtime-task11.cover | tail -1
  ```

- [x] **Step 2: Catalog RED/GREEN**

  新测试创建顺序相反的两个 database，断言 `ListDatabases()` 返回稳定排序且返回切片修改不影响后续结果；覆盖空 catalog。

  ```bash
  timeout 180s go test ./internal/catalog -count=1 -coverprofile=/tmp/mts-catalog-task11.cover -timeout 3m
  ```

- [x] **Step 3: Engine RED/GREEN**

  新测试分别覆盖 `Engine.ListDatabases` facade 与 local metadata store `ListDatabases`，断言排序、context 取消/关闭后的错误传播保持既有契约。

  ```bash
  timeout 180s go test ./internal/engine -count=1 -coverprofile=/tmp/mts-engine-task11.cover -timeout 3m
  ```

**验收：** 三个包分别 `>=90.0%`，测试覆盖真实行为而非仅调用函数。

**实现备注（2026-07-30）：** 新增真实 local user manager 生命周期测试，覆盖用户更新/列表/删除、授权分页与撤销、改密、token 撤销和深复制，并覆盖 runtime storage facade 与非法路径错误，`internal/runtime` 达到 `90.0%`。Catalog 新增 database/policy/series 排序、独立切片、无 WAL checkpoint、字符串列表和 snapshot I/O 错误测试，达到 `90.1%`。Engine 新增 local/facade `ListDatabases` 和不等长列窗口切分边界测试，达到 `90.1%`。三包定向测试均通过，未修改生产代码。

### Task 3: 补齐 WAL 与 SSTable 覆盖率

**预算：** 预计 90 分钟，硬超时 3 小时；每包测试 3 分钟。

**Files:**
- Create: `internal/wal/coverage_closure_test.go`
- Create: `internal/sstable/coverage_closure_task11_test.go`

**Interfaces:** 复用包内编码、part pack 与 reader helper；不增加生产 test hook。

- [x] **Step 1: WAL RED/GREEN**

  覆盖 legacy point 的全部 field 类型和 unsupported type 错误以执行 `appendField`；用真实 WAL 覆盖 `AppendTyped` 成功/关闭后错误、`ApproxMemoryBytes` 空与追加后增长，并验证 replay 数据一致。

  ```bash
  timeout 180s go test ./internal/wal -count=1 -coverprofile=/tmp/mts-wal-task11.cover -timeout 3m
  ```

- [x] **Step 2: SSTable RED/GREEN 第一批**

  针对 windowed float/int 解码分别覆盖 float-int codec、delta int、delta-RLE、截断输入和非法 codec；断言窗口边界、值类型和错误。

- [x] **Step 3: SSTable RED/GREEN 第二批**

  用真实 part 写入并调用 `NewSeriesBatchReader`，覆盖多 batch、EOF、Close；覆盖 pack `Size/ReadAt` 边界、logical component helper 的读取/覆盖/删除，以及 legacy open error 关闭路径可达到的错误组合。

  ```bash
  timeout 180s go test ./internal/sstable -count=1 -coverprofile=/tmp/mts-sstable-task11.cover -timeout 3m
  ```

**验收：** WAL 与 SSTable 分别 `>=90.0%`；测试验证编码往返、边界与错误路径，无生产性能路径改动。

**实现备注（2026-07-30）：** WAL 新增 typed batch、字段类型、部分行、非法输入、内存估算、pending flush 与重启 replay 测试，达到 `91.2%`。SSTable 分批覆盖 windowed delta/RLE/const-step 解码、pack section reader、逻辑组件读写、legacy close 错误聚合、损坏 pack/index/value 传播、组件尺寸回退及流式 writer 错误路径；最终精确覆盖 `3970/4411 = 90.0023%`。定向测试均通过，新增文件均小于 300 行，未修改生产代码或存储格式。

### Task 4: 补齐 mts-server 覆盖率

**预算：** 预计 90 分钟，硬超时 3 小时；定向测试 5 分钟。

**Files:**
- Create: `cmd/mts-server/grpc_coverage_task11_test.go`
- Create: `cmd/mts-server/http_coverage_task11_test.go`
- Create: `cmd/mts-server/storage_coverage_task11_test.go`

**Interfaces:** 复用 `openTestRuntime`、`openBufconnClient`、`invokeOK`、HTTP helper 和真实临时数据目录。

- [x] **Step 1: gRPC wrapper contract RED/GREEN**

  通过 bufconn 调用当前 0%/低覆盖 wrapper：typed batch write、delete、data limits/contract、session、用户修改/删除/授权撤销、database/retention/meta list、config/spec/error code、maintenance/ops/health/memory/compaction/validate/export、downsample list/disable/drop/reset、snapshot list/delete。每个调用断言响应字段或明确 gRPC code，不能只断言无 panic。

- [x] **Step 2: HTTP/batch/error RED/GREEN**

  覆盖 batch user disabled、batch downsample policy、stream batch decode/validate、typed write/delete、snapshot/audit 列表与删除、缺省 JSON body、primary retention；断言 HTTP status、error code 和部分成功语义。

- [x] **Step 3: storage/helper RED/GREEN**

  覆盖 latest snapshot path、restore fatal counting、middleware `Unwrap`、字段日志转换及可安全调用的启动辅助错误路径。

  ```bash
  timeout 300s go test ./cmd/mts-server -count=1 -coverprofile=/tmp/mts-server-task11.cover -timeout 5m
  go tool cover -func=/tmp/mts-server-task11.cover | tail -1
  ```

**验收：** `cmd/mts-server >=90.0%`，新增测试文件均不超过 300 行；若需更多内容，按协议职责拆文件。

**实现备注（2026-07-30）：** 通过真实 bufconn 和 HTTP runtime 覆盖 typed write、delete、session、admin/user/database/retention/meta/config/downsample/snapshot wrapper 的成功、鉴权、取消与输入错误路径，并补齐 batch stream、snapshot/helper 和 Pprof 默认关闭合同。最终精确覆盖 `3249/3609 = 90.0249%`，新增五个测试文件均小于 300 行，未修改生产行为。

### Task 5: Dashboard Lint、Coverage、Audit 与 DOM Sink Gate

**预算：** 预计 60 分钟，硬超时 2 小时。

**Files:**
- Create: `cmd/mts-dashboard/eslint.config.js`
- Create: `cmd/mts-dashboard/scripts/check-dangerous-dom.mjs`
- Create: `cmd/mts-dashboard/scripts/check-dangerous-dom.test.mjs`
- Modify: `cmd/mts-dashboard/package.json`
- Modify: `cmd/mts-dashboard/package-lock.json`

**Interfaces:**
- Produces npm scripts `lint`、`test:coverage`、`test:security`、`audit:ci`、`gate`。

- [x] **Step 1: DOM gate RED**

  测试输入覆盖 `v-html`、`innerHTML`、`outerHTML`、`insertAdjacentHTML`、`document.write`、`eval`、`new Function`，断言逐项报告；普通 `textContent` 和 Vue interpolation 不报告。先运行测试确认因模块缺失失败。

- [x] **Step 2: DOM gate GREEN**

  实现纯函数 `findDangerousDOMUsages(source, file)` 和递归 CLI；CLI 只扫描 `src` 中 `.ts/.js/.vue`，按路径排序输出 `file:line: sink`，发现任一项退出 1。

- [x] **Step 3: ESLint flat config**

  使用 `@eslint/js` recommended、`typescript-eslint` recommended、`eslint-plugin-vue` flat essential、`eslint-plugin-no-unsanitized` recommended；关闭与现有 `vue-tsc` 重复且会误报 Vue 宏的规则，只做 correctness/security，不引入格式化 churn。

- [x] **Step 4: 覆盖率和官方审计**

  `test:coverage` 使用 Node 原生 coverage，排除 `*.test.ts`，阈值 lines/functions/branches 为 `90/90/70`；`audit:ci` 固定 `--registry=https://registry.npmjs.org --audit-level=high`。

- [x] **Step 5: 验证 Dashboard gate**

  ```bash
  cd cmd/mts-dashboard
  timeout 5m npm ci
  timeout 5m npm run lint
  timeout 5m npm run test:coverage
  timeout 60s npm run test:security
  timeout 5m npm run audit:ci
  timeout 10m npm run gate
  ```

**验收：** 门禁可阻塞真实违规；现有源码通过；生产 dependencies 不增加，Vite bundle 不包含 lint/coverage 工具。

**实现备注（2026-07-30）：** 新增 ESLint 10 flat config、Node 22 原生 coverage 门禁、官方 registry 高危审计和独立危险 DOM sink 扫描器。扫描器合同覆盖七类 sink、安全文本写入及 CLI 非零退出；统一 `npm run gate` 通过。实测 697 个单测，覆盖率 `lines 96.65% / branches 73.35% / functions 95.01%`，审计为 0 漏洞，源码 sink 扫描为 0 命中；新增依赖均位于 `devDependencies`。

### Task 6: 根 CI 集成、文档与全量验证

**预算：** 预计 2-4 小时，硬超时 8 小时；每个 E2E 独立限时。

**Files:**
- Modify: `Makefile`
- Modify: `scripts/ci_gate.sh`
- Modify: `README.md`
- Modify: `docs/ops/nightly-gates.md`
- Modify: `docs/review/code-review-2026-07-30-0708.md`
- Modify: `docs/superpowers/plans/2026-07-30-full-project-review-remediation.md`
- Modify: `.superpowers/sdd/2026-07-30-full-project-review-remediation/progress.md`
- Modify: this plan

- [x] **Step 1: 接入统一入口**

  新增 `make dashboard-gate`；`scripts/ci_gate.sh` 在 Go coverage 后执行 Dashboard `npm ci` 和 `npm run gate`，所有阶段带 timeout。

- [x] **Step 2: 格式化、lint、逐包覆盖率与 race**

  ```bash
  timeout 5m goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .
  timeout 12m golangci-lint run --timeout 12m ./...
  timeout 10m go test ./... -count=1 -timeout 10m
  timeout 10m make coverage
  timeout 15m go test -race -count=1 -timeout 15m ./cmd/mts-server ./internal/runtime ./internal/catalog ./internal/engine ./internal/wal ./internal/sstable
  ```

- [x] **Step 3: Dashboard 与 bundle**

  运行 `npm run gate`、typecheck、build、Playwright；记录总 raw/gzip 与主入口 raw/gzip，对比 Task 10 基线，总 gzip与主入口 gzip不得增长超过 5%，纯工具变更预期为 0。

- [x] **Step 4: Go E2E、漏洞与性能**

  逐个 build/run `tests/e2e` 可执行目录并清理二进制；运行 `govulncheck ./...`、`make bench`。测试-only 变更不应改变 storage benchmark，仍需保留正式 gate 证据。

- [x] **Step 5: 文档闭环**

  将 `REV-20260730-P1-02/P2-02` 标为已处理；总计划 Task 11 全部勾选并写实测覆盖率、Dashboard 阈值、依赖版本、验证命令与 bundle/performance 结果；SDD ledger 追加 Task 11 complete。

- [x] **Step 6: 清理和 diff 检查**

  删除本轮 `/tmp` profile、E2E 临时二进制和 Dashboard 临时输出；保留用户 `data/`、`test-results/`；检查新增权限、`git diff --check` 和端口监听。

**验收：** Go/Dashboard 的统一工程门禁均可复现通过，报告和计划与证据一致，无生产性能或 bundle 劣化。

**实现备注（2026-07-30）：** 递归 `goimports-reviser`、全仓 lint/test、逐包 coverage、核心与 server race 均通过；全部生产 Go 包 `>=90%`。Dashboard gate 为 697/697、lines `96.65%`、branches `73.35%`、functions `95.01%`、0 漏洞、0 危险 sink，typecheck/build 与 Playwright 3/3 通过。15/15 Go E2E、fault/scale smoke、`govulncheck ./...`、官方 npm audit 和正式 `make bench` 通过；benchmark 四项 `sec/op` 相对基线约为 `0%/0%/-11.43%/0%`，无显著 allocation 回归。bundle 为总 raw `1,403,096 B`、总 gzip `422,715 B`、主入口 raw `334,442 B`、gzip `111,248 B`，相对 Task 10 基线不劣化。统一 CI 最终 artifact scan 通过；本轮临时二进制、profile 与 Dashboard 测试输出已清理，用户根 `data/`、`test-results/` 保留。

## Self-Review

- Task 11 四项要求分别映射到 Task 1-6，没有遗漏逐包覆盖率、Dashboard lint/coverage/audit/DOM sink 和统一 CI。
- Go gate 的“无语句包”与“低覆盖包”行为均有 RED/GREEN contract test，不再依赖易漂移的静态包列表。
- Dashboard 覆盖率明确限定为当前 Node 单元测试实际加载的 TypeScript 生产模块，不虚构 `.vue` 页面覆盖；页面行为继续由 Playwright 覆盖。
- 所有生产行为改动均被排除；若测试发现真实缺陷，必须先更新本计划和评审报告再修复。
- 没有占位步骤；文件、接口、阈值、命令、超时和验收标准均已明确。
