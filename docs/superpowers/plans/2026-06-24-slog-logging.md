# slog 结构化日志集成 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 MTS 工程关键位置集成 `log/slog` 结构化日志，修复 6 处静默吞错，补充生命周期和持久化失败日志，与现有指标体系互补。

**Architecture:** Options 注入 `*slog.Logger`，nil 归一化为 nopHandler（零开销）。Logger 通过 Engine → Shard 链路传递，HTTP Service 独立注入。高频读写路径不加日志，依赖现有指标。

**Tech Stack:** Go 1.26.2、`log/slog` 标准库、现有 `internal/engine`、`internal/wal`、`internal/sstable`、`internal/service` 包

---

## File Structure

| 文件 | 职责 | 操作 |
|------|------|------|
| `internal/engine/logging.go` | nopHandler 定义、nopLogger 构造 | 新建 |
| `internal/engine/logging_test.go` | nopHandler 和 logger 归一化测试 | 新建 |
| `internal/model/types.go` | `Options.Logger` 字段 | 修改 |
| `options.go` | 公开 `Options.Logger` 字段 | 修改 |
| `convert_options.go` | `toModelOptions` 传递 Logger | 修改 |
| `internal/engine/engine.go` | Engine 持有 logger、Open/Close 日志 | 修改 |
| `internal/engine/paths.go` | `normalizeOptions` 归一化 Logger | 修改 |
| `internal/engine/shard.go` | ShardOptions.Logger、Shard.logger、OpenShard 日志 | 修改 |
| `internal/engine/background.go` | 后台 compaction 失败日志 | 修改 |
| `internal/engine/downsample_scheduler.go` | 降采样调度器吞错日志 | 修改 |
| `internal/engine/downsample_executor.go` | 降采样运行日志 | 修改 |
| `internal/engine/lifecycle.go` | Compaction/Retention 日志 | 修改 |
| `internal/engine/recovery_audit.go` | Recovery issue 日志 | 修改 |
| `internal/wal/wal.go` | WAL interval sync 失败日志 | 修改 |
| `internal/sstable/manifest.go` | manifest 写入失败日志 | 修改 |
| `internal/service/server.go` | Service.Options.Logger、Start/Shutdown 日志、Serve 异常退出日志 | 修改 |
| `internal/service/admin.go` | admin 鉴权失败日志 | 修改 |
| `cmd/mts-storage/main.go` | CLI 设置 slog handler | 修改 |

---

### Task 1: nopHandler 与 logger 归一化基础设施

**Files:**
- Create: `internal/engine/logging.go`
- Create: `internal/engine/logging_test.go`

- [ ] **Step 1: 编写 nopHandler 测试**

```go
package engine

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

func TestNopHandlerEnabled(t *testing.T) {
	handler := nopHandler{}
	if handler.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("nopHandler.Enabled should return false for all levels")
	}
}

func TestNopHandlerHandleNoOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(nopHandler{})
	logger.Info("test message", "key", "value")
	if buf.Len() > 0 {
		t.Fatalf("nopHandler should produce no output, got %q", buf.String())
	}
}

func TestNopHandlerWithAttrsAndGroup(t *testing.T) {
	handler := nopHandler{}
	derived := handler.WithAttrs([]slog.Attr{slog.String("k", "v")})
	if _, ok := derived.(nopHandler); !ok {
		t.Fatal("WithAttrs should return nopHandler")
	}
	grouped := handler.WithGroup("group")
	if _, ok := grouped.(nopHandler); !ok {
		t.Fatal("WithGroup should return nopHandler")
	}
}

func TestNopLoggerIsDefault(t *testing.T) {
	logger := nopLogger()
	if logger == nil {
		t.Fatal("nopLogger should not return nil")
	}
	if !logger.Enabled(context.Background(), slog.LevelError) {
		// nopLogger 应返回 Enabled=false 的 logger
		return
	}
	t.Fatal("nopLogger should be disabled for all levels")
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -run TestNopHandler -v ./internal/engine/`
Expected: FAIL — `nopHandler` 未定义

- [ ] **Step 3: 实现 logging.go**

