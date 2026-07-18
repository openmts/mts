# MTS 兼容性与稳定性边界

本文档约定单机 MTS 在 public API、磁盘格式与 experimental 功能上的兼容承诺。

## 1. Public API 稳定性

- Go module 根包 `github.com/openmts/mts` 为唯一 public 嵌入式 API。
- `internal/*` 不承诺兼容，可在任意版本破坏。
- 稳定错误 sentinel（可用 `errors.Is`）包括：
  - `ErrInvalidOptions` / `ErrNotFound` / `ErrUnsupported` / `ErrInvalidPrecision`
  - `ErrCardinalityLimit` / `ErrStorageMemoryLimitExceeded` / `ErrReadBudgetExceeded`
  - `ErrResourceExhausted` / `ErrEngineBusy`
  - 用户管理相关 `ErrUser*` / `ErrPermissionDenied` / `ErrInvalidCredentials`
- 新增 public 符号遵循语义化版本：
  - **patch**：缺陷修复、可观测增强、文档，不破坏现有调用
  - **minor**：向后兼容新增 API / 配置项
  - **major**：删除/重命名 public API，或改变磁盘可读语义

## 2. 磁盘格式

- SSTable / WAL / Manifest 使用 magic + version 识别。
- **向前兼容（读旧写新）**：同 major 磁盘代际内，新版本应能打开并查询旧数据目录。
- **向后兼容（读新写旧）**：不承诺。
- 破坏性格式变更必须：
  1. 提升 format version
  2. 在 CHANGELOG 标注
  3. 提供迁移说明或工具路径

### 最低可读策略

| 组件 | 策略 |
| --- | --- |
| WAL | 打开时按 segment version 解析；未知 version 失败并返回可诊断错误 |
| SSTable Part | metadata version 校验；不支持则拒绝加载该 part |
| Manifest | 版本不匹配时 fail-fast，避免静默丢数据 |
| Catalog/Metadata | 本地文件 schema 演进需可读旧字段默认值 |


### SSTable metadata component sizes（POC）

- 新写 part 在 `metadata.bin` 中嵌入各组件 size（flag `component_sizes`）。
- 打开时优先使用嵌入 size；`OpenPart` 深校验仍验证组件存在与损坏拒绝。
- 残缺测试 fixture 可不嵌入 size，打开路径回退 Stat。


### 压缩默认策略（POC）

- 未显式配置时，默认：`Timestamp=delta-of-delta`、`Float=xor`、`Int=delta`、`String=dictionary`。
- 显式 `plain` 不会被覆盖；`Enabled=false` 时仍走未压缩 page 路径。

## 3. Experimental 功能

以下能力可用，但语义/指标名可能在 minor 版本调整：

- `LayeredExecutor` 查询路径（非权威；权威为 Engine + queryexec）
- 部分 admin/debug 接口与详细 explain 字段扩展
- 后台维护细粒度 skip reason 字符串

Experimental 功能变更不构成 major，但应在 CHANGELOG 记录。

## 4. 运维与门禁建议

合并影响存储/查询热路径的变更时，至少：

1. `make test`（含产物清理）
2. `golangci-lint`
3. `make test-race`（或 CI 中核心包 race）
4. 选定 e2e / fault 冒烟
5. 关键路径 bench 对比（median 劣化 >10% 需说明）

完整商用门禁：`make ci`（`scripts/ci_gate.sh`）。


## 5. Format / API 兼容矩阵

| 维度 | 当前策略 | 破坏性变更门槛 |
| --- | --- | --- |
| Public Go API (`github.com/openmts/mts`) | semver；新增字段默认零值兼容 | major：删除/改语义 |
| 错误 sentinel | `errors.Is` 稳定集合见上文 | major：移除 sentinel |
| WAL segment | magic + version；未知 version fail-fast | format major + CHANGELOG |
| SSTable part metadata | version 校验；不支持则拒绝 part | format major + CHANGELOG |
| Manifest | 版本不匹配 fail-fast | format major + CHANGELOG |
| QueryExplain 扩展字段 | minor 可增字段；旧客户端忽略未知 JSON 字段 | major：删除/改类型 |
| 配置 Options 新字段 | minor；零值保持旧行为或安全默认 | major：改变零值安全语义 |

## 6. 本轮相关默认变更（兼容说明）

| 配置 | 默认 | 兼容性 |
| --- | --- | --- |
| `QueryProtection.DefaultMaxSamples` | `DefaultOptions` 为 1e6；零值 Options 不注入 | 显式 budget 优先 |
| `MemTableDisorderFlushRatio` | 0.25（DefaultOptions） | `<=0` 关闭 |
| `MaxConcurrentCompaction` | 归一化默认 1 | `<=0` 使用默认 |
| `MaxConcurrentDownsample` | 归一化默认 2 | 既有行为 |


## 7. POC 格式说明（2026-07-18）

当前为 POC：**不承诺旧 WAL formatID=1 可读**。WAL segment formatID=2，写入统一列式 batch payload。
并行 compact 默认跨 shard，受 `MaxConcurrentCompaction` 限制。


## 8. 并行 Compact 配额

- `Options.MaxConcurrentCompaction` 与 `Options.Compaction.MaxConcurrent` 等价，归一化时互相同步。
- `<=0` 时默认 `min(GOMAXPROCS, 4)`。
- 峰值 CPU/内存约随并发度 N 线性上升，生产建议从 1~2 起调。
