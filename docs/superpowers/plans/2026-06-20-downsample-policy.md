# Downsample Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 MTS 实现单机规则驱动物化降采样，将原始时序数据按固定窗口聚合写入目标 retention policy。

**Architecture:** 降采样作为 Engine 层后台规则系统实现，不改变 SSTable 文件格式，不接入 compaction rollup。Policy 和 watermark 通过 LocalMetadataStore 本地持久化，执行结果走现有 Engine Write 路径，用户显式查询目标 retention policy。

**Tech Stack:** Go、LocalMetadataStore、Engine、querylang Builder、queryexec aggregate、WAL/MemTable/SSTable 写入主链路、observability、tests/e2e、tests/scale。

---

## 固定边界

- 不实现 SQL、InfluxQL、PromQL parser。
- 不实现自动查询路由。
- 不在 compaction 中生成 rollup SSTable。
- 不实现分布式调度。
- 不接入外部元数据系统。

## 文件职责

- `internal/model/types.go`：新增 `DownsamplePolicy`、`DownsampleFunction`、`DownsampleWatermark`、`DownsampleStats`。
- `types.go`：暴露 public downsample 类型和转换函数。
- `internal/engine/downsample_validate.go`：policy 校验、函数规范化、字段命名、窗口对齐。
- `internal/engine/downsample_metadata.go`：MetadataStore downsample 接口扩展和 Engine 管理方法。
- `internal/catalog/downsample.go`：LocalMetadataStore 背后的本地 policy/watermark 持久化。
- `internal/engine/downsample_scheduler.go`：scheduler 生命周期、tick、并发保护。
- `internal/engine/downsample_executor.go`：按窗口执行查询并写入目标。
- `internal/engine/downsample_stats.go`：运行指标与 HealthSnapshot 集成。
- `internal/observability/prometheus_text.go`：Prometheus 文本指标输出。
- `docs/storage/downsample-runbook.md`：运维文档。
- `tests/e2e/downsample_policy/main.go`：端到端正确性。
- `tests/scale/downsample_policy/main.go`：100K smoke 性能与 RSS。

## Task 1: 模型与校验

**状态:** 待执行。

**EARS:**
- When policy 名称为空、interval 非正、函数为空或 source/target 完全相同时，系统应返回明确校验错误。
- When function `mean` 被配置时，系统应规范化为 `avg`。
- When `As` 为空时，系统应生成 `<function>_<field>` 字段名。

**Files:**
- Modify: `internal/model/types.go`
- Create: `internal/engine/downsample_validate.go`
- Test: `internal/engine/downsample_validate_test.go`

- [ ] **Step 1: 写失败测试**

新增测试函数：

```go
func TestValidateDownsamplePolicyRejectsInvalidInput(t *testing.T) {
	policy := model.DownsamplePolicy{}
	if err := validateDownsamplePolicy(policy); err == nil {
		t.Fatal("validateDownsamplePolicy(empty) error = nil, want error")
	}
	policy = model.DownsamplePolicy{
		Name: "raw-loop",
		SourceDatabase: "metrics", SourceRetention: "autogen", SourceMeasurement: "cpu",
		TargetDatabase: "metrics", TargetRetention: "autogen", TargetMeasurement: "cpu",
		Interval: time.Minute,
		Functions: []model.DownsampleFunction{{Function: "avg", Field: "usage"}},
		Delay: time.Minute,
		RefreshInterval: time.Minute,
		Lookback: time.Minute,
	}
	if err := validateDownsamplePolicy(policy); err == nil {
		t.Fatal("validateDownsamplePolicy(source=target) error = nil, want error")
	}
}

func TestNormalizeDownsamplePolicyFunctionsAndOutputNames(t *testing.T) {
	policy := model.DownsamplePolicy{
		Name: "cpu_1m",
		SourceDatabase: "metrics", SourceRetention: "autogen", SourceMeasurement: "cpu",
		TargetDatabase: "metrics", TargetRetention: "rp_1m", TargetMeasurement: "cpu",
		Interval: time.Minute,
		Functions: []model.DownsampleFunction{
			{Function: "mean", Field: "usage"},
			{Function: "max", Field: "usage", As: "usage_peak"},
		},
		Delay: time.Minute,
		RefreshInterval: time.Minute,
		Lookback: 3 * time.Minute,
		Enabled: true,
	}
	got, err := normalizeDownsamplePolicy(policy)
	if err != nil {
		t.Fatalf("normalizeDownsamplePolicy() error = %v", err)
	}
	if got.Functions[0].Function != "avg" || got.Functions[0].As != "avg_usage" {
		t.Fatalf("first function = %#v, want avg_usage", got.Functions[0])
	}
	if got.Functions[1].As != "usage_peak" {
		t.Fatalf("second As = %q, want usage_peak", got.Functions[1].As)
	}
}
```

