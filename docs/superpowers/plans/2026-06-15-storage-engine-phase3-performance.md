# Storage Engine Phase 3 Performance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce write-path allocation and CPU overhead for wide time-series points by adding true batch write paths, low-peak MemTable flush, and wide10 benchmark/pprof coverage.

**Architecture:** Preserve the public `Engine.Write` API and existing WAL/SSTable binary formats. Internally resolve points in batches, group resolved points by shard, append one WAL batch per shard group, apply each group to MemTable under one lock, and swap MemTable data during flush instead of cloning.

**Tech Stack:** Go 1.26.2, standard library only for production code, `go test`, `pprof`, `goimports-reviser`, `gofmt`, `golangci-lint`.

---

## Constraints And Budgets

- 预计耗时：6-10 小时；硬超时：24 小时。
- 单包测试超时：180s；全量测试超时：600s；benchmark 超时：900s；lint 超时：12m。
- 不改变公开 API，不改变 WAL/SSTable 文件格式。
- 新增目录权限保持 `0700`，新增文件权限保持 `0600`。
- 禁止在 for 循环中使用 defer。
- 所有新增行为先写失败测试，再实现。
- 每完成一个 task，更新本计划 checkbox 和实现备注。

## File Structure

- Modify: `internal/catalog/catalog.go` - add `ResolvePoints` batch API.
- Modify: `internal/catalog/resolve.go` - share locked resolve helpers for single and batch paths.
- Modify: `internal/catalog/catalog_test.go` - add batch resolve equivalence and error tests.
- Modify: `internal/memtable/memtable.go` - add `ApplyBatch`, `Restore`, and swap-based `SnapshotAndReset`.
- Modify: `internal/memtable/memtable_test.go` - add batch apply and snapshot isolation tests.
- Modify: `internal/engine/engine.go` - rewrite `Write` to use batch resolve and shard grouping.
- Modify: `internal/engine/shard.go` - add `WriteBatch`, restore snapshot on flush failure.
- Modify: `internal/engine/engine_test.go` - add WAL frame count and flush failure visibility tests.
- Modify: `internal/bench/storage_bench_test.go` - add wide10 write benchmark.
- Modify: `tests/pprof/storage_engine/main.go` - add `-field-layout=default|wide10`.
- Modify: `tests/pprof/storage_engine/main_test.go` - add wide10 config and workload tests.
- Modify: `internal/sstable/read.go` - lazy-load index rows on query.
- Modify: `internal/sstable/internal_test.go` - assert bad index block fails at query time.
- Modify: `docs/benchmarks/storage-engine-phase2.md` or create `docs/benchmarks/storage-engine-phase3.md` - record baseline and optimization results.

## Task 1: MemTable Batch Apply And Swap Snapshot

**Files:**
- Modify: `internal/memtable/memtable.go`
- Modify: `internal/memtable/memtable_test.go`

- [x] **Step 1: Add failing tests for batch apply and swap isolation**

Add tests to `internal/memtable/memtable_test.go`:

```go
func TestApplyBatchMatchesApply(t *testing.T) {
	points := []model.ResolvedPoint{
		resolvedPoint(1, 10, 1, model.Float64Value(1)),
		resolvedPoint(1, 11, 2, model.Float64Value(2)),
		resolvedPoint(2, 10, 3, model.Int64Value(3)),
	}
	oneByOne := New()
	for _, point := range points {
		if err := oneByOne.Apply(point); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
	}
	batched := New()
	if err := batched.ApplyBatch(points); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}
	want := oneByOne.Snapshot().Query(Query{Start: 0, End: 10})
	got := batched.Snapshot().Query(Query{Start: 0, End: 10})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ApplyBatch() columns = %#v, want %#v", got, want)
	}
}

func TestSnapshotAndResetSwapsDataAndKeepsSnapshotStable(t *testing.T) {
	mem := New()
	if err := mem.Apply(resolvedPoint(1, 10, 1, model.Float64Value(1))); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	snapshot := mem.SnapshotAndReset()
	if mem.SampleCount() != 0 {
		t.Fatalf("SampleCount() after reset = %d, want 0", mem.SampleCount())
	}
	if err := mem.Apply(resolvedPoint(1, 10, 2, model.Float64Value(2))); err != nil {
		t.Fatalf("Apply() second error = %v", err)
	}
	got := snapshot.Query(Query{Start: 0, End: 10})
	if len(got) != 1 || got[0].Samples[0].Value.Float64 != 1 {
		t.Fatalf("old snapshot changed after new write: %#v", got)
	}
}
```

