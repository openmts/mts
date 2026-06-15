# Storage Engine Phase 2 Benchmarks

## 环境

- GOOS: `linux`
- GOARCH: `amd64`
- CPU: `AMD Ryzen 7 7840HS w/ Radeon 780M Graphics`

## 写入基线

命令：

```bash
go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s
```

结果：

```text
BenchmarkEngineWriteBatch/points=1000-16         	      76	  14431194 ns/op	 8834518 B/op	   62468 allocs/op
BenchmarkEngineWriteBatch/points=1000-16         	      80	  14449191 ns/op	 8835046 B/op	   62468 allocs/op
BenchmarkEngineWriteBatch/points=1000-16         	      84	  14452843 ns/op	 8836506 B/op	   62469 allocs/op
BenchmarkEngineWriteBatch/points=10000-16        	      21	  51934093 ns/op	45844912 B/op	  254383 allocs/op
BenchmarkEngineWriteBatch/points=10000-16        	      22	  51123216 ns/op	45843042 B/op	  254380 allocs/op
BenchmarkEngineWriteBatch/points=10000-16        	      22	  51637642 ns/op	45845082 B/op	  254382 allocs/op
```

## Profile Smoke

命令：

```bash
cd tests/pprof/storage_engine && go build -o storage_engine . && ./storage_engine -mode=query -points 10000 -series 100 -cpu-profile cpu.prof -mem-profile mem.prof
```

结果：CPU 和 heap profile 均生成成功，命令结束后已清理 `storage_engine`、`cpu.prof`、`mem.prof`。
