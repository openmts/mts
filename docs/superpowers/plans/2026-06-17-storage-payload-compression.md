# Storage Payload Compression Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 SSTable typed encoding 之后增加 snappy、lz4、zstd payload 级通用压缩。

**Architecture:** 新增 `payload_compression.go` 封装算法解析、压缩、解压和 header 读写；LZ4 使用 `github.com/pierrec/lz4/v4` 的 pure Go block codec。`compression.go` 的 `appendCodecPayload/readCodecPayload` 负责套用 wrapper，typed decoder 保持原有职责。

**Tech Stack:** Go 1.26.2、`github.com/klauspost/compress/snappy`、`github.com/klauspost/compress/zstd`、`github.com/pierrec/lz4/v4`。

---

### Task 1: 配置与规格落地

**Files:**
- Modify: `internal/model/types.go`
- Modify: `types.go`
- Create: `docs/superpowers/specs/2026-06-17-storage-payload-compression-design.md`
- Create: `docs/superpowers/plans/2026-06-17-storage-payload-compression.md`

- [x] **Step 1: 写入 EARS 规格**

实现备注：已在 spec 中明确算法、header、错误路径、klauspost/lz4 约束和测试策略。

- [x] **Step 2: 增加 public/internal 配置字段**

在 `CompressionOptions` 增加：

```go
Algorithm string
```

并在 `toModelCompressionOptions` 中传递该字段。

验收：`go test -count=1 ./... -run 'Test.*CompressionOptions|Test.*Options' -timeout 180s` 编译通过。

实现备注：已在 public/internal `CompressionOptions` 增加 `Algorithm` 字段，并通过 `toModelCompressionOptions` 传递。

### Task 2: 先写 payload 压缩失败测试

**Files:**
- Modify: `internal/sstable/compression_test.go`

- [x] **Step 1: 增加 round trip 测试**

新增测试 `TestPayloadCompressionAlgorithmsRoundTrip`，覆盖 `none/snappy/lz4/zstd`，断言解压结果等于原始 payload。

- [x] **Step 2: 增加错误路径测试**

新增测试：

```go
TestPayloadCompressionRejectsUnknownAlgorithm
TestCompressedPayloadRejectsCorruptSize
TestCompressedPayloadRejectsUnknownAlgorithmID
```

验收：实现前运行 `go test -count=1 ./internal/sstable -run 'TestPayloadCompression|TestCompressedPayload' -timeout 180s` 应失败，失败原因是函数或行为尚未实现。

实现备注：已确认 RED，失败原因为缺少 `appendCodecPayloadWithCompression`、`payloadCompressionNone` 和 `CompressionOptions.Algorithm`。

### Task 3: 实现 payload 压缩抽象

**Files:**
- Create: `internal/sstable/payload_compression.go`
- Modify: `internal/sstable/compression.go`

- [x] **Step 1: 定义算法 ID 与解析函数**

实现 `payloadCompressionAlgorithmID(name string) (byte, error)`，支持 `""`、`none`、`snappy`、`lz4`、`zstd`。

- [x] **Step 2: 实现 snappy/zstd 编解码**

`snappy` 使用 `snappy.Encode/Decode`；`zstd` 使用 `zstd.NewWriter/NewReader` 的 `EncodeAll/DecodeAll`，并通过 `sync.Pool` 复用实例。

- [x] **Step 3: 改造 codec payload wrapper**

把 `appendCodecPayload` 扩展为算法参数，并新增 `appendCodecPayloadWithCompression`；把 `readCodecPayload` 改为读取算法、原始长度、存储长度并解压。

验收：Task 2 测试中 `none/snappy/zstd` 相关用例通过，`lz4` 用例仍失败。

实现备注：已新增 `payload_compression.go`，wrapper header 包含 typed codec、payload 算法、原始长度、存储长度和存储 payload。

### Task 4: 接入 pure Go LZ4 block codec

**Files:**
- Modify: `internal/sstable/payload_compression.go`
- Test: `internal/sstable/compression_test.go`