- [x] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./internal/memtable -run 'TestApplyBatchMatchesApply|TestSnapshotAndResetSwapsDataAndKeepsSnapshotStable' -timeout 180s
```

Expected: FAIL because `ApplyBatch` is not defined.

- [x] **Step 3: Implement MemTable batch and restore helpers**

Implement:

```go
func (m *MemTable) ApplyBatch(points []model.ResolvedPoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, point := range points {
		for _, field := range point.Fields {
			if m.applyField(point, field) {
				m.sampleCount++
			}
		}
	}
	return nil
}

func (m *MemTable) SnapshotAndReset() *Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := &Snapshot{data: m.data, sampleCount: m.sampleCount}
	m.data = make(tableData)
	m.sampleCount = 0
	return snapshot
}

func (m *MemTable) Restore(snapshot *Snapshot) {
	if snapshot == nil || snapshot.sampleCount == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for seriesID, fields := range snapshot.data {
		dstFields := ensureFields(m.data, seriesID)
		for fieldID, samples := range fields {
			dstSamples := ensureSamples(dstFields, fieldID)
			for timestamp, sample := range samples {
				if mergeSample(dstSamples, timestamp, sample) {
					m.sampleCount++
				}
			}
		}
	}
}
```

- [x] **Step 4: Run memtable package**

Run:

```bash
go test ./internal/memtable -timeout 180s
```

Expected: PASS.

实现备注：已新增 `ApplyBatch`、`Restore` 和 swap 版 `SnapshotAndReset`。`go test ./internal/memtable -run 'TestApplyBatchMatchesApply|TestSnapshotAndResetKeepsSnapshotStable' -timeout 180s` 先按预期因 `ApplyBatch` 缺失失败；实现后 `go test ./internal/memtable -timeout 180s` 通过。

## Task 2: Catalog Batch Resolve

**Files:**
- Modify: `internal/catalog/catalog.go`
- Modify: `internal/catalog/catalog_test.go`

- [x] **Step 1: Add failing tests for batch resolve**

Add tests:

```go
func TestResolvePointsMatchesResolvePoint(t *testing.T) {
	dir := t.TempDir()
	cat, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	points := []model.Point{
		catalogPoint("cpu", "host-a", 1),
		catalogPoint("cpu", "host-a", 2),
		catalogPoint("cpu", "host-b", 3),
	}
	got, err := cat.ResolvePoints(points)
	if err != nil {
		t.Fatalf("ResolvePoints() error = %v", err)
	}
	if len(got) != len(points) {
		t.Fatalf("ResolvePoints() len = %d, want %d", len(got), len(points))
	}
	if got[0].SeriesID != got[1].SeriesID {
		t.Fatalf("same tags got different series ids: %d vs %d", got[0].SeriesID, got[1].SeriesID)
	}
	if got[0].SeriesID == got[2].SeriesID {
		t.Fatalf("different tags got same series id: %d", got[0].SeriesID)
	}
}

func TestResolvePointsRejectsInvalidBeforePartialResult(t *testing.T) {
	dir := t.TempDir()
	cat, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	_, err = cat.ResolvePoints([]model.Point{{Measurement: "", Fields: map[string]model.FieldValue{"v": model.Float64Value(1)}}})
	if !errors.Is(err, ErrEmptyMeasurement) {
		t.Fatalf("ResolvePoints() error = %v, want ErrEmptyMeasurement", err)
	}
}
```

- [x] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./internal/catalog -run 'TestResolvePoints' -timeout 180s
```