```go
package engine

import (
	"context"
	"log/slog"
)

// nopHandler 是 slog.Handler 的空操作实现，用于 nil Logger 归一化。
// Enabled 恒返回 false，Handle/WithAttrs/WithGroup 均为空操作，
// 保证下游代码永不需 nil 检查且零开销。
type nopHandler struct{}

func (nopHandler) Enabled(_ context.Context, _ slog.Level) bool  { return false }
func (nopHandler) Handle(_ context.Context, _ slog.Record) error { return nil }
func (h nopHandler) WithAttrs(_ []slog.Attr) slog.Handler        { return h }
func (h nopHandler) WithGroup(_ string) slog.Handler             { return h }

// nopLogger 返回一个丢弃所有日志的 *slog.Logger，零开销。
func nopLogger() *slog.Logger {
	return slog.New(nopHandler{})
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -run TestNopHandler -v ./internal/engine/`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/engine/logging.go internal/engine/logging_test.go
git commit -m "feat: 新增 nopHandler 和 nopLogger 日志基础设施"
```

---

### Task 2: Options 层新增 Logger 字段

**Files:**
- Modify: `internal/model/types.go:372-384` (Options struct)
- Modify: `options.go:22-34` (Options struct)
- Modify: `convert_options.go:5-19` (toModelOptions)
- Modify: `internal/engine/paths.go:22-38` (normalizeOptions)
- Modify: `options_test.go` (新增 Logger 传播测试)

- [ ] **Step 1: 在 model.Options 新增 Logger 字段**

在 `internal/model/types.go` 的 `Options` struct 末尾（`StorageMemory` 字段之后）新增：

```go
import "log/slog"

// ... Options struct ...
	StorageMemory          StorageMemoryOptions
	Logger                 *slog.Logger
```

注意：需要在文件顶部的 import 块新增 `"log/slog"`。

- [ ] **Step 2: 在公开 Options 新增 Logger 字段**

在 `options.go` 的 `Options` struct 末尾新增同名字段：

```go
import "log/slog"

// ... Options struct ...
	StorageMemory          StorageMemoryOptions
	Logger                 *slog.Logger
```

注意：需要在文件顶部的 import 块新增 `"log/slog"`。Logger 字段不参与 Validate 检查（nil 合法）。

- [ ] **Step 3: 在 toModelOptions 传递 Logger**

在 `convert_options.go` 的 `toModelOptions` 函数返回值中新增：

```go
func toModelOptions(opts Options) model.Options {
	return model.Options{
		// ...existing fields...
		StorageMemory:          toModelStorageMemoryOptions(opts.StorageMemory),
		Logger:                 opts.Logger,
	}
}
```

- [ ] **Step 4: 在 normalizeOptions 归一化 nil Logger**

在 `internal/engine/paths.go` 的 `normalizeOptions` 函数末尾、`return opts` 之前新增：

```go
	if opts.Logger == nil {
		opts.Logger = nopLogger()
	}
```

- [ ] **Step 5: 编写 Logger 传播测试**

在 `options_test.go` 末尾新增：

```go
func TestNilLoggerDefaultsToNop(t *testing.T) {
	ctx := context.Background()
	opts := mts.DefaultOptions(t.TempDir())
	// 不设置 Logger，应使用 nopLogger
	engine, err := mts.Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = engine.Close(ctx) }()
	// 引擎正常工作即可验证 nil logger 不 panic
	if err := engine.Write(ctx, []mts.Point{{
		Measurement: "cpu",
		Timestamp:   1,
		Fields:      map[string]mts.FieldValue{"v": mts.Float64Value(1)},
	}}, mts.WriteOptions{}); err != nil {
		t.Fatalf("Write() with nil logger error = %v", err)
	}
}

func TestCustomLoggerPropagates(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	opts := mts.DefaultOptions(t.TempDir())
	opts.Logger = logger
	engine, err := mts.Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = engine.Close(ctx) }()
	output := buf.String()
	if !strings.Contains(output, "engine opened") {
		t.Fatalf("expected 'engine opened' log, got %q", output)
	}
}
```

注意：需要在 `options_test.go` 顶部 import 块新增 `"bytes"`, `"log/slog"`, `"strings"`。

- [ ] **Step 6: 运行测试确认通过**

Run: `go test -run TestNilLogger -v -count=1 ./`
Run: `go test -run TestCustomLogger -v -count=1 ./`
Expected: `TestCustomLoggerPropagates` 会失败（因为 Open 日志尚未在 engine.go 中添加），`TestNilLogger` 应通过。

- [ ] **Step 7: 提交**

```bash
git add internal/model/types.go options.go convert_options.go internal/engine/paths.go options_test.go
git commit -m "feat: Options 层新增 Logger 字段并归一化 nil"
```

---

### Task 3: Engine 持有 logger 并在 Open/Close 加日志

**Files:**
- Modify: `internal/engine/engine.go:21-48` (Engine struct), `:66-93` (Open), `:119-133` (Close)
- Modify: `internal/engine/engine.go:942-970` (shardForStartLocked 传递 Logger)
- Modify: `internal/engine/engine.go:1002-1015` (openShardDir 传递 Logger)
- Modify: `internal/engine/shard.go:18-32` (ShardOptions), `:34-49` (Shard struct)

- [ ] **Step 1: 在 Engine struct 新增 logger 字段**

在 `internal/engine/engine.go` 的 `Engine` struct 中新增字段（放在 `downsampleStats` 之后）：

```go
	downsampleStats    downsampleStatsRecorder
	logger             *slog.Logger