运行：

```bash
timeout 180s go test ./internal/engine -run 'TestValidateDownsample|TestNormalizeDownsample' -count=1 -timeout 3m
```

Expected: FAIL，提示类型或函数未定义。

- [ ] **Step 2: 实现模型和校验**

在 `internal/model/types.go` 增加：

```go
type DownsamplePolicy struct {
	Name              string
	SourceDatabase    string
	SourceRetention   string
	SourceMeasurement string
	TargetDatabase    string
	TargetRetention   string
	TargetMeasurement string
	Interval          time.Duration
	Functions         []DownsampleFunction
	GroupByTags       []string
	Delay             time.Duration
	RefreshInterval   time.Duration
	Lookback          time.Duration
	Enabled           bool
}

type DownsampleFunction struct {
	Function string
	Field    string
	As       string
}

type DownsampleWatermark struct {
	PolicyName         string
	CompletedUntilUnix int64
	LastRunUnix        int64
	LastSuccessUnix    int64
	LastError          string
}
```

在 `downsample_validate.go` 实现 `normalizeDownsamplePolicy`、`validateDownsamplePolicy`、`downsampleOutputFieldName`、`alignDownsampleWindow`。

- [ ] **Step 3: 验证**

运行：

```bash
timeout 180s go test ./internal/engine -run 'TestValidateDownsample|TestNormalizeDownsample' -count=1 -timeout 3m
```

Expected: PASS。

## Task 2: LocalMetadataStore 持久化

**状态:** 待执行。

**EARS:**
- When Engine 重启时，系统应恢复所有 downsample policy 和 watermark。
- When metadata 写入失败时，系统应返回错误，不能只更新内存。

**Files:**
- Modify: `internal/engine/metadata_store.go`
- Create: `internal/catalog/downsample.go`
- Test: `internal/catalog/downsample_test.go`
- Test: `internal/engine/downsample_metadata_test.go`

- [ ] **Step 1: 写失败测试**

新增 catalog 测试：

```go
func TestCatalogDownsamplePoliciesPersistAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	cat, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	policy := model.DownsamplePolicy{
		Name: "cpu_1m",
		SourceDatabase: "metrics", SourceRetention: "autogen", SourceMeasurement: "cpu",
		TargetDatabase: "metrics", TargetRetention: "rp_1m", TargetMeasurement: "cpu",
		Interval: time.Minute,
		Functions: []model.DownsampleFunction{{Function: "avg", Field: "usage", As: "avg_usage"}},
		Delay: time.Minute,
		RefreshInterval: time.Minute,
		Lookback: 3 * time.Minute,
		Enabled: true,
	}
	if err := cat.UpsertDownsamplePolicy(policy); err != nil {
		t.Fatalf("UpsertDownsamplePolicy() error = %v", err)
	}
	if err := cat.UpdateDownsampleWatermark(model.DownsampleWatermark{
		PolicyName: "cpu_1m", CompletedUntilUnix: int64(time.Minute),
	}); err != nil {
		t.Fatalf("UpdateDownsampleWatermark() error = %v", err)
	}
	if err := cat.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(reopen) error = %v", err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Fatalf("Close(reopened) error = %v", err)
		}
	}()
	policies, err := reopened.ListDownsamplePolicies()
	if err != nil {
		t.Fatalf("ListDownsamplePolicies() error = %v", err)
	}
	if len(policies) != 1 || policies[0].Name != "cpu_1m" {
		t.Fatalf("policies = %#v, want cpu_1m", policies)
	}
	watermark, ok := reopened.DownsampleWatermark("cpu_1m")
	if !ok || watermark.CompletedUntilUnix != int64(time.Minute) {
		t.Fatalf("watermark = %#v ok=%v, want persisted watermark", watermark, ok)
	}
}
```

Expected: FAIL，方法未定义。

- [ ] **Step 2: 实现本地持久化**

在 catalog 增加内存 map：

```go
downsamplePolicies  map[string]model.DownsamplePolicy
downsampleWatermark map[string]model.DownsampleWatermark
```

