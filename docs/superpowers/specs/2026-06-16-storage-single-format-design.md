# Storage Single Format Design

## 背景

mts 仍处于概念设计阶段，当前没有线上数据兼容负担。此前为 WAL、SSTable、Catalog、Manifest、Envelope、value block 增加了多处版本字段、旧格式读取分派和 legacy JSON 拒绝分支。这些代码会增加读写路径分支、测试复杂度和后续优化成本。

本轮目标是移除所有“版本兼容性相关代码”，只保留当前唯一格式。

## EARS 需求

- 当写入 WAL frame 时，系统应只写当前 frame 结构，不应写 record version 字段。
- 当解码 WAL batch 或 tombstone payload 时，系统应直接按当前格式解码，不应根据 batch/tombstone version 分派。
- 当写入 Catalog、Manifest、SSTable metadata/index/metaindex envelope 时，系统应只写 magic、flags、payload length、payload、CRC，不应写 version 字段。
- 当读取 envelope 时，系统应只校验 magic、payload length 和 CRC，不应处理 maxVersion 或 unsupported version。
- 当读取 SSTable value payload 时，系统应只支持当前 page-index + page payload 格式，不应保留旧 v2/v3/v5 兼容分支。
- 当检测到旧 JSON 文件时，系统不需要提供 legacy JSON 专用错误；缺失当前二进制文件按当前格式缺失处理。
- 当 payload 损坏、magic 不匹配、CRC 不匹配、encoding kind 非当前格式时，系统仍应返回明确错误。

## 保留项

- 保留 magic：用于识别文件类型，防止误读。
- 保留 CRC：用于检测损坏。
- 保留 WAL record type：用于区分 write batch 与 tombstone。
- 保留当前格式内部必要的 encoding kind：例如 value page index 与 value page payload 的类型字节。该字节不用于兼容旧版本，只用于识别当前 payload 类型。

## 移除项

- Envelope `Version` 字段、`maxVersion` 参数和 unsupported version 检查。
- WAL frame `recordVersion` 字节和 unsupported wal record version 分支。
- WAL batch v2/v3 分派、`decodeBatchV2`、旧 point 全量 identity 编码兼容测试。
- Tombstone payload version。
- SSTable metadata `FormatVersion` 字段及检查。
- SSTable value block v2/v3/v4/v5 命名和旧格式兼容读取分支。
- Manifest/metadata legacy JSON 文件名和 legacy JSON 专用错误。
- 所有“兼容旧格式”的测试和文档断言。

## 验证策略

- 单包定向：`go test -count=1 ./internal/codec ./internal/wal ./internal/sstable ./internal/catalog ./internal/engine -timeout 180s`。
- 全量：`go test -count=1 ./... -coverprofile=coverage.out -timeout 600s`，总覆盖率不低于 90.0%。
- Lint：`golangci-lint run --timeout 12m`。
- E2E：逐个 build/run `tests/e2e/*` 并清理二进制。