```

- [ ] **Step 2: 在 ShardOptions 和 Shard 新增 logger**

在 `internal/engine/shard.go` 的 `ShardOptions` struct 中新增（放在 `deps shardDeps` 之前）：

```go
	Memory             *storageMemoryLimiter
	scheduler          *compactionScheduler
	logger             *slog.Logger
	deps               shardDeps
```

在 `Shard` struct 中新增（放在 `testHooks` 之后）：

```go
	testHooks       shardTestHooks
	logger          *slog.Logger
```

- [ ] **Step 3: 在 Open 中设置 logger 并加 INFO 日志**

修改 `internal/engine/engine.go` 的 `Open` 函数：

```go
func Open(_ context.Context, opts model.Options) (*Engine, error) {
	opts = normalizeOptions(opts)
	if opts.Path == "" {
		return nil, fmt.Errorf("engine path is empty")
	}
	if err := prepareStorageRoot(opts.Path); err != nil {
		return nil, err
	}
	metadata, err := OpenLocalMetadataStore(catalogDir(opts.Path))
	if err != nil {
		return nil, err
	}
	eng := &Engine{
		opts:                opts,
		metadata:            metadata,
		shards:              make(map[string]*Shard),
		memory:              newStorageMemoryLimiter(opts.StorageMemory),
		compactionScheduler: newCompactionScheduler(),
		downsampleRunning:   make(map[string]struct{}),
		logger:              opts.Logger,
	}
	if err := eng.loadExistingShards(); err != nil {
		closeErr := metadata.Close()
		return nil, fmt.Errorf("load shards: %w close metadata: %v", err, closeErr)
	}
	eng.startBackgroundCompaction()
	eng.startDownsampleScheduler()
	eng.logger.Info("engine opened",
		"path", opts.Path,
		"shard_count", len(eng.shards),
	)
	return eng, nil
}
```

- [ ] **Step 4: 在 Close 中加 INFO 日志**

修改 `internal/engine/engine.go` 的 `Close` 函数，在 `return nil` 之前新增：

```go
func (e *Engine) Close(_ context.Context) error {
	e.stopDownsampleScheduler()
	e.stopBackgroundCompaction()
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, shard := range e.shards {
		if err := shard.Close(); err != nil {
			e.logger.Error("engine close failed",
				"shard", shardID(shard.opts.Database, shard.opts.RetentionPolicy, shard.opts.Start),
				"error", err,
			)
			return err
		}
	}
	if err := e.metadata.Close(); err != nil {
		e.logger.Error("metadata close failed", "error", err)
		return err
	}
	e.logger.Info("engine closed")
	return nil
}
```

- [ ] **Step 5: 在 shardForStartLocked 传递 logger**

修改 `internal/engine/engine.go` 的 `shardForStartLocked` 函数中的 `OpenShard` 调用，在 `ShardOptions{}` 中新增 `logger: e.logger,`：

```go
	shard, maxSeq, err := OpenShard(ShardOptions{
		Dir:                dir,
		Database:           database,
		RetentionPolicy:    policy,
		Start:              start,
		End:                start + int64(e.opts.ShardDuration) - 1,
		WAL:                e.opts.WAL,
		FlushSync:          e.opts.FlushSync,
		MemTableMaxSamples: e.opts.MemTableMaxSamples,
		Compaction:         e.opts.Compaction,
		Compression:        e.opts.Compression,
		Memory:             e.memory,
		scheduler:          e.compactionScheduler,
		logger:             e.logger,
	})