新增 `downsample.bin`，使用二进制 envelope 编码 policy 和 watermark。写入流程使用临时文件、fsync、rename、父目录 fsync，权限遵循 `0600`。

- [ ] **Step 3: 扩展 MetadataStore**

新增接口：

```go
type DownsampleMetadataStore interface {
	UpsertDownsamplePolicy(context.Context, model.DownsamplePolicy) error
	DropDownsamplePolicy(context.Context, string) error
	ListDownsamplePolicies(context.Context) ([]model.DownsamplePolicy, error)
	DownsampleWatermark(context.Context, string) (model.DownsampleWatermark, bool, error)
	UpdateDownsampleWatermark(context.Context, model.DownsampleWatermark) error
}
```

`MetadataStore` 嵌入该接口，`LocalMetadataStore` 转发到 catalog。

- [ ] **Step 4: 验证**

运行：

```bash
timeout 180s go test ./internal/catalog ./internal/engine -run 'Test.*Downsample.*Metadata|TestCatalogDownsample' -count=1 -timeout 3m
```

Expected: PASS。

## Task 3: Engine 与 public API

**状态:** 待执行。

**EARS:**
- When 用户通过 public API 创建、禁用、启用或删除 policy 时，系统应更新本地 metadata。
- When policy 被禁用时，List 返回的 policy 应保留但 `Enabled=false`。

**Files:**
- Modify: `internal/engine/downsample_metadata.go`
- Modify: `types.go`
- Create: `downsample.go`
- Test: `downsample_test.go`
- Test: `internal/engine/downsample_api_test.go`

- [ ] **Step 1: 写 public API 失败测试**

```go
func TestDownsamplePolicyPublicAPIPersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	eng, err := Open(ctx, Options{Path: dir})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	policy := DownsamplePolicy{
		Name: "cpu_1m",
		SourceDatabase: "default", SourceRetention: "autogen", SourceMeasurement: "cpu",
		TargetDatabase: "default", TargetRetention: "rp_1m", TargetMeasurement: "cpu",
		Interval: time.Minute,
		Functions: []DownsampleFunction{{Function: "avg", Field: "usage"}},
		Delay: time.Minute,
		RefreshInterval: time.Minute,
		Lookback: 3 * time.Minute,
		Enabled: true,
	}
	if err := eng.CreateDownsamplePolicy(ctx, policy); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	if err := eng.DisableDownsamplePolicy(ctx, "cpu_1m"); err != nil {
		t.Fatalf("DisableDownsamplePolicy() error = %v", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	reopened, err := Open(ctx, Options{Path: dir})
	if err != nil {
		t.Fatalf("Open(reopen) error = %v", err)
	}
	defer func() {
		if err := reopened.Close(ctx); err != nil {
			t.Fatalf("Close(reopened) error = %v", err)
		}
	}()
	policies, err := reopened.ListDownsamplePolicies(ctx)
	if err != nil {
		t.Fatalf("ListDownsamplePolicies() error = %v", err)
	}
	if len(policies) != 1 || policies[0].Enabled {
		t.Fatalf("policies = %#v, want disabled policy", policies)
	}
}
```

Expected: FAIL，public API 未定义。

- [ ] **Step 2: 实现 public 类型和转换**

在 `types.go` 暴露 `DownsamplePolicy`、`DownsampleFunction`、`DownsampleWatermark`，并实现 `toModelDownsamplePolicy`、`fromModelDownsamplePolicy`。

- [ ] **Step 3: 实现 Engine API**

Engine 方法调用 `normalizeDownsamplePolicy` 后写 metadata：

```go
func (e *Engine) CreateDownsamplePolicy(ctx context.Context, policy model.DownsamplePolicy) error {
	normalized, err := normalizeDownsamplePolicy(policy)
	if err != nil {
		return err
	}
	return e.metadata.UpsertDownsamplePolicy(ctx, normalized)
}
```

Public package 包装内部 Engine 方法。

- [ ] **Step 4: 验证**

```bash
timeout 180s go test . ./internal/engine -run 'TestDownsamplePolicyPublicAPI|Test.*Downsample.*API' -count=1 -timeout 3m
```

Expected: PASS。

## Task 4: Window Planner 与 Executor

**状态:** 待执行。

**EARS:**
- When 当前时间未超过 `delay` 后的完整窗口边界时，executor 不应处理半窗口。
- When 查询源窗口为空时，executor 不应写入目标点。
- When 聚合结果存在时，executor 应写入目标 database、retention policy 和 measurement。

