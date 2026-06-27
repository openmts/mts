package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestRuntimeEngineUsesDefaultUserManager(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	engine := openTestRuntimeEngine(t, ctx, Options{
		Storage: model.Options{Path: dir, ShardDuration: time.Hour},
	})
	if err := engine.Users().CreateUser(ctx, User{Name: "alice"}); err != nil {
		closeErr := engine.Close(ctx)
		t.Fatalf("CreateUser() error = %v close=%v", err, closeErr)
	}
	closeTestRuntimeEngine(t, ctx, engine)

	reopened := openTestRuntimeEngine(t, ctx, Options{
		Storage: model.Options{Path: dir, ShardDuration: time.Hour},
	})
	defer closeTestRuntimeEngine(t, ctx, reopened)

	runtimeUser, ok, err := reopened.Users().GetUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if !ok || runtimeUser.Name != "alice" {
		t.Fatalf("GetUser() = %#v, %v; want alice true", runtimeUser, ok)
	}
}

func openTestRuntimeEngine(t *testing.T, ctx context.Context, opts Options) *Engine {
	t.Helper()
	engine, err := OpenEngine(ctx, opts)
	if err != nil {
		t.Fatalf("OpenEngine() error = %v", err)
	}
	return engine
}

func closeTestRuntimeEngine(t *testing.T, ctx context.Context, engine *Engine) {
	t.Helper()
	if err := engine.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
