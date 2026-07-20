# MTS Dashboard 全量前端检视报告（2026-07-21）

- **时间**: 2026-07-21 00:02（Asia/Chongqing）
- **范围**: `cmd/mts-dashboard` + 与 `cmd/mts-server` HTTP 契约对齐
- **基线提交**: `0507562`（P136 签核引导合入后）
- **性质**: 只读检视 + EARS 任务清单；**不宣称可商用目标完成**

---

## 1. 结论

Dashboard 在 P124–P136 后已具备可商用后台的主体骨架：鉴权/改密、查询写入、库与用户、运维与降采样、审计、存储与就绪中心、命令面板、大量列表虚拟滚动、签核引导。  
历史 **P0 契约错误（删除 JSON / 降采样 enable-disable）已修复**；`/readyz`、metrics、admin health、maintenance errors、api-spec、authz 预检等此前缺口多数已接入。

当前剩余短板以 **列表密度与边界体验**、**非 admin 元数据面**、**查询高级语义暴露**、以及 **部署侧人工项** 为主，未见新的阻断级契约错误。

| 维度 | 评级 | 说明 |
|---|---|---|
| 功能闭环 | 良 | 读写/运维/权限/存储/就绪主路径可用 |
| 前后端契约 | 良 | 历史 P0 已对齐；无新发现阻断错位 |
| 列表性能 | 良 | 主列表已 VT；ApiSpec/Overview/Readiness doctor 仍有原生表 |
| 非 admin 体验 | 良 | 库列表/RP 均有 data 面只读路径（P140/P145） |
| 查询表达力 | 中高 | aggregates/window/group/order + predicates DSL；expr 树 UI 仍非目标 |
| 可商用部署 | 中 | 边缘证书/cron/异地备份仍人工，不计入评分 |

---

## 2. 近期已合入（本会话）

| Commit | 项 | 内容 |
|---|---|---|
| `33e996e` | P133 | Query 行结果/列摘要虚拟滚动 + testid |
| `7e5acec` | P134 | Query 历史 VT + 筛选 + 上限 200 |
| `241fdaf` | P135 | 命令面板密度/sticky/结果计数 |
| `0507562` | P136 | Readiness 签核引导 + 空白示例 |

---

## 3. 历史问题状态复检

| ID | 描述 | 状态 |
|---|---|---|
| FE-ALIGN-P0-01 | 删除 JSON snake_case vs Go 字段 | **已修复**（`DeleteRequest` 已有 `json` tag；前端 `start_time` 等正确） |
| FE-ALIGN-P0-02 | 降采样 pause/resume 错名 | **已修复**（UI 使用 enable/disable） |
| ready/metrics/health/maint/api-spec/authz | 未接入 | **已修复**（readyz、Metrics 页、Overview/Ops、ApiSpec、authz check） |
| 大表 DOM | 多页原生列表 | **大部分已修复**（P124–P134）；见剩余项 |

---

## 4. 前后端 API 对齐矩阵（摘要）

### 4.1 前端已用且后端存在

- Auth：`login/logout/password`、`authz/database/check`
- Data：`write` / `write/typed` / `write/points-typed`、`query/rows|columns|explain|stream`、`delete`、`databases/*/measurements|fields|series`
- Admin：databases/RP、flush/compact/retention、downsample policies/statuses/actions、storage*、audit、doctor、version、health、maintenance/errors、stats*、config*、api-spec、error-codes
- Users：CRUD + database-permissions

### 4.2 后端有、前端弱/未暴露

| 能力 | 影响 | 优先级 |
|---|---|---|
| 独立 `GET /api/v1/data/query/stats` | Query 页引擎快照入口已接（P143） | 已完成 |
| series 列表 query 过滤 UI | fields/series 选择器 + 客户端筛选（P142/P146；无服务端分页） | 部分完成 |
| Query `predicates` / `expr` | predicates DSL 已暴露；expr 树 UI 仍非目标 | 部分完成 |
| 非 admin 的 RP 列表 API | 仅 admin 路径；data 用户 RP 手填 | P1 |
| `GET /api/v1/admin/config`（非 effective） | 已有 effective/schema/validate/reload | P3 |
| pprof `/debug/pprof/*` | 故意不进 UI | — |

### 4.3 部署侧（不计 readiness 评分）

1. 生产边缘证书 / HSTS 人工验收  
2. 目标环境 cron/systemd 实装与演练  
3. 跨主机异地备份 + 失败告警真实跑通  

---

## 5. 剩余问题（EARS 来源）

### P1