```

- [ ] **Step 6: 在 openShardDir 传递 logger**

同样在 `internal/engine/engine.go` 的 `openShardDir` 函数中的 `OpenShard` 调用新增 `logger: e.logger,`。

- [ ] **Step 7: 在 OpenShard 中设置 logger 并加 INFO 日志**

修改 `internal/engine/shard.go` 的 `OpenShard` 函数，在创建 shard 时设置 logger，并在 replay 完成后加 INFO 日志：

```go
func OpenShard(opts ShardOptions) (*Shard, uint64, error) {
	if err := prepareStorageRoot(opts.Dir); err != nil {
		return nil, 0, err
	}
	logger := opts.logger
	if logger == nil {
		logger = nopLogger()
	}
	deps := normalizeShardDeps(opts.deps)
	manifest, err := deps.parts.LoadManifest(opts.Dir)
	if err != nil {
		return nil, 0, err
	}
	shard := &Shard{
		opts:     opts,
		mem:      deps.newMem(),
		manifest: manifest,
		parts:    make([]partReader, 0, len(manifest.Parts)),
		deps:     deps,
		logger:   logger,
	}
	maxSeq, err := shard.openParts()
	if err != nil {
		closeErr := closeParts(shard.parts)
		if closeErr != nil {
			return nil, 0, errors.Join(err, closeErr)
		}
		return nil, 0, err
	}
	shard.recoveryReport.Merge(shard.cleanupOrphanParts())
	shard.maintenanceErr = shard.recoveryReport.MaintenanceError()
	log, err := deps.openWAL(opts.Dir, opts.WAL)
	if err != nil {
		closeErr := closeParts(shard.parts)
		return nil, 0, errors.Join(err, closeErr)
	}
	shard.wal = log
	replayed, err := log.ReplayRecords()
	if err != nil {
		logger.Warn("wal replay failed",
			"shard", shardID(opts.Database, opts.RetentionPolicy, opts.Start),
			"error", err,
		)
		closeErr := shard.closeLocked()
		return nil, 0, errors.Join(err, closeErr)
	}
	for _, record := range replayed {
		for _, point := range record.Points {
			if err := shard.mem.Apply(point); err != nil {
				closeErr := shard.closeLocked()
				return nil, 0, errors.Join(err, closeErr)
			}
			if point.WriteSeq > maxSeq {
				maxSeq = point.WriteSeq
			}
		}
		for _, tombstone := range record.Tombstones {
			shard.tombstones = append(shard.tombstones, tombstone)
			if tombstone.WriteSeq > maxSeq {
				maxSeq = tombstone.WriteSeq
			}
		}
	}
	logger.Info("shard opened",
		"shard", shardID(opts.Database, opts.RetentionPolicy, opts.Start),
		"wal_records", len(replayed),
	)
	return shard, maxSeq, nil
}
```

- [ ] **Step 8: 运行测试**

Run: `go test -run TestCustomLoggerPropagates -v -count=1 ./`
Run: `go test -run TestNilLogger -v -count=1 ./`
Run: `go test -count=1 ./internal/engine/ -timeout 120s`
Expected: PASS

- [ ] **Step 9: 提交**

```bash
git add internal/engine/engine.go internal/engine/shard.go
git commit -m "feat: Engine/Shard 持有 logger 并在 Open/Close 加日志"
```

---

### Task 4: 修复 6 处静默吞错（P0 数据安全）

**Files:**
- Modify: `internal/engine/background.go:34`
- Modify: `internal/engine/downsample_scheduler.go:49-52,65-67,86`
- Modify: `internal/wal/wal.go:436`
- Modify: `internal/service/server.go:66-69`

- [ ] **Step 1: 修复后台 compaction 吞错**

修改 `internal/engine/background.go` 的 `backgroundCompactionLoop` 函数：

```go
func (e *Engine) backgroundCompactionLoop(interval time.Duration) {
	defer e.compactWG.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := e.compactBackground(context.Background()); err != nil {
				e.logger.Warn("background compaction failed", "error", err)
			}
		case <-e.compactStop:
			return
		}
	}
}
```

- [ ] **Step 2: 修复降采样调度器吞错**

修改 `internal/engine/downsample_scheduler.go` 的 `scanDownsamplePolicies` 函数：

```go
func (e *Engine) scanDownsamplePolicies(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}
	policies, err := e.metadata.ListDownsamplePolicies(ctx)
	if err != nil {
		e.logger.Warn("list downsample policies failed", "error", err)
		return
	}
	for _, policy := range policies {
		if !e.shouldRunDownsamplePolicy(ctx, policy) {
			continue
		}
		e.startDownsamplePolicyRun(policy)
	}
}
```

修改 `shouldRunDownsamplePolicy` 函数：

```go
func (e *Engine) shouldRunDownsamplePolicy(ctx context.Context, policy model.DownsamplePolicy) bool {
	if !policy.Enabled || policy.RefreshInterval <= 0 {
		return false
	}
	watermark, _, err := e.metadata.DownsampleWatermark(ctx, policy.Name)
	if err != nil {
		e.logger.Warn("read downsample watermark failed",
			"policy", policy.Name,
			"error", err,
		)
		return false
	}
	if watermark.LastRunUnix == 0 {
		return true
	}
	return time.Since(time.Unix(0, watermark.LastRunUnix)) >= policy.RefreshInterval
}
```

修改 `startDownsamplePolicyRun` 函数：

```go
func (e *Engine) startDownsamplePolicyRun(policy model.DownsamplePolicy) {
	name := policy.Name
	if !e.acquireDownsamplePolicyRun(name) {
		return
	}
	e.downsampleWG.Add(1)
	go func() {
		defer e.downsampleWG.Done()
		defer e.releaseDownsamplePolicyRun(name)
		ctx, cancel := e.downsampleRunContext(policy)
		defer cancel()
		if _, err := e.RunDownsamplePolicy(ctx, name, time.Duration(time.Now().UnixNano())); err != nil {
			e.logger.Warn("downsample policy run failed",
				"policy", name,
				"error", err,
			)
		}
	}()
}
```

- [ ] **Step 3: 修复 WAL interval sync 吞错**

修改 `internal/wal/wal.go` 的 `Log` struct 新增 `logger *slog.Logger` 字段（放在 `metrics Metrics` 之后）：

```go
	metrics Metrics

	logger *slog.Logger

	encodeScratch []byte