Expected: FAIL because `ResolvePoints` is not defined.

- [x] **Step 3: Implement `ResolvePoints`**

Implement one-lock batch resolve:

```go
func (c *Catalog) ResolvePoints(points []model.Point) ([]model.ResolvedPoint, error) {
	for _, point := range points {
		if err := validatePoint(point); err != nil {
			return nil, err
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	resolved := make([]model.ResolvedPoint, 0, len(points))
	for _, point := range points {
		series, err := c.resolveSeriesLocked(point.Measurement, point.Tags)
		if err != nil {
			return nil, err
		}
		fields, err := c.resolveFieldsLocked(point.Measurement, point.Fields)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, model.ResolvedPoint{
			Database: point.Database, RetentionPolicy: point.RetentionPolicy,
			Measurement: point.Measurement, Tags: cloneTags(point.Tags),
			SeriesID: series.ID, Timestamp: point.Timestamp, Fields: fields,
		})
	}
	return resolved, nil
}
```

- [x] **Step 4: Run catalog package**

Run:

```bash
go test ./internal/catalog -timeout 180s
```

Expected: PASS.

实现备注：已新增 `Catalog.ResolvePoints`，先验证整个 batch，再在一个锁内 resolve。`go test ./internal/catalog -run 'TestCatalogResolvePoints' -timeout 180s` 先按预期因方法缺失失败；实现后 `go test ./internal/catalog -timeout 180s` 通过。

## Task 3: Engine And Shard Batch Write

**Files:**
- Modify: `internal/engine/engine.go`
- Modify: `internal/engine/shard.go`
- Modify: `internal/engine/engine_test.go`

- [x] **Step 1: Add failing engine tests**

Add tests:

```go
func TestEngineWriteBatchUsesSingleWALFramePerShard(t *testing.T) {
	dir := t.TempDir()
	eng, err := Open(context.Background(), model.Options{Path: dir, ShardDuration: time.Hour, MemTableMaxSamples: 1000})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := make([]model.Point, 0, 10)
	for index := range 10 {
		points = append(points, testPoint("batch", int64(index)))
	}
	if err := eng.Write(context.Background(), points, model.WriteOptions{}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := eng.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	walPath := filepath.Join(dir, "data", "default", "autogen", "shards", "0", "wal", "000001.wal")
	frames, err := countWALFrames(walPath)
	if err != nil {
		t.Fatalf("countWALFrames() error = %v", err)
	}
	if frames != 1 {
		t.Fatalf("wal frames = %d, want 1", frames)
	}
}

func TestShardFlushFailureRestoresSnapshot(t *testing.T) {
	shard := openTestShard(t, ShardOptions{Dir: t.TempDir(), Database: "default", RetentionPolicy: "autogen", Start: 0, End: int64(time.Hour), MemTableMaxSamples: 1})
	shard.testHooks.afterPartWriteBeforeManifest = func() error { return errors.New("inject manifest failure") }
	point := resolvedEnginePoint(1, 1, model.Float64Value(1))
	err := shard.WriteBatch([]model.ResolvedPoint{point}, false)
	if err == nil {
		t.Fatal("WriteBatch() error = nil, want injected failure")
	}
	columns, queryErr := shard.Query(memtable.Query{Start: 0, End: 1})
	if queryErr != nil {
		t.Fatalf("Query() error = %v", queryErr)
	}
	if len(columns) == 0 {
		t.Fatal("Query() returned no columns after failed flush")
	}
}
```

