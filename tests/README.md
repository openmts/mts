# tests

`tests/e2e` 存放端到端功能测试，每个子目录是独立可执行用例。

`tests/fault` 存放持久化故障矩阵，用于验证 create/write/sync/rename/remove/stat/walk 等故障返回和重启恢复。

`tests/scale` 存放规模化写入、查询、compaction、restart 和 soak workload，输出机器可读 JSON。

`tests/pprof` 存放性能剖析 workload，每个子目录是独立可执行程序，可生成 CPU、heap 等 profile。

常用门禁：

```bash
go test -count=1 ./tests/fault/... ./tests/scale/... -timeout 600s
for dir in tests/e2e/*; do (cd "$dir" && go build -o testbin . && timeout 120s ./testbin; status=$?; rm -f testbin; exit $status) || exit $?; done
```