**Files:**
- Create: `internal/engine/downsample_executor.go`
- Test: `internal/engine/downsample_executor_test.go`

- [ ] **Step 1: 写失败测试**

```go
func TestDownsampleExecutorWritesAggregatedPoints(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{Path: t.TempDir(), ShardDuration: time.Hour})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	for minute := int64(0); minute < 5; minute++ {
		if err := eng.Write(ctx, []model.Point{{
			Database: "metrics", RetentionPolicy: "autogen", Measurement: "cpu",
			Tags: map[string]string{"host": "a"},
			Timestamp: minute * int64(time.Minute),
			Fields: map[string]model.FieldValue{"usage": model.Float64Value(float64(minute))},
		}}, model.WriteOptions{}); err != nil {
			t.Fatalf("Write(raw) error = %v", err)
		}
	}
	policy := mustNormalizeDownsamplePolicyForTest(t, model.DownsamplePolicy{
		Name: "cpu_5m",
		SourceDatabase: "metrics", SourceRetention: "autogen", SourceMeasurement: "cpu",
		TargetDatabase: "metrics", TargetRetention: "rp_5m", TargetMeasurement: "cpu",
		Interval: 5 * time.Minute,
		Functions: []model.DownsampleFunction{{Function: "avg", Field: "usage"}},
		GroupByTags: []string{"host"},
		Delay: time.Minute,
		RefreshInterval: time.Minute,
		Lookback: 5 * time.Minute,
		Enabled: true,
	})
	result, err := eng.runDownsampleWindow(ctx, policy, 0, int64(5*time.Minute))
	if err != nil {
		t.Fatalf("runDownsampleWindow() error = %v", err)
	}
	if result.PointsWritten != 1 {
		t.Fatalf("PointsWritten = %d, want 1", result.PointsWritten)
	}
	rows, err := eng.QueryRows(ctx, model.Query{
		Database: "metrics", RetentionPolicy: "rp_5m", Measurement: "cpu",
		Fields: []string{"avg_usage"}, StartTime: 0, EndTime: int64(5*time.Minute),
	})
	if err != nil {
		t.Fatalf("QueryRows(target) error = %v", err)
	}
	if len(rows) != 1 || rows[0].Fields["avg_usage"].Float64 != 2 {
		t.Fatalf("rows = %#v, want avg_usage=2", rows)
	}
}
```

Expected: FAIL，executor 未定义。

- [ ] **Step 2: 实现窗口执行**

实现 `runDownsampleWindow(ctx, policy, start, end)`：

- 构造 `model.Query`，设置 source、time range、aggregates、group tags、group window。
- 调用 `QueryColumns`。
- 将 `ColumnSeries` 转换为 `model.Point`。
- 通过 `Write(ctx, points, model.WriteOptions{})` 写入 target。

- [ ] **Step 3: 验证**

```bash
timeout 180s go test ./internal/engine -run 'TestDownsampleExecutor' -count=1 -timeout 3m
```

Expected: PASS。

## Task 5: Watermark、Refresh 与手动运行

**状态:** 待执行。

**EARS:**
- When 窗口执行成功时，系统应推进 watermark。
- When 任一窗口执行失败时，系统应保留原 watermark。
- When lookback 覆盖已完成窗口时，系统应重新执行 refresh 窗口。

**Files:**
- Modify: `internal/engine/downsample_executor.go`
- Test: `internal/engine/downsample_watermark_test.go`

- [ ] **Step 1: 写失败测试**

```go
func TestRunDownsamplePolicyAdvancesWatermarkAndRefreshesLookback(t *testing.T) {
	ctx := context.Background()
	eng := openEngineWithRawDownsampleSamples(t)
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	policy := testDownsamplePolicy()
	if err := eng.CreateDownsamplePolicy(ctx, policy); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	result, err := eng.RunDownsamplePolicy(ctx, "cpu_1m", 5*time.Minute)
	if err != nil {
		t.Fatalf("RunDownsamplePolicy() error = %v", err)
	}
	if result.WindowsProcessed == 0 {
		t.Fatalf("WindowsProcessed = 0, want at least one window")
	}
	watermark, ok, err := eng.metadata.DownsampleWatermark(ctx, "cpu_1m")
	if err != nil || !ok {
		t.Fatalf("DownsampleWatermark() = %#v ok=%v err=%v", watermark, ok, err)
	}
	if watermark.CompletedUntilUnix <= 0 {
		t.Fatalf("CompletedUntilUnix = %d, want advanced", watermark.CompletedUntilUnix)
	}
}
```

