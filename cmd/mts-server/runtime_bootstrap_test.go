package main

import (
	"context"
	"testing"

	mts "github.com/openmts/mts"
)

func TestRuntimeDoesNotCreateImplicitAdmin(t *testing.T) {
	runtime := openRuntimeForBootstrapTest(t)
	if user, ok, err := runtime.engine.GetUser(context.Background(), "admin"); err != nil {
		t.Fatalf("GetUser(admin) error = %v", err)
	} else if ok {
		t.Fatalf("GetUser(admin) = %#v, want missing", user)
	}
}

func TestRuntimePreservesAdminDispositionOnRestart(t *testing.T) {
	tests := []struct {
		name string
		user mts.User
	}{
		{
			name: "disabled",
			user: mts.User{Name: "admin", Role: mts.UserRoleAdmin, Disabled: true},
		},
		{
			name: "demoted",
			user: mts.User{Name: "admin", Role: mts.UserRoleUser},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := bootstrapTestConfig(t)
			runtime, err := openRuntime(ctx, cfg)
			if err != nil {
				t.Fatalf("openRuntime(first) error = %v", err)
			}
			if err := runtime.engine.CreateUser(ctx, test.user); err != nil {
				t.Fatalf("CreateUser(admin) error = %v", err)
			}
			if err := runtime.shutdown(ctx); err != nil {
				t.Fatalf("shutdown(first) error = %v", err)
			}

			reopened, err := openRuntime(ctx, cfg)
			if err != nil {
				t.Fatalf("openRuntime(reopened) error = %v", err)
			}
			t.Cleanup(func() {
				if err := reopened.shutdown(ctx); err != nil {
					t.Fatalf("shutdown(reopened) error = %v", err)
				}
			})
			got, ok, err := reopened.engine.GetUser(ctx, "admin")
			if err != nil {
				t.Fatalf("GetUser(admin) error = %v", err)
			}
			if !ok {
				t.Fatal("GetUser(admin) missing")
			}
			if got.Role != test.user.Role || got.Disabled != test.user.Disabled {
				t.Fatalf("GetUser(admin) = %#v, want role=%q disabled=%v", got, test.user.Role, test.user.Disabled)
			}
		})
	}
}

func openRuntimeForBootstrapTest(t *testing.T) *serverRuntime {
	t.Helper()
	runtime, err := openRuntime(context.Background(), bootstrapTestConfig(t))
	if err != nil {
		t.Fatalf("openRuntime() error = %v", err)
	}
	t.Cleanup(func() {
		if err := runtime.shutdown(context.Background()); err != nil {
			t.Fatalf("shutdown runtime error = %v", err)
		}
	})
	return runtime
}

func bootstrapTestConfig(t *testing.T) config {
	t.Helper()
	cfg := defaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.HTTP.Addr = "127.0.0.1:0"
	cfg.GRPC.Addr = "127.0.0.1:0"
	return cfg
}
