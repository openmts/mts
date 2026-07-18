# Nightly / CI 分层门禁

面向存储与查询热路径变更的建议分层门禁。当前仓库已具备对应 Make 目标；Nightly 建议按层串行固化。

## L0 每次 PR / 本地提交前

```bash
make test
golangci-lint run ./...
```

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

也可直接：

```bash
make ci   # scripts/ci_gate.sh
```

## 失败处理

1. 正确性失败：阻塞合并。
2. bench median 劣化 >10%：说明原因或回退。
3. soak/fault 失败：先判定是否环境噪声，再按根因修复。