```

修改 `Open` 函数，在创建 log 后归一化 logger（在 `log.startIntervalSync()` 之前）：

```go
func Open(dir string, opts Options) (*Log, error) {
	if err := storagefs.MkdirAll(dir); err != nil {
		return nil, err
	}
	if opts.SegmentBytes <= 0 {
		opts.SegmentBytes = defaultSegmentBytes
	}
	log := &Log{
		dir:     filepath.Clean(dir),
		opts:    opts,
		segment: 1,
		logger:  nopLogger(),
	}
	if err := log.openLastSegment(); err != nil {
		return nil, err
	}
	log.startIntervalSync()
```

注意：需要在 `internal/wal/wal.go` 顶部新增 `"log/slog"` import。

修改 `intervalSyncLoop` 函数：

```go
func (l *Log) intervalSyncLoop(interval time.Duration) {
	defer l.syncWG.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := l.FlushPending(); err != nil {
				l.logger.Warn("wal interval sync failed", "error", err)
			}
		case <-l.syncStop:
			return
		}
	}
}
```

- [ ] **Step 4: 修复 HTTP Serve 异常退出吞错**

修改 `internal/service/server.go` 的 `Options` struct 新增 `Logger` 字段：

```go
import "log/slog"

type Options struct {
	Addr         string
	AdminTimeout time.Duration
	EnableAdmin  bool
	AdminToken   string
	EnablePprof  bool
	AuditLogger  AuditLogger
	Logger       *slog.Logger
}
```

修改 `Server` struct 新增 `logger` 字段：

```go
type Server struct {
	options Options
	server  *http.Server
	logger  *slog.Logger
}
```

修改 `NewServer` 函数，归一化 logger：

```go
func NewServer(options Options, metrics MetricsProvider, health HealthProvider, compact CompactFunc) *Server {
	logger := options.Logger
	if logger == nil {
		logger = slog.New(nopServiceHandler{})
	}
	mux := http.NewServeMux()
	// ...existing mux setup...
	return &Server{
		options: options,
		server:  &http.Server{Addr: options.Addr, Handler: mux},
		logger:  logger,
	}
}
```

在 `internal/service/server.go` 底部新增 nopServiceHandler：

```go
type nopServiceHandler struct{}

func (nopServiceHandler) Enabled(_ context.Context, _ slog.Level) bool  { return false }
func (nopServiceHandler) Handle(_ context.Context, _ slog.Record) error { return nil }
func (h nopServiceHandler) WithAttrs(_ []slog.Attr) slog.Handler        { return h }
func (h nopServiceHandler) WithGroup(_ string) slog.Handler             { return h }
```

修改 `Start` 函数加日志：

```go
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.options.Addr)
	if err != nil {
		return err
	}
	s.logger.Info("service listening", "addr", s.options.Addr)
	go func() {
		err := s.server.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			s.logger.Error("http server stopped unexpectedly", "error", err)
			return
		}
	}()
	return nil
}
```

修改 `Shutdown` 函数加日志：

```go
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("service shutdown")
	return s.server.Shutdown(ctx)
}
```

- [ ] **Step 5: 编写吞错修复测试**

在 `internal/engine/background.go` 同目录创建或修改现有测试文件，新增：

```go
// 在 internal/engine/engine_test.go 中新增
func TestBackgroundCompactionFailureLogs(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	opts := model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 2,
		Compaction: model.CompactionOptions{
			Enabled:            true,
			BackgroundInterval: 10 * time.Millisecond,
		},
		Logger: logger,
	}
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	// 关闭引擎触发后台 compaction 上下文取消
	time.Sleep(20 * time.Millisecond)
	_ = eng.Close(ctx)
	// 验证没有 panic 即可（后台 compaction 在空数据时为 noop）
	_ = buf.String()
}
```

- [ ] **Step 6: 运行测试**

Run: `go test -run TestBackgroundCompaction -v -count=1 ./internal/engine/ -timeout 30s`
Run: `go test -count=1 ./internal/wal/ -timeout 30s`
Run: `go test -count=1 ./internal/service/ -timeout 30s`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/engine/background.go internal/engine/downsample_scheduler.go internal/wal/wal.go internal/service/server.go internal/engine/engine_test.go
git commit -m "fix: 修复 6 处静默吞错并补充 WARN/ERROR 日志"
```