- [x] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./internal/engine -run 'TestEngineWriteBatchUsesSingleWALFramePerShard|TestShardFlushFailureRestoresSnapshot' -timeout 180s
```

Expected: FAIL because writes are still one WAL frame per point and `WriteBatch` is missing.

- [x] **Step 3: Implement batch grouping**

Implement:

```go
func (e *Engine) Write(_ context.Context, points []model.Point, opts model.WriteOptions) error {
	if len(points) == 0 {
		return nil
	}
	normalized := make([]model.Point, len(points))
	for index, point := range points {
		normalized[index] = normalizePoint(e.opts, point)
	}
	resolved, err := e.catalog.ResolvePoints(normalized)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	groups := make(map[*Shard][]model.ResolvedPoint)
	for index := range resolved {
		e.writeSeq++
		resolved[index].WriteSeq = e.writeSeq
		shard, err := e.shardForLocked(resolved[index].Database, resolved[index].RetentionPolicy, resolved[index].Timestamp)
		if err != nil {
			return err
		}
		groups[shard] = append(groups[shard], resolved[index])
	}
	for shard, batch := range groups {
		if err := shard.WriteBatch(batch, opts.Sync); err != nil {
			return err
		}
	}
	return nil
}
```

Add `Shard.WriteBatch` and make `Write` delegate to it.

- [x] **Step 4: Restore snapshot on flush failure**

Change `flushLocked` so that after `SnapshotAndReset`, any Part/manifest/open failure restores the snapshot before returning. WAL truncation must still occur only after manifest commit and hooks succeed.

- [x] **Step 5: Run engine package**

Run:

```bash
go test ./internal/engine -timeout 180s
```

Expected: PASS.

实现备注：已将 `Engine.Write` 改为批量 resolve 并按 shard 分组；新增 `Shard.WriteBatch`，一次 WAL append 后批量 apply MemTable；flush 失败时调用 `MemTable.Restore` 恢复 swap 出来的 snapshot。定向测试先因 `WriteBatch` 缺失失败；实现后补充同 shard 单 WAL frame、跨 shard 分组和 flush 失败恢复测试，`go test ./internal/engine -timeout 180s` 通过。

## Task 4: Wide10 Benchmark And Pprof Workload

**Files:**
- Modify: `internal/bench/storage_bench_test.go`
- Modify: `tests/pprof/storage_engine/main.go`
- Modify: `tests/pprof/storage_engine/main_test.go`

- [x] **Step 1: Add tests for field layout parsing and wide10 point**

Add tests to `tests/pprof/storage_engine/main_test.go`:

```go
func TestWide10WorkloadPointHasExpectedFields(t *testing.T) {
	point := workloadPoint(7, 3, fieldLayoutWide10)
	if len(point.Fields) != 10 {
		t.Fatalf("field count = %d, want 10", len(point.Fields))
	}
	counts := countFieldTypes(point.Fields)
	if counts[mts.FieldFloat64] != 5 || counts[mts.FieldInt64] != 3 ||
		counts[mts.FieldString] != 1 || counts[mts.FieldBool] != 1 {
		t.Fatalf("field type counts = %#v", counts)
	}
}
```

- [x] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./tests/pprof/storage_engine -run TestWide10WorkloadPointHasExpectedFields -timeout 180s
```

Expected: FAIL because field layout support is not implemented.

- [x] **Step 3: Implement `-field-layout`**

Add to config:

```go
fieldLayout string
```

Add constants:

```go
const (
	fieldLayoutDefault = "default"
	fieldLayoutWide10  = "wide10"
)
```

Parse and validate `-field-layout`, update write/query/compact/replay paths to call `workloadPoint(index, cfg.series, cfg.fieldLayout)`.

- [x] **Step 4: Add wide10 benchmark**

Add `BenchmarkEngineWriteWideBatch` using 10-field points and sizes `1000`, `10000`.

- [x] **Step 5: Run pprof tests and benchmark smoke**

Run:

```bash
go test ./tests/pprof/storage_engine -timeout 180s
go test ./internal/bench -bench='BenchmarkEngineWrite(Batch|WideBatch)$' -benchmem -count=1 -timeout 900s
```

Expected: PASS with benchmark rows for normal and wide batch writes.

