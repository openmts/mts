package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openmts/mts/internal/faultinject"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/storagefs"
)

func TestRetentionRemoveFailureIsolatesClosedShardAndRetries(t *testing.T) {
	ctx := context.Background()
	eng := openTestEngine(t, ctx, model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
		Retention:     time.Hour,
	})
	point := model.Point{
		Measurement: "cpu",
		Timestamp:   0,
		Fields:      map[string]model.FieldValue{"value": model.Float64Value(1)},
	}
	if err := eng.Write(ctx, []model.Point{point}, model.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := eng.Flush(ctx); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	fs := faultinject.NewFS()
	fs.Fail(faultinject.OpRemove, errors.New("injected remove failure"))
	restore := storagefs.SetFaultController(fs)
	err := eng.ApplyRetention(ctx, time.Unix(0, int64(2*time.Hour)))
	if err == nil || !strings.Contains(err.Error(), "fault remove") {
		t.Fatalf("ApplyRetention() error = %v, want remove fault", err)
	}
	if got := len(eng.shards); got != 0 {
		t.Errorf("active shard count = %d, want 0 after close", got)
	}
	writeErr, panicErr := writeRetentionPoint(eng, ctx, point)
	restore()
	if panicErr != nil {
		t.Fatalf("Write() panicked after retention failure: %v", panicErr)
	}
	if writeErr == nil || !strings.Contains(writeErr.Error(), "fault remove") {
		t.Fatalf("Write() error = %v, want pending cleanup error", writeErr)
	}
	if err := eng.ApplyRetention(ctx, time.Unix(0, int64(2*time.Hour))); err != nil {
		t.Fatalf("ApplyRetention(retry) error = %v", err)
	}
	closeTestEngine(t, ctx, eng)
}

func writeRetentionPoint(
	eng *Engine,
	ctx context.Context,
	point model.Point,
) (err error, panicErr error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			panicErr = fmt.Errorf("%v", recovered)
		}
	}()
	err = eng.Write(ctx, []model.Point{point}, model.WriteOptions{Sync: true})
	return err, nil
}

func TestRetentionRetriesPartiallyDeletedShard(t *testing.T) {
	ctx := context.Background()
	eng, shardDir := openExpiredRetentionShard(t, ctx, t.TempDir())
	files := &partialRemoveFileOps{}
	for _, shard := range eng.shards {
		shard.deps.files = files
	}
	if err := eng.ApplyRetention(ctx, time.Unix(0, int64(2*time.Hour))); err == nil {
		t.Fatal("ApplyRetention() error = nil, want partial remove failure")
	}
	if got := len(eng.shards); got != 0 {
		t.Fatalf("active shard count = %d, want 0", got)
	}
	if err := eng.ApplyRetention(ctx, time.Unix(0, int64(2*time.Hour))); err != nil {
		t.Fatalf("ApplyRetention(retry) error = %v", err)
	}
	if _, err := os.Stat(shardDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(shard dir) error = %v, want not exist", err)
	}
	closeTestEngine(t, ctx, eng)
}

func TestRetentionRemoveFailureRecoversAfterRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	eng, _ := openExpiredRetentionShard(t, ctx, dir)
	fs := faultinject.NewFS()
	fs.FailNext(faultinject.OpRemove, errors.New("injected remove failure"))
	restore := storagefs.SetFaultController(fs)
	err := eng.ApplyRetention(ctx, time.Unix(0, int64(2*time.Hour)))
	restore()
	if err == nil {
		t.Fatal("ApplyRetention() error = nil, want remove failure")
	}
	closeTestEngine(t, ctx, eng)

	reopened := openTestEngine(t, ctx, retentionFailureOptions(dir))
	if err := reopened.ApplyRetention(ctx, time.Unix(0, int64(2*time.Hour))); err != nil {
		t.Fatalf("ApplyRetention(reopened) error = %v", err)
	}
	if got := len(reopened.shards); got != 0 {
		t.Fatalf("active shard count = %d, want 0", got)
	}
	closeTestEngine(t, ctx, reopened)
}

func openExpiredRetentionShard(
	t *testing.T,
	ctx context.Context,
	dir string,
) (*Engine, string) {
	t.Helper()
	eng := openTestEngine(t, ctx, retentionFailureOptions(dir))
	point := model.Point{
		Measurement: "cpu",
		Timestamp:   0,
		Fields:      map[string]model.FieldValue{"value": model.Float64Value(1)},
	}
	if err := eng.Write(ctx, []model.Point{point}, model.WriteOptions{Sync: true}); err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("Write() error = %v", err)
	}
	if err := eng.Flush(ctx); err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("Flush() error = %v", err)
	}
	for _, shard := range eng.shards {
		return eng, shard.opts.Dir
	}
	t.Fatal("expired shard missing")
	return nil, ""
}

func retentionFailureOptions(dir string) model.Options {
	return model.Options{Path: dir, ShardDuration: time.Hour, Retention: time.Hour}
}

type partialRemoveFileOps struct {
	defaultFileOps
	failed bool
}

func (f *partialRemoveFileOps) RemoveAll(path string) error {
	if f.failed {
		return os.RemoveAll(filepath.Clean(path))
	}
	f.failed = true
	entries, err := os.ReadDir(filepath.Clean(path))
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		if err := os.RemoveAll(filepath.Join(path, entries[0].Name())); err != nil {
			return err
		}
	}
	return errors.New("partial remove failure")
}
