# Storage P1-05/04/02 Closure Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans

**Goal:** 闭环检视报告剩余 P1：Catalog 基数硬限制、public Delete API、查询权威栈固化与一致性测试。无弱实现、无遗留 TODO。

**Architecture:**
- Cardinality 配置走 `Options.Cardinality` → model → catalog.Limits，在 create series/field/tag value 时拒绝。
- Delete 走 public `DeleteRequest` → engine 分 shard `DeleteRange`（tombstone + writeSeq）。
- 查询权威路径固定为 `engine + queryexec`；LayeredExecutor 标注实验/非权威，并补 golden 对比测试。

**Tech Stack:** Go 1.26、现有测试与 make 门禁。

## Task 1 P1-05 Cardinality
- [x] 测试超限拒绝
- [x] catalog Limits + 错误
- [x] Options 贯通 + Validate
- [x] 单元/覆盖

## Task 2 P1-04 Delete API
- [x] public Delete + engine 实现
- [x] 测试：按时间/series 删除后查询不可见
- [x] 错误与 precision

## Task 3 P1-02 Query authority
- [x] 文档标注权威栈
- [x] Layered vs engine golden 一致性测试
- [x] arch/注释固化

## Task 4 全量验证
- [x] make test / e2e / lint / bench
- [x] 更新 review

## 实现备注


## 实现备注（2026-07-18 闭环）

### P1-05 Cardinality
- `catalog.Limits` / `OpenWithOptions`
- create series/field/tag value 时硬拒绝
- `Options.Cardinality` → model → engine metadata open
- public `ErrCardinalityLimit`

### P1-04 Delete API
- public `DeleteRequest` + `Engine.Delete`
- internal `engine.Delete` 分 shard `DeleteRange` + writeSeq
- 支持 measurement / tags / time range；precision 转换

### P1-02 Query authority
- README/doc.go/LayeredExecutor 注释明确权威路径 = Engine+queryexec
- golden: LayeredExecutor 与 Engine.QueryRows / QuerySpecRows 一致

### 验证
- make test 通过
- make e2e 通过
- golangci-lint 0 issues
- bench median 相对基线见会话对比

