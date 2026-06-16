# Storage Single Format Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 移除存储层所有版本兼容代码，只保留当前唯一持久化格式。

**Architecture:** 保留 magic、CRC、WAL record type 和当前格式必要的 encoding kind；删除 version 字段、旧格式分派、legacy JSON 专用处理和兼容测试。修改按 codec -> WAL -> SSTable/Catalog/Manifest -> 测试验证推进。

**Tech Stack:** Go、二进制 WAL、SSTable、Catalog、Manifest、go test、golangci-lint、goimports-reviser。

---

### Task 1: Codec Envelope 单格式化

**Files:**
- Modify: `internal/codec/envelope.go`
- Modify: `internal/codec/codec_test.go`
- Modify callers under `internal/catalog` and `internal/sstable`

- [x] **Step 1: 删除 envelope version 字段和参数**
  - 实现备注：`Envelope` 只保留 magic、flags、payload，frame header 删除 version 两字节。
- [x] **Step 2: 更新所有 MarshalEnvelope / UnmarshalEnvelope 调用**
  - 实现备注：catalog、metadata、manifest、sstable metadata/index/metaindex 已切换为单格式签名。
- [x] **Step 3: 删除 unsupported version 测试并保留 magic/CRC/truncation 测试**
  - 实现备注：移除 unsupported version 测试，legacy JSON 路径测试改为当前二进制损坏检测或忽略旧文件名。
- [x] **Step 4: 运行 `go test -count=1 ./internal/codec ./internal/catalog ./internal/sstable -timeout 180s`**
  - 验证结果：通过。

### Task 2: WAL 单格式化

**Files:**
- Modify: `internal/wal/wal.go`
- Modify: `internal/wal/encoding.go`
- Modify: `internal/wal/internal_test.go`
- Modify: `internal/wal/wal_test.go`

- [x] **Step 1: 删除 WAL frame recordVersion 字节**
  - 实现备注：frame body 改为 `[recordType][payload][crc]`，CRC 偏移同步更新。
- [x] **Step 2: 删除 batchVersionV2 / batchVersion / tombstoneVersion 与对应分派**
  - 实现备注：batch 直接从 identity dictionary count 开始，tombstone 直接从 tombstone count 开始。
- [x] **Step 3: 删除 decodeBatchV2 / readPoint 旧格式兼容路径**
  - 实现备注：移除旧全量 identity 解码路径，当前字典 batch 函数改为无版本命名。
- [x] **Step 4: 更新 WAL 测试为当前格式 round-trip 与损坏检测**
  - 实现备注：删除 v2 兼容测试，保留字典 round-trip、紧凑性、坏引用、截断和未知 record type 测试。
- [x] **Step 5: 运行 `go test -count=1 ./internal/wal -timeout 180s`**
  - 验证结果：通过。

### Task 3: SSTable 单格式化

**Files:**
- Modify: `internal/sstable/types.go`
- Modify: `internal/sstable/encoding.go`
- Modify: `internal/sstable/compression.go`
- Modify: `internal/sstable/metadata_encoding.go`
- Modify: `internal/sstable/read.go`
- Modify: `internal/sstable/manifest.go`
- Modify tests under `internal/sstable`

- [x] **Step 1: 删除 metadata FormatVersion 与检查**
  - 实现备注：metadata payload 直接从 PartMeta 开始，reader 不再检查格式版本。
- [x] **Step 2: 删除 legacy metadata/manifest JSON 检测**
  - 实现备注：移除旧 JSON 文件名常量和专用拒绝逻辑，缺失当前文件按当前路径处理。
- [x] **Step 3: 将 valueEncodingV2/V3/V4/V5 重命名为当前格式 encoding kind**
  - 实现备注：改为 `valueEncodingPagePlain`、`valueEncodingPageIndex`、`valueEncodingPageCompressed`。
- [x] **Step 4: 删除旧 value block 兼容读取与测试**
  - 实现备注：删除 standalone value block 旧格式读写，测试改为当前 plain page 读写。
- [x] **Step 5: 保留当前 page-index + page payload 与 compressed page 读写**
  - 实现备注：`readValueColumn` 仍通过 page index 定位 page，page 支持 plain 与 compressed 两种当前 kind。
- [x] **Step 6: 运行 `go test -count=1 ./internal/sstable -timeout 180s`**
  - 验证结果：通过。

### Task 4: 集成验证

**Files:**
- Modify: `docs/superpowers/plans/2026-06-16-storage-single-format.md`
- Optional Modify: related docs that state compatibility as current behavior

- [x] **Step 1: 运行 goimports-reviser**
  - 验证结果：`goimports-reviser -project-name codeberg.org/mts/mts -recursive -format -rm-unused .` 通过。
- [x] **Step 2: 运行定向包测试**
  - 验证结果：`go test -count=1 ./internal/codec ./internal/wal ./internal/sstable ./internal/catalog ./internal/engine -timeout 180s` 通过。
- [x] **Step 3: 运行全量测试与覆盖率**
  - 验证结果：`go test -count=1 ./... -coverprofile=coverage.out -timeout 600s` 通过；总覆盖率 90.2%。
- [x] **Step 4: 运行 golangci-lint**
  - 验证结果：`golangci-lint run --timeout 12m` 通过，0 issues。
- [x] **Step 5: 逐个运行 tests/e2e**
  - 验证结果：`tests/e2e/*` 全部 build/run 通过。
- [x] **Step 6: 清理 coverage、e2e binary、临时 profile**
  - 验证结果：已删除 `coverage.out`，e2e 目录无残留可执行二进制。

## 自检

- Spec coverage：Task 1 覆盖 envelope，Task 2 覆盖 WAL，Task 3 覆盖 SSTable/manifest，Task 4 覆盖验证。
- Placeholder scan：无占位项。
- Type consistency：不改变公开 API，只改变内部持久化格式。