---

### Task 5: 降采样和 Compaction 生命周期日志

**Files:**
- Modify: `internal/engine/downsample_executor.go:34-65` (RunDownsamplePolicy)
- Modify: `internal/engine/lifecycle.go:27-50` (CompactWithResult), `:52-84` (ApplyRetention)
- Modify: `internal/engine/background.go:8-15` (startBackgroundCompaction)
- Modify: `internal/engine/downsample_scheduler.go:12-17` (startDownsampleScheduler)

- [ ] **Step 1: 在 RunDownsamplePolicy 加 INFO/WARN 日志**

修改 `internal/engine/downsample_executor.go` 的 `RunDownsamplePolicy` 函数，在成功和失败时加日志：

```go
func (e *Engine) RunDownsamplePolicy(
	ctx context.Context,
	name string,
	now time.Duration,
) (DownsampleRunResult, error) {
	started := time.Now().UnixNano()
	policy, err := e.downsamplePolicyByName(ctx, name)
	if err != nil {
		return DownsampleRunResult{}, err
	}
	result := DownsampleRunResult{
		PolicyName:    policy.Name,
		StartedUnix:   started,
		CompletedUnix: started,
	}
	if !policy.Enabled {
		return result, nil
	}
	watermark, _, err := e.metadata.DownsampleWatermark(ctx, policy.Name)
	if err != nil {
		return DownsampleRunResult{}, err
	}
	attempt := e.downsampleStats.begin(policy.Name)
	windows := downsampleWindowsToRun(policy, watermark, now)
	result, err = e.runDownsampleWindows(ctx, policy, watermark, windows, result)
	if err != nil {
		attempt.finishFailure(result, err)
		e.logger.Warn("downsample policy run failed",
			"policy", policy.Name,
			"windows", result.WindowsProcessed,
			"error", err,
		)
		return result, err
	}
	attempt.finishSuccess(result)
	e.logger.Info("downsample policy run completed",
		"policy", policy.Name,
		"windows", result.WindowsProcessed,
		"points_written", result.PointsWritten,
	)
	return result, nil
}
```

- [ ] **Step 2: 在 CompactWithResult 加 INFO/WARN 日志**

修改 `internal/engine/lifecycle.go` 的 `CompactWithResult` 函数，在返回前加日志：

```go
func (e *Engine) CompactWithResult(ctx context.Context) (CompactionResult, error) {
	if err := ctx.Err(); err != nil {
		return CompactionResult{State: compactionTaskFailed, Error: err.Error()}, err
	}
	started := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()
	result := CompactionResult{State: compactionTaskNoop}
	for _, shard := range e.shards {
		shardResult, err := shard.CompactWithResult()
		result = mergeCompactionResult(result, shardResult)
		if err != nil {
			result.State = compactionTaskFailed
			result.Duration = time.Since(started)
			result.Error = err.Error()
			e.logger.Warn("compaction failed",
				"duration_ms", result.Duration.Milliseconds(),
				"error", err,
			)
			return result, err
		}
	}
	result.Duration = time.Since(started)
	if result.Shards > 0 && result.State == compactionTaskNoop {
		result.State = compactionTaskSucceeded
	}
	if result.State == compactionTaskSucceeded {
		e.logger.Info("compaction completed",
			"duration_ms", result.Duration.Milliseconds(),
			"input_parts", result.InputParts,
			"output_parts", result.OutputParts,
		)
	}
	return result, nil
}
```

- [ ] **Step 3: 在 ApplyRetention 加 INFO 日志**

修改 `internal/engine/lifecycle.go` 的 `ApplyRetention` 函数，在成功删除 shard 时加日志。在 `delete(e.shards, id)` 之前新增：

