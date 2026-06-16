# Storage Engine Benchmark Guide

## pprof

预生成 points，避免造数成本混入 query profile：

```bash
go build -o /tmp/mts-storage-pprof ./tests/pprof/storage_engine
/tmp/mts-storage-pprof -mode=query -field-layout=wide10 -points=100000 -series=1000 -query-repeat=20 -prebuild-points \
  -cpu-profile=/tmp/mts-query.cpu.prof \
  -mem-profile=/tmp/mts-query.heap.prof
```

检查热点：

```bash
go tool pprof -top -alloc_space /tmp/mts-query.heap.prof
go tool pprof -top /tmp/mts-query.cpu.prof
```

## benchstat

基线和当前结果都使用 10 次采样：

```bash
go test ./internal/bench -run '^$' -bench 'BenchmarkEngineWrite' -benchmem -count=10 > /tmp/mts-old.txt
go test ./internal/bench -run '^$' -bench 'BenchmarkEngineWrite' -benchmem -count=10 > /tmp/mts-new.txt
benchstat /tmp/mts-old.txt /tmp/mts-new.txt
```

没有统计显著性时，不声明性能提升。

## local gate

```bash
scripts/storage_benchmark_gate.sh docs/benchmarks/storage-engine-baseline.txt /tmp/mts-storage-benchmark.txt
```

脚本运行 10K default/wide10 写入 benchmark，输出 `sec/op`、`B/op`、`allocs/op`，存在基线时自动执行 `benchstat`。
