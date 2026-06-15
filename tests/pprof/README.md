# pprof

这里存放用于性能问题剖析的独立 workload。示例：

```bash
cd tests/pprof/storage_engine
go build -o storage_engine .
./storage_engine -points 100000 -series 100 -cpu-profile cpu.prof -mem-profile mem.prof
go tool pprof cpu.prof
go tool pprof -alloc_space mem.prof
rm -f storage_engine cpu.prof mem.prof
```

约定：默认使用临时数据目录并自动清理；如需复查磁盘文件，可通过 `-data-dir` 指定目录。