#### FE-FULL-P1-01 ApiSpec 端点表未虚拟化
- **位置**: `ApiSpecPage.vue` 原生 `<tr v-for>`
- **风险**: 全量 registry 端点增多时主线程与滚动变差
- **建议**: 每 namespace 或扁平列表接 `VirtualTable` + 空态

#### FE-FULL-P1-02 Overview 健康/Doctor/维护错误原生列表
- **位置**: `OverviewPage.vue` healthChecks / doctorChecks / maintenanceErrors
- **风险**: doctor/maint 条目增多时 DOM 膨胀；与 Ops 页体验不一致
- **建议**: VirtualTable 或上限 + 虚拟滚动；统一 EmptyState

#### FE-FULL-P1-03 Readiness Doctor checks 原生表
- **位置**: `ReadinessPage.vue` doctor.checks
- **建议**: VirtualTable + testid

#### FE-FULL-P1-04 非 admin RP/库 元数据 — **已修复（P140 RP + P145 库列表）**
- **位置**: `api/meta.ts` `listRetentionPolicies` → `/api/v1/admin/databases/.../retention-policies`
- **风险**: 有 data 权限用户查询/写入页 RP 下拉为空，只能手填
- **建议**: 服务端提供 data 面只读 RP 列表，或明确 UX 手填提示（已有部分）

### P2

#### FE-FULL-P2-01 Query predicates/expr 未暴露
- **位置**: `queryFormBuild` / QueryPage
- **影响**: 无法在 UI 构造字段谓词/表达式；高级过滤需 API 直调

#### FE-FULL-P2-02 Write 表单行无限增长
- **位置**: `WritePage.vue` `addRow` 无上限
- **建议**: 行数上限（如 50）+ 提示走 Line/TypedBatch

#### FE-FULL-P2-03 VITE_BASE 子路径 vs 服务端根托管
- **位置**: `vite.config.ts` vs `mts-server` dashboard handler 固定根
- **影响**: 子路径构建前端时，嵌入托管可能 404

#### FE-FULL-P2-04 series meta 未进查询表单
- **建议**: measurement 选定后可选 series tags 预填

#### FE-FULL-P2-05 部署侧签核仍 open
- 边缘证书、cron/systemd、异地备份告警（**不计分**）

### P3

#### FE-FULL-P3-01 独立 query/stats 端点入口 — **已修复（P143）**
#### FE-FULL-P3-02 Account 着陆选项等小列表 EmptyState 一致性 — **已修复（P144）**

---

## 6. 虚拟列表覆盖盘点

| 页面/组件 | VirtualTable | 备注 |
|---|---|---|
| AccessMatrix / Grants / Users / Audit | ✅ | |
| Databases / Downsample / Storage / Config | ✅ | |
| Metrics / Ops logs / Notify / Query rows-cols-history | ✅ | |
| CommandPalette | ❌（有意） | 规模可控 + 密度优化 P135 |
| ApiSpec endpoints | ❌ | P1 |
| Overview doctor/health/maint | ❌ | P1 |
| Readiness doctor | ❌ | P1 |
| Write form rows | ❌ | 表单编辑器，宜上限而非 VT |

---

## 7. 验证建议（合入门禁）

```bash
cd cmd/mts-dashboard && npm test && npm run build && npm run test:e2e
make e2e
timeout 180s env GOSUMDB=sum.golang.org go test -count=1 -timeout 120s ./...
```

---

## 8. 推荐实施顺序

1. P137：ApiSpec + Overview + Readiness doctor 虚拟滚动与空态  
2. P138：Write 表单行上限 + 非 admin RP UX 强化  
3. P139：Query predicates（可选分阶段）  
4. 部署侧仍人工，不进评分

## 处理状态（2026-07-21）
| ID | 状态 |
|---|---|
| FE-FULL-P1-01 ApiSpec VT | **已修复**（P137） |
| FE-FULL-P1-02 Overview lists VT | **已修复**（P137） |
| FE-FULL-P1-03 Readiness doctor VT | **已修复**（P137） |
| FE-FULL-P1-04 非 admin RP/库 | **已修复**（P140 RP + P145 库列表 data 面） |
| FE-FULL-P2-01 predicates | **已修复**（P139 DSL；expr 树仍非目标） |
| FE-FULL-P2-02 Write 行上限 | **已修复**（P138 上限 50） |
| FE-FULL-P2-03 VITE_BASE | **已对齐说明**（P141；代码本已支持 dashboard_base） |
| FE-FULL-P3-01 query/stats | **已修复**（P143） |
| FE-FULL-P3-02 Account landing | **已修复**（P144） |
| FE series filter | **部分完成**（P146 客户端筛选；无服务端分页） |
| 部署侧三项 | open 不计分 |