- [x] **Step 1: 接入 LZ4 block encode**

使用 `github.com/pierrec/lz4/v4` 的 `Compressor.CompressBlock` 和 `CompressBlockBound`。compressor 通过 `sync.Pool` 复用，避免反复分配内部表。

- [x] **Step 2: 接入 LZ4 block decode**

使用 `github.com/pierrec/lz4/v4` 的 `UncompressBlock`，并保留 payload header 的 raw size 校验。

验收：`go test -count=1 ./internal/sstable -run 'TestPayloadCompressionAlgorithmsRoundTrip|TestCompressedPayload' -timeout 180s` 通过。

实现备注：已改为使用 `github.com/pierrec/lz4/v4`。LZ4 block 不可压缩返回 0 时，单段 payload 回退为 `none` 存原文，避免扩大落盘。

### Task 5: SSTable 集成与尺寸验证

**Files:**
- Modify: `internal/sstable/compression.go`
- Modify: `internal/sstable/internal_test.go`
- Modify: `internal/sstable/sstable_test.go`

- [x] **Step 1: 写入路径传入配置算法**

`marshalCompressedValueBlock` 对 timestamps、writeSeqs、values 三段调用压缩 wrapper。

- [x] **Step 2: 增加 SSTable round trip 测试**

覆盖四种字段类型，分别用 `snappy/lz4/zstd` 写入后查询验证。

- [x] **Step 3: 增加落盘尺寸测试**

写入重复字符串和规律数值，对比 `Algorithm=""` 与 `Algorithm="zstd"` 的 `values.bin` 大小，断言 zstd 更小。

验收：`go test -count=1 ./internal/sstable -run 'Test.*PayloadCompression|TestWritePart.*Compression|TestPart.*Compression' -timeout 180s` 通过。

实现备注：已通过 `go test -count=1 ./internal/sstable -run 'TestPayloadCompression|TestCompressedPayload|TestWritePartWithPayloadCompression' -timeout 180s` 和 `go test -count=1 ./internal/sstable -timeout 180s`。

### Task 6: 依赖、格式化和全量验证

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [x] **Step 1: 添加指定依赖**

运行：

```bash
go get github.com/klauspost/compress@v1.18.6
go mod tidy
```

实现备注：已添加 `github.com/klauspost/compress v1.18.6` 并执行 `go mod tidy`。

- [x] **Step 2: 格式化**

运行：

```bash
timeout 300s goimports-reviser -project-name codeberg.org/mts/mts -recursive -format -rm-unused .
```

实现备注：已执行，命令退出码 0。

- [x] **Step 3: 全量测试与覆盖率**

运行：

```bash
go test -count=1 ./... -coverprofile=coverage.out -timeout 600s
```

覆盖率验收：`go tool cover -func=coverage.out | tail -1` 总覆盖率 `>=90.0%`。

实现备注：已执行 `go test -count=1 ./... -coverprofile=coverage.out -timeout 600s`，并确认总覆盖率 `90.1%`。

- [x] **Step 4: lint**

运行：

```bash
golangci-lint run --timeout 12m
```

实现备注：已执行，输出 `0 issues.`。

- [x] **Step 5: e2e**

逐个执行 `tests/e2e/*` 下的 Go 用例：`go build -o testbin && ./testbin`，每个用例设置独立超时，完成后删除 `testbin`。

实现备注：已逐个 build 并用 `timeout 120s ./testbin` 运行 `compaction_integrity`、`flush_manifest_recovery`、`no_json_storage`、`query_pruning`、`retention`、`simple_integrity`、`wal_recovery`，全部通过。

- [x] **Step 6: 清理产物并提交**

删除 `coverage.out`、`*.prof`、`*.testbin`、e2e 二进制等临时产物。提交：

```bash
git add .
git commit -m "feat(storage): 增加SSTable payload压缩"
```

实现备注：已清理 `coverage.out`、`sstable.cover`、`tests/e2e/*/testbin` 等临时产物，`git diff --check` 无输出；本任务代码提交使用 `feat(storage): 增加SSTable payload压缩`。