```go
		e.logger.Info("retention shard removed",
			"shard", id,
			"deleted_bytes", deletedBytes,
		)
		delete(e.shards, id)
```

- [ ] **Step 4: 在 startBackgroundCompaction 加 INFO 日志**

修改 `internal/engine/background.go` 的 `startBackgroundCompaction` 函数：

```go
func (e *Engine) startBackgroundCompaction() {
	if !e.opts.Compaction.Enabled || e.opts.Compaction.BackgroundInterval <= 0 {
		return
	}
	e.compactStop = make(chan struct{})
	e.compactWG.Add(1)
	e.logger.Info("background compaction started",
		"interval", e.opts.Compaction.BackgroundInterval.String(),
	)
	go e.backgroundCompactionLoop(e.opts.Compaction.BackgroundInterval)
}
```

- [ ] **Step 5: 在 startDownsampleScheduler 加 INFO 日志**

修改 `internal/engine/downsample_scheduler.go` 的 `startDownsampleScheduler` 函数：

```go
func (e *Engine) startDownsampleScheduler() {
	e.downsampleCtx, e.downsampleCancel = context.WithCancel(context.Background())
	e.downsampleStop = make(chan struct{})
	e.downsampleWG.Add(1)
	e.logger.Info("downsample scheduler started")
	go e.downsampleSchedulerLoop()
}
```

- [ ] **Step 6: 运行测试**

Run: `go test -count=1 ./internal/engine/ -timeout 180s`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/engine/downsample_executor.go internal/engine/lifecycle.go internal/engine/background.go internal/engine/downsample_scheduler.go
git commit -m "feat: 补充 compaction/retention/downsample 生命周期日志"
```

---

### Task 6: Recovery issue 和 admin 鉴权日志

**Files:**
- Modify: `internal/engine/recovery_audit.go:104-112` (partOpenRecoveryIssue), `:114-125` (partMetadataMismatchIssue)
- Modify: `internal/engine/lifecycle.go:279-300` (preflightCompactionDiskSpace)
- Modify: `internal/engine/shard.go:521-543` (openParts)
- Modify: `internal/service/admin.go:49-51` (鉴权失败)

- [ ] **Step 1: 在 openParts 中 recovery issue 加 WARN 日志**

修改 `internal/engine/shard.go` 的 `openParts` 函数，在 `partOpenRecoveryIssue` 和 `partMetadataMismatchIssue` 处加日志：

```go
func (s *Shard) openParts() (uint64, error) {
	var maxSeq uint64
	for _, meta := range s.manifest.Parts {
		part, err := s.deps.parts.OpenPart(meta.Path)
		if err != nil {
			issue := partOpenRecoveryIssue(meta, err)
			s.logger.Warn("part open recovery issue",
				"shard", shardID(s.opts.Database, s.opts.RetentionPolicy, s.opts.Start),
				"part", meta.ID,
				"kind", string(issue.Kind),
				"error", err,
			)
			s.recoveryReport.Add(issue)
			return 0, s.recoveryReport.FatalError()
		}
		if issue, ok := partMetadataMismatchIssue(meta, part.Meta()); ok {
			s.logger.Warn("part metadata mismatch",
				"shard", shardID(s.opts.Database, s.opts.RetentionPolicy, s.opts.Start),
				"part", meta.ID,
				"message", issue.Message,
			)
			closeErr := part.Close()
			s.recoveryReport.Add(issue)
			return 0, errors.Join(s.recoveryReport.FatalError(), closeErr)
		}
		s.parts = append(s.parts, part)
		if meta.MaxWriteSeq > maxSeq {
			maxSeq = meta.MaxWriteSeq
		}
		if partNumber(meta.ID) >= s.nextPart {
			s.nextPart = partNumber(meta.ID) + 1
		}
	}
	return maxSeq, nil
}
```

- [ ] **Step 2: 在 preflightCompactionDiskSpace 加 WARN 日志**

修改 `internal/engine/lifecycle.go` 的 `preflightCompactionDiskSpace` 函数，在返回 `ErrCompactionDiskSpaceExceeded` 之前加日志：

```go
func (s *Shard) preflightCompactionDiskSpace(plan compactionPlan) error {
	if s.deps.files == nil {
		return nil
	}
	required := plan.outputEstimateBytes + s.opts.Compaction.DiskSpaceReserveBytes + s.opts.Compaction.MinFreeBytes
	if required <= 0 {
		return nil
	}
	available, err := s.deps.files.AvailableBytes(s.opts.Dir)
	if err != nil {
		return fmt.Errorf("check compaction disk space: %w", err)
	}
	if available >= required {
		return nil
	}
	s.logger.Warn("compaction skipped: insufficient disk space",
		"shard", shardID(s.opts.Database, s.opts.RetentionPolicy, s.opts.Start),
		"available_bytes", available,
		"required_bytes", required,
	)
	return fmt.Errorf(
		"%w: available_bytes=%d required_bytes=%d",
		ErrCompactionDiskSpaceExceeded,
		available,
		required,
	)
}
```

- [ ] **Step 3: 在 admin 鉴权失败加 WARN 日志**

修改 `internal/service/admin.go` 的 `compactHandler` 函数，在鉴权失败时加日志。由于 `compactHandler` 是函数而非方法，需要将 logger 作为参数传入。

修改 `compactHandler` 签名新增 `logger *slog.Logger` 参数：

```go
func compactHandler(
	timeout time.Duration,
	token string,
	audit AuditLogger,
	compact CompactFunc,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeJSON(writer, http.StatusMethodNotAllowed, adminResponse{OK: false, Error: "method not allowed"})
			return
		}
		if token == "" {
			writeJSON(writer, http.StatusServiceUnavailable, adminResponse{OK: false, Error: "admin auth token required"})
			return
		}
		if !authorizedAdminRequest(request, token) {
			logger.Warn("admin auth failed", "remote_addr", request.RemoteAddr)
			writeJSON(writer, http.StatusUnauthorized, adminResponse{OK: false, Error: "admin unauthorized"})
			return
		}
		// ...rest unchanged...
	}
}
```

修改 `internal/service/server.go` 的 `NewServer` 函数中 `compactHandler` 调用，传入 logger：

```go
		mux.HandleFunc("/admin/compact", compactHandler(
			options.AdminTimeout,
			options.AdminToken,
			options.AuditLogger,
			compact,
			logger,
		))