实现备注：`tests/pprof/storage_engine` 已新增 `-field-layout=default|wide10`，默认兼容旧 4 字段布局；`wide10` 生成 5 float、3 int、1 string、1 bool。`internal/bench` 新增 `BenchmarkEngineWriteWideBatch`。随后增加 WAL batch 精确容量预估和 Engine 同 shard 分组复用 resolved slice，benchmark smoke 中 4 字段 10K 降至约 `34.1ms/op, 45.5MB/op, 153033 allocs/op`，wide10 10K 降至约 `67.6ms/op, 90.4MB/op, 195165 allocs/op`。进一步将 SSTable Part index rows 改为查询懒加载，100K wide10 写入后的 in-use heap 从约 `40MB` 降至约 `12.8MB`。

## Task 5: Benchmark, Quality Gate, And Documentation

**Files:**
- Create: `docs/benchmarks/storage-engine-phase3.md`
- Modify: `docs/superpowers/plans/2026-06-15-storage-engine-phase3-performance.md`

- [x] **Step 1: Run focused benchmarks**

Run:

```bash
go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s
```

Expected: PASS. Record key rows in benchmark doc.

- [x] **Step 2: Run wide10 pprof smoke**

Run:

```bash
go run ./tests/pprof/storage_engine -mode=write -field-layout=wide10 -points 100000 -series 1000 -mem-profile /tmp/mts-wide10-mem.prof
go tool pprof -alloc_space -top /tmp/mts-wide10-mem.prof
rm -f /tmp/mts-wide10-mem.prof
```

Expected: command completes, profile is readable, temp profile is removed.

- [x] **Step 3: Run full tests with coverage**

Run:

```bash
go test ./... -coverprofile=coverage.out -timeout 600s
go tool cover -func=coverage.out | tail -1
rm -f coverage.out
```

Expected: PASS and total coverage `>=90.0%`.

- [x] **Step 4: Run formatting and lint**

Run:

```bash
goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .
gofmt -w $(git ls-files '*.go')
golangci-lint run --timeout 12m
```

Expected: all commands pass.

- [x] **Step 5: Run e2e build/run and clean binaries**

Run each e2e directory:

```bash
for dir in tests/e2e/*; do
  [ -d "$dir" ] || continue
  name="$(basename "$dir")"
  (cd "$dir" && go build -o "$name" . && "./$name")
  rm -f "$dir/$name"
done
```

Expected: all e2e cases pass and no e2e binary remains.

- [x] **Step 6: Verify no generated artifacts remain**

Run:

```bash
git status --short
find . -maxdepth 4 \( -name '*.prof' -o -name 'coverage.out' -o -perm -111 \) -type f -print
```

Expected: only intended source/doc changes appear in git status; no profile, coverage, or test binary artifacts remain.

实现备注：最终 benchmark 已写入 `docs/benchmarks/storage-engine-phase3.md`。`go run ./tests/pprof/storage_engine -mode=write -field-layout=wide10 -points 100000 -series 1000 -mem-profile /tmp/mts-wide10-mem.prof` 通过；lazy index 后 `inuse_space` 约 `12.8MB`，`alloc_space` 约 `1696MB`，profile 已删除。`go test ./... -coverprofile=coverage.out -timeout 600s` 通过，总覆盖率 `90.0%`，`coverage.out` 已删除。`goimports-reviser`、`gofmt`、`golangci-lint run --timeout 12m` 通过，lint 输出 `0 issues.`。`tests/e2e` 全量 build/run 通过并清理二进制；产物扫描仅显示 `.git/hooks/*.sample`，无本次生成的 profile、coverage 或 e2e binary 残留。

## Self Review

- Spec coverage: tasks cover batch write, WAL batch frame reduction, MemTable batch apply, swap snapshot, wide10 benchmark/pprof, documentation, and final quality gate.
- Placeholder scan: no TBD/TODO placeholders.
- Type consistency: new public/internal names are `ResolvePoints`, `ApplyBatch`, `Restore`, `WriteBatch`, and `fieldLayout`.
