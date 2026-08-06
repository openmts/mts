# Nightly / CI 分层门禁

面向存储与查询热路径变更的建议分层门禁。当前仓库已具备对应 Make 目标；Nightly 建议按层串行固化。

## L0 每次 PR / 本地提交前

```bash
make test
golangci-lint run ./...
make coverage
make dashboard-gate
```

`make coverage` 逐个检查有生产语句的 Go 包，最低覆盖率为 `90%`。`make dashboard-gate` 使用锁定依赖执行 ESLint、Node 单测覆盖率（lines/functions `90%`、branches `70%`）、危险 DOM sink 扫描及 npm 官方 registry 的 high 级漏洞审计。

## L1 核心 race（热路径包）

```bash
go test -race ./internal/engine ./internal/memtable ./internal/wal ./internal/sstable -count=1 -timeout 10m
```

## L2 e2e

```bash
make e2e
```

## L3 Nightly

```bash
make fault-matrix
make storage-soak
make scale-storage   # 或 storage-matrix
make bench           # scripts/storage_benchmark_gate.sh
```

`make bench` 默认使用版本化基线 `docs/benchmarks/storage-engine-baseline.txt`，并要求安装官方 `benchstat`：

```bash
go install golang.org/x/perf/cmd/benchstat@latest
```

门禁串行运行 4 个 10K 存储/查询 benchmark，各采样 10 次。只有在同一 Go 版本、CPU、goos/goarch 和 CPU governor 下才比较；缺基线、缺工具、样本不完整或环境不一致都会失败。`sec/op` median 劣化超过 10% 时失败；`B/op` 或 `allocs/op` 的 current median 增大且 `benchstat` 判定 `p < 0.05` 时失败。

只有在专用 benchmark runner 上确认环境稳定后，才显式更新基线：

```bash
bash scripts/storage_benchmark_gate.sh --update-baseline
```

基线会以 0600 权限原子更新，并记录 UTC、Go 版本、CPU、governor 和采样参数。禁止因门禁失败直接重试直到通过；应先定位环境噪声或性能根因。

也可直接：

```bash
make ci   # scripts/ci_gate.sh
```

## 失败处理

1. 正确性失败：阻塞合并。
2. benchmark gate 非零退出：阻塞合并；定位环境不一致、`sec/op` median >10% 或显著 allocation 回归后再处理。
3. soak/fault 失败：先判定是否环境噪声，再按根因修复。
