# Storage Next Hotpath Optimization Design

## 目标

继续压低 mts 存储层写入和读取热路径的临时分配，重点处理上一轮提交后仍存在的压缩页 fallback、压缩页写入侧 metadata 拆分、WAL batch 编码、Catalog 单点多 tag key 和 benchmark baseline 治理。

## EARS 需求清单

- 当读取 SSTable compressed page 且 value codec 为 XOR、delta 或 dictionary 时，系统应直接按 query 生成 `VersionedSample`，避免先构造完整 `[]FieldValue` 再 build/filter。
- 当写入 SSTable compressed page 时，系统应避免为了 timestamp/writeSeq payload 额外长期持有 `[]int64` 和 `[]uint64` 两个中间切片；编码应直接从 `[]VersionedSample` 流式生成 payload。
- 当编码 WAL batch identity 字典时，系统应复用 batch-local scratch buffer 构造 identity key，避免每条 point 额外分配临时 `[]byte`。
- 当编码 WAL batch field name refs 时，系统应使用连续 arena 存储 refs，避免为每个 point 分配独立 `[]int`。
- 当单点 Catalog resolve 处理多 tag series 时，系统应复用 Catalog 内部 scratch 构造 canonical key，避免每次都分配 tag key scratch。
- 当执行本地 benchmark gate 时，系统应支持创建或更新 baseline 文件，便于后续自动比较性能回归。
- 如果上述优化路径遇到 malformed payload、未知 codec 或类型不匹配，系统应返回明确错误，不应 panic 或静默忽略。
- 如果优化完成，系统应通过定向测试、全量测试、覆盖率、lint 和 e2e 验证，且清理临时产物。

## 设计

SSTable 读取侧在 `readCodecPayloadSamples` 中增加 typed streaming decoder：float XOR、int delta、string dictionary 不再返回 `[]FieldValue`，而是顺序解码并只 append 命中 query 的样本。plain codec 继续使用已有 streaming 路径。

SSTable 写入侧把 compressed page metadata 编码改成从 samples 直接生成 timestamp payload 和 writeSeq payload。timestamp encoder 保留现有候选策略，但新增 samples 入口，避免先 split 两个 metadata slice。

WAL batch encoder 增加 batch scratch：identity key 使用同一个 `[]byte` buffer 复用；field refs 使用一个连续 `[]int` arena，每个 point 只持有 arena 子切片。

Catalog 在 `Catalog` 内增加受互斥锁保护的 `seriesKeyScratch []string`。单点 resolve 多 tag 和 batch resolve 分别复用自己的 scratch，保持 key 语义和排序稳定。

benchmark gate 脚本增加 `--update-baseline`，在本地确认性能基线时可原子写入 baseline 文件；默认仍只运行并对比，不会改 baseline。

## 验证

- `go test -count=1 ./internal/sstable -run 'TestCompressed|TestValuePage' -timeout 180s`
- `go test -count=1 ./internal/wal -timeout 180s`
- `go test -count=1 ./internal/catalog -timeout 180s`
- `go test -count=1 ./internal/bench ./tests/pprof/storage_engine -timeout 180s`
- `timeout 300s goimports-reviser -project-name codeberg.org/mts/mts -recursive -format -rm-unused .`
- `go test -count=1 ./... -coverprofile=coverage.out -timeout 600s`
- `go tool cover -func=coverage.out | tail -1`，总覆盖率不低于 `90.0%`
- `golangci-lint run --timeout 12m`
- 逐个 build/run `tests/e2e/*` 并删除二进制。

## 自检

- Placeholder scan：无 TBD/TODO/后续增强占位。
- Scope：只优化存储热路径和本地性能治理，不改变公开数据模型和持久化格式。
- Type consistency：保留现有 API，新增 helper 均为包内私有实现。
