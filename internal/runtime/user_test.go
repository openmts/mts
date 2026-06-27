package runtime_test

import (
	"context"
	"errors"
	"testing"

	"github.com/openmts/mts/internal/runtime"
	"github.com/openmts/mts/internal/user"
)

func TestOpenUserRuntimeDefaultsToLocalManager(t *testing.T) {
	t.Parallel()
	manager, err := runtime.OpenUserManager(t.TempDir(), runtime.UserOptions{})
	if err != nil {
		t.Fatalf("OpenUserManager() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	if err := manager.CreateUser(context.Background(), user.User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	got, ok, err := manager.GetUser(context.Background(), "alice")
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if !ok || got.Name != "alice" {
		t.Fatalf("GetUser() = %#v, %v, want alice", got, ok)
	}
}

func TestOpenUserRuntimeRejectsUnsupportedEndpoint(t *testing.T) {
	t.Parallel()
	_, err := runtime.OpenUserManager(t.TempDir(), runtime.UserOptions{Endpoint: "ldap://example"})
	if !errors.Is(err, user.ErrUnsupportedEndpoint) {
		t.Fatalf("OpenUserManager() error = %v, want ErrUnsupportedEndpoint", err)
	}
}