Expected: FAIL。

- [ ] **Step 2: 实现运行计划**

新增：

```go
type DownsampleRunResult struct {
	WindowsProcessed int
	PointsWritten    int
	StartedUnix      int64
	CompletedUnix    int64
	Error             string
}
```

实现 `downsampleWindowsToRun(policy, watermark, now)`，返回 refresh windows 和 new windows，按时间升序执行。

- [ ] **Step 3: 验证**

```bash
timeout 180s go test ./internal/engine -run 'TestRunDownsamplePolicy|Test.*Watermark' -count=1 -timeout 3m
```

Expected: PASS。

## Task 6: Scheduler 生命周期

**状态:** 待执行。

**EARS:**
- When Engine 打开时，scheduler 应启动并扫描 enabled policy。
- When Engine 关闭时，scheduler 应停止并等待任务退出。
- When 同一 policy 正在运行时，scheduler 不应并发启动第二个任务。

**Files:**
- Create: `internal/engine/downsample_scheduler.go`
- Modify: `internal/engine/engine.go`
- Test: `internal/engine/downsample_scheduler_test.go`

- [ ] **Step 1: 写失败测试**

```go
func TestDownsampleSchedulerRunsEnabledPolicyAndStopsOnClose(t *testing.T) {
	ctx := context.Background()
	eng := openEngineWithRawDownsampleSamples(t)
	policy := testDownsamplePolicy()
	policy.RefreshInterval = 10 * time.Millisecond
	policy.Delay = time.Minute
	if err := eng.CreateDownsamplePolicy(ctx, policy); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("CreateDownsamplePolicy() error = %v close = %v", err, closeErr)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rows, err := eng.QueryRows(ctx, model.Query{
			Database: "metrics", RetentionPolicy: "rp_1m", Measurement: "cpu",
			Fields: []string{"avg_usage"}, StartTime: 0, EndTime: int64(5*time.Minute),
		})
		if err == nil && len(rows) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
```

Expected: FAIL。

- [ ] **Step 2: 实现 scheduler**

Engine 增加：

```go
downsampleStopOnce sync.Once
downsampleStop     chan struct{}
downsampleWG       sync.WaitGroup
downsampleRunning  map[string]struct{}
```

`Open` 调用 `startDownsampleScheduler()`，`Close` 调用 `stopDownsampleScheduler()`。

- [ ] **Step 3: 验证**

```bash
timeout 180s go test ./internal/engine -run 'TestDownsampleScheduler' -count=1 -timeout 3m
```

Expected: PASS。

## Task 7: Metrics、Health 与 Runbook

**状态:** 待执行。

**EARS:**
- When downsample policy 运行成功或失败时，系统应暴露运行次数、失败次数、处理窗口数、写入点数和 watermark。
- When 运维人员排查降采样异常时，文档应说明 policy、watermark、delay、lookback 和目标 retention policy 的检查方法。

**Files:**
- Modify: `internal/model/types.go`
- Modify: `internal/engine/metrics.go`
- Create: `internal/engine/downsample_stats.go`
- Modify: `internal/observability/prometheus_text.go`
- Create: `docs/storage/downsample-runbook.md`
- Test: `internal/engine/downsample_stats_test.go`
- Test: `internal/observability/prometheus_text_test.go`

- [ ] **Step 1: 写失败测试**

```go
func TestDownsampleStatsExposeWatermarkAndFailures(t *testing.T) {
	var stats downsampleStatsRecorder
	stats.recordSuccess("cpu_1m", 2, 10, time.Second, int64(2*time.Minute))
	stats.recordFailure("cpu_1m", errors.New("query failed"))
	snapshot := stats.snapshot()
	if snapshot.RunsTotal != 2 || snapshot.FailuresTotal != 1 ||
		snapshot.WindowsTotal != 2 || snapshot.PointsWrittenTotal != 10 {
		t.Fatalf("snapshot = %#v, want counters", snapshot)
	}
}
```

Expected: FAIL。

- [ ] **Step 2: 实现指标**

新增 `model.DownsampleStats`，Engine metrics snapshot 合并 downsample counters。Prometheus 输出指标名使用 `mts_downsample_*`。

- [ ] **Step 3: 写 runbook**

