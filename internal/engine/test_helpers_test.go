package engine

import (
	"context"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func openTestEngine(t *testing.T, ctx context.Context, opts model.Options) *Engine {
	t.Helper()
	engine, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	return engine
}

func closeTestEngine(t *testing.T, ctx context.Context, engine *Engine) {
	t.Helper()
	if err := engine.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
