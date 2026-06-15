# tests Modules Design

## 目标

为 `mts` 增加统一的 `tests` 目录，将端到端功能验证与性能剖析入口分离，后续新增模块测试时有固定落点和运行方式。

## 需求

- When 需要新增端到端测试时，系统 shall 将测试放在 `tests/e2e/<case>`，每个 case shall 是可独立 `go build` 和执行的 `main` 包。
- When e2e case 运行时，系统 shall 只依赖 `mts` 公共 API，不直接导入 `internal` 包。
- When e2e case 发现数据不一致、恢复失败或查询错误时，系统 shall 以非零退出码失败。
- When 需要剖析性能问题时，系统 shall 将 workload 放在 `tests/pprof/<target>`，并通过命令行参数控制数据量、profile 输出和数据目录。
- If 未显式传入数据目录，pprof workload shall 使用临时目录并在退出时清理。
- If 显式传入 profile 输出路径，pprof workload shall 使用 `0600` 权限创建 profile 文件。

## 目录设计

```text
tests/
  README.md
  e2e/
    README.md
    simple_integrity/
      main.go
  pprof/
    README.md
    storage_engine/
      main.go
```

## 运行约定

`tests/e2e/*` 使用已有项目约定运行：

```bash
cd tests/e2e/simple_integrity
go build -o simple_integrity .
./simple_integrity
rm -f simple_integrity
```

`tests/pprof/*` 用于手动剖析，不作为普通单元测试自动执行：

```bash
cd tests/pprof/storage_engine
go build -o storage_engine .
./storage_engine -points 100000 -series 100 -cpu-profile cpu.prof -mem-profile mem.prof
go tool pprof cpu.prof
rm -f storage_engine cpu.prof mem.prof
```

## 验收

- `go test ./... -timeout 600s` 能编译所有新增 `main` 包。
- `tests/e2e/simple_integrity` 能构建并执行通过。
- `tests/pprof/storage_engine` 能构建，并能在指定 profile 参数时生成 profile 文件。
- 新增目录权限为 `0700`，新增文件权限为 `0600`。