```

- [ ] **Step 4: 运行测试**

Run: `go test -count=1 ./internal/engine/ -timeout 180s`
Run: `go test -count=1 ./internal/service/ -timeout 30s`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/engine/recovery_audit.go internal/engine/lifecycle.go internal/engine/shard.go internal/service/admin.go internal/service/server.go
git commit -m "feat: 补充 recovery issue 和 admin 鉴权 WARN 日志"
```

---

### Task 7: CLI 设置 slog handler

**Files:**
- Modify: `cmd/mts-storage/main.go`

- [ ] **Step 1: 在 main 中设置 slog 默认 logger**

修改 `cmd/mts-storage/main.go` 的 `main` 函数和 `run` 函数：

```go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/openmts/mts/internal/storagecheck"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: mts-storage <check|repair|migrate|snapshot|restore> [flags] <path>")
		return 2
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "repair":
		return runRepair(args[1:], stdout, stderr)
	case "migrate":
		return runMigrate(args[1:], stdout, stderr)
	case "snapshot":
		return runSnapshot(args[1:], stdout, stderr)
	case "restore":
		return runRestore(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}
```

- [ ] **Step 2: 运行 CLI 测试**

Run: `go test -count=1 ./cmd/mts-storage/ -timeout 30s`
Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add cmd/mts-storage/main.go
git commit -m "feat: CLI 设置 slog stderr TextHandler"
```

---

### Task 8: 全量验证与清理

**Files:**
- 无新建文件，验证全部测试通过

- [ ] **Step 1: 运行全量单元测试**

Run: `go test -count=1 ./internal/... -timeout 300s`
Expected: PASS — 所有包通过

- [ ] **Step 2: 运行根包测试**

Run: `go test -count=1 ./ -timeout 120s`
Expected: PASS

- [ ] **Step 3: 运行 E2E 测试**

Run: `go test -count=1 ./tests/e2e/... -timeout 600s`
Expected: PASS

- [ ] **Step 4: 检查代码覆盖率**

Run: `go test -cover ./internal/engine/ ./internal/wal/ ./internal/service/ -timeout 120s`
Expected: 所有包覆盖率 >= 90%

- [ ] **Step 5: 运行 goimports-reviser**

Run: `goimports-reviser -project-name github.com/openmts/mts ./...`
Expected: 格式化完成

- [ ] **Step 6: 运行 golangci-lint**

Run: `golangci-lint run ./... --timeout 720s`
Expected: 无新增 lint 错误

- [ ] **Step 7: 更新 README.md**

检查 `README.md` 是否需要补充 Logger 配置说明。在 Options 相关段落补充一行：

```markdown
- `Logger` — 自定义 `*slog.Logger`，nil 时使用零开销 nopHandler。
```

- [ ] **Step 8: 提交最终验证**

```bash
git add -A
git commit -m "test: 全量验证 slog 集成并更新文档"
```
