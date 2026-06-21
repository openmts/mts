# Downsample Performance Case Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 扩展降采样 scale 用例，输出 raw 写入、降采样物化写入和目标 RP 查询的分阶段性能报告。

**Architecture:** 复用 `tests/scale/downsample_policy` 单入口，不新增重复测试目录。用例按 `write raw -> create policy -> run downsample -> query target -> optional verify` 执行，每个阶段单独计时并输出 JSON 报告。

**Tech Stack:** Go、MTS public API、`tests/scale/downsample_policy`、JSON report、RSS peak。

---

## Task 1: 报告结构与配置

**状态:** 已完成。

**EARS:**
- When 用例运行时，报告应包含 raw 写入耗时、写入吞吐、降采样耗时、查询耗时、查询行数、verify 状态和 RSS peak。
- When 用户传入 `-query-limit` 或 `-verify=false` 时，用例应按配置执行。

**Files:**
- Modify: `tests/scale/downsample_policy/main.go`
- Test: `tests/scale/downsample_policy/main_test.go`

- [x] **Step 1: 扩展 `report` 与 `config`**

新增字段：

```go
WriteDurationNanos      int64
WriteThroughput         float64
DownsampleDurationNanos int64
DownsampleThroughput    float64
QueryDurationNanos      int64
QueryRows               int
QueryLimit              int
Verify                  bool
Verified                bool
```

- [x] **Step 2: 增加参数校验**

**实现备注:** 已扩展报告字段，新增 `-query-limit` 和 `-verify` 参数，并对 points、series、query limit 做显式校验。

新增 `-query-limit` 和 `-verify` 参数，要求 query limit 大于 0。

## Task 2: 分阶段性能采集

**状态:** 已完成。

**EARS:**
- When 写入 raw 数据完成时，系统应记录写入耗时和 points/sec。
- When 降采样执行完成时，系统应记录执行耗时、物化点数和 points/sec。
- When 查询目标 RP 完成时，系统应记录查询耗时、返回行数和可选校验结果。

**Files:**
- Modify: `tests/scale/downsample_policy/main.go`

- [x] **Step 1: `runWorkload` 返回分阶段报告**
- [x] **Step 2: 新增 `queryTarget` 与 `verifyRows`**
- [x] **Step 3: 保持默认 100K/100 series/limit 2000**

**实现备注:** `runWorkload` 已按 raw write、downsample、target query 三阶段计时。`verifyRows` 会按 host 和窗口重新计算 `avg/min/max/count/last`，默认查询目标 RP `LIMIT 2000`。

## Task 3: 验证与性能运行

**状态:** 已完成。

**EARS:**
- When 定向测试运行时，downsample performance scale 包应通过。
- When 默认 100K 性能用例运行时，应输出可读 JSON 性能报告。

**Files:**
- Modify: `tests/scale/downsample_policy/main_test.go`

- [x] **Step 1: 更新 smoke 测试断言**
- [x] **Step 2: 运行格式化**
- [x] **Step 3: 运行定向测试**
- [x] **Step 4: 运行 100K 性能用例并记录结果**

**验证命令:**

```bash
timeout 300s goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .
timeout 600s go test ./tests/scale/downsample_policy -count=1 -timeout 10m
timeout 600s go run ./tests/scale/downsample_policy -points 100000 -series 100 -query-limit 2000 -verify=true
```

**100K 执行结果:**

```json
{
  "points": 100000,
  "series": 100,
  "query_limit": 2000,
  "verify": true,
  "verified": true,
  "windows_processed": 18,
  "points_written": 1700,
  "query_rows": 1700,
  "duration_nanos": 283460337,
  "write_duration_nanos": 190250162,
  "downsample_duration_nanos": 84901075,
  "query_duration_nanos": 7428788,
  "write_throughput_points_per_second": 525623.7311377428,
  "downsample_throughput_points_per_second": 20023.303591856755,
  "rss_peak_bytes": 13598720,
  "completed_until_unix": 1080000000000
}
```

**最终验证命令:**

```bash
timeout 600s go test ./... -count=1 -timeout 10m
timeout 720s golangci-lint run ./...
timeout 720s go test ./... -cover -count=1 -timeout 10m
timeout 60s find . -type f \( -name '*.test' -o -name '*.prof' -o -name '*.pprof' -o -name 'coverage.out' -o -name '*.coverprofile' \) -not -path './.git/*' -print
timeout 60s git diff --check
```