文档覆盖：

- 创建 policy 的推荐参数。
- 如何判断 watermark 落后。
- 如何处理 target retention policy 无数据。
- 如何处理 late data 和 lookback。
- 如何禁用 policy。

- [ ] **Step 4: 验证**

```bash
timeout 180s go test ./internal/engine ./internal/observability -run 'TestDownsampleStats|TestPrometheus.*Downsample' -count=1 -timeout 3m
```

Expected: PASS。

## Task 8: E2E、Fault 与 Scale Smoke

**状态:** 待执行。

**EARS:**
- When raw 数据写入后运行 downsample policy，目标 retention policy 应可查询正确聚合结果。
- When target 写入失败时，watermark 不应推进。
- When 100K raw 点运行降采样时，测试应输出耗时、RSS 和写入点数。

**Files:**
- Create: `tests/e2e/downsample_policy/main.go`
- Create: `tests/fault/downsample_policy/main.go`
- Create: `tests/scale/downsample_policy/main.go`
- Test: `tests/e2e/downsample_policy`
- Test: `tests/fault/downsample_policy`
- Test: `tests/scale/downsample_policy`

- [ ] **Step 1: 写 e2e 用例**

`tests/e2e/downsample_policy/main.go` 执行：

1. 创建 Engine。
2. 写入 10 分钟 raw 数据。
3. 创建 1 分钟 policy。
4. 手动运行 policy。
5. 查询 `rp_1m`。
6. 校验 `avg_usage`、`max_usage`、`count_usage`。

- [ ] **Step 2: 写 fault 用例**

使用 faultinject 让 metadata watermark save 或 target write 失败，断言 `CompletedUntilUnix` 不推进，错误可见。

- [ ] **Step 3: 写 scale smoke**

参数：

```text
points=100000
series=100
interval=1m
functions=avg,min,max,count,last
```

报告 JSON 字段：

```json
{
  "points": 100000,
  "windows_processed": 0,
  "points_written": 0,
  "duration_nanos": 0,
  "rss_peak_bytes": 0
}
```

- [ ] **Step 4: 验证**

```bash
timeout 600s go test ./tests/e2e/downsample_policy ./tests/fault/downsample_policy ./tests/scale/downsample_policy -count=1 -timeout 10m
```

Expected: PASS。

## Task 9: 全量质量门禁

**状态:** 待执行。

**EARS:**
- When 全量 CI gate 执行时，downsample 新能力应通过 format、test、lint、coverage 和 artifact scan。
- When coverage 低于核心包 90% 时，系统应补齐测试而不是降低门槛。

**Files:**
- Modify: `scripts/ci_gate.sh` if new packages need explicit smoke coverage.
- Update: `docs/query/builder-aggregate-functions.md`
- Update: `docs/storage/operations-runbook.md`

- [ ] **Step 1: 文档更新**

更新聚合文档，移除 `downsample` 稳定拒绝描述，说明降采样是 policy 能力，不是 aggregate function。

- [ ] **Step 2: 运行格式化**

```bash
timeout 300s goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .
```

Expected: exit 0。

- [ ] **Step 3: 运行全量测试**

```bash
timeout 600s go test ./... -count=1 -timeout 10m
```

Expected: PASS。

- [ ] **Step 4: 运行 lint**

```bash
timeout 720s golangci-lint run ./...
```

Expected: `0 issues.`

- [ ] **Step 5: 运行 coverage**

```bash
timeout 720s go test ./... -cover -count=1 -timeout 10m
```

Expected: 核心包覆盖率 >= 90%。

- [ ] **Step 6: 运行 CI gate**

```bash
timeout 900s bash scripts/ci_gate.sh
```

Expected: PASS。

- [ ] **Step 7: 扫描临时产物**

```bash
timeout 60s find . -type f \( -name '*.test' -o -name '*.prof' -o -name '*.pprof' -o -name 'coverage.out' -o -name '*.coverprofile' \) -not -path './.git/*' -print
```

Expected: 无输出。

## Self-Review

- Spec coverage：Task 1-9 覆盖模型、校验、metadata、Engine API、executor、watermark、scheduler、metrics、runbook、e2e、fault、scale 和 CI gate。
- Placeholder scan：本文不使用未决占位标记。
- Type consistency：所有计划中的类型以 `model.DownsamplePolicy`、`model.DownsampleFunction`、`model.DownsampleWatermark` 为准；public package 通过转换函数暴露等价类型。
