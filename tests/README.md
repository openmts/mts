# tests

`tests/e2e` 存放端到端功能测试，每个子目录是独立可执行用例。

`tests/fault` 存放持久化故障矩阵，用于验证 WAL、Manifest、PartWriter、FileOps、Compaction、Retention 等关键故障点和重启恢复。

`tests/scale` 存放规模化写入、查询、compaction、restart 和 soak workload，输出机器可读 JSON。

`tests/pprof` 存放性能剖析 workload，每个子目录是独立可执行程序，可生成 CPU、heap 等 profile。

常用门禁：

```bash
make e2e
make fault
make scale
```

单场景入口可执行 `make e2e-public-api`、`make fault-matrix`、`make storage-100k`、`make bench-query`，完整列表见 `make help`。

`tests/fault/storage_fault_matrix` 会输出包含 case、operation、stage、expected、recovered、rows、maintenance_issues 的 JSON 报告。
