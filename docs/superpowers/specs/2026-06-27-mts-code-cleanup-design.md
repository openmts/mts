# MTS 代码清理设计

## 背景

基于 `docs/review/code-review-2026-06-27-1540.md` 的重复代码检视，本轮目标是在不破坏当前模块边界的前提下清理可安全合并的重复代码。

## EARS 需求

- When 多个包需要浅拷贝 map 或 slice 时，系统应通过职责明确的 `internal/collections` 泛型工具复用逻辑。
- When 多个包需要按字符串 key 排序 map 时，系统应通过 `internal/collections.SortedKeys` 复用逻辑。
- When HTTP handler 需要严格 JSON decode 或 JSON response 时，系统应通过 `internal/httpjson` 统一基础协议行为。
- When `mts-server` 将内部错误映射到 HTTP/gRPC 响应时，系统应先使用统一错误分类，再由协议层转换状态码。
- When `mts-server` 从 HTTP header 或 gRPC metadata 获取凭据时，系统应通过统一 credential source 合并 admin/data 鉴权逻辑。
- When `mts-server` 注册 HTTP route、gRPC method 或写 stream record 时，系统应使用命名常量，避免协议字符串散落。
- When 测试代码存在高频 open/close 重复时，系统应在对应包内用局部 helper 收敛，不创建跨包测试 utils。
- If 重复代码属于 WAL/Catalog/SSTable/User metadata 持久化格式 reader/writer，系统应保持分离，避免格式演进耦合。
- If 抽象会形成宽泛顶层 utils 包，系统应改用职责明确的 internal 小包。

## 设计

新增 `internal/collections`，仅放纯泛型集合工具，不依赖业务包。第一批提供 `CloneMap`、`CloneMapNilIfEmpty`、`CloneSlice`、`SortedKeys`，替换报告中列出的重复 clone 和 sorted key 函数。

新增 `internal/httpjson`，仅封装 `DecodeStrict`、`Write` 和 `WriteRaw`。业务错误结构仍留在调用方，避免把领域错误放入通用包。

`cmd/mts-server` 增加协议常量文件，集中 HTTP route、gRPC method、stream type、通用消息和 bearer prefix。HTTP 和 gRPC 错误映射改为先调用 `classifyAPIError`，再转换成 HTTP status 或 gRPC code。

`cmd/mts-server` 鉴权层引入 `credentialSource`，HTTP request 与 gRPC metadata 各自适配，admin/data 逻辑共用统一函数。

HTTP/gRPC operation registry 本轮采用低风险落地：先将 route/method 清单常量化，并用小 wrapper 收敛 method/admin/data 的重复，不强行把所有 handler 改造成大型 registry，以避免一次性改动过大。

## 验收

- 报告中 P0/P1/P2 待处理项均更新状态。
- `cloneStringMap` 和 sorted key 机械重复显著减少，剩余 clone 函数只保留深拷贝业务语义。
- HTTP/gRPC 错误分类共用同一函数，未知错误在 gRPC 侧映射为 `codes.Internal`。
- HTTP/gRPC admin/data 鉴权通过同一 credential source 逻辑。
- 协议硬编码字符串被常量化。
- 通过 `goimports-reviser`、`golangci-lint`、`go test ./...`、`git diff --check`。

