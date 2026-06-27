package mts_test

import (
	"context"
	"testing"

	mts "github.com/openmts/mts"
)

func openTestEngine(t *testing.T, ctx context.Context, opts mts.Options) *mts.Engine {
	t.Helper()
	engine, err := mts.Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	return engine
}

func closeTestEngine(t *testing.T, ctx context.Context, engine *mts.Engine) {
	t.Helper()
	if err := engine.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
