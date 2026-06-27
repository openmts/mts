package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openmts/mts/internal/user"
)

func TestRuntimeLocalUserManagerSupportsUserPermissionAndAuth(t *testing.T) {
	ctx := context.Background()
	manager, err := openRuntimeUserManager(t.TempDir(), UserOptions{})
	if err != nil {
		t.Fatalf("openRuntimeUserManager() error = %v", err)
	}
	defer closeRuntimeUserManager(t, manager)

	if err := manager.CreateUser(ctx, User{Name: "alice", Role: UserRoleAdmin}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := manager.SetPassword(ctx, "alice", "secret"); err != nil {
		t.Fatalf("SetPassword() error = %v", err)
	}
	if err := manager.GrantDatabasePermission(ctx, "alice", "metrics", DatabasePermissionAdmin); err != nil {
		t.Fatalf("GrantDatabasePermission() error = %v", err)
	}
	if err := manager.CheckDatabasePermission(ctx, "alice", "metrics", DatabasePermissionWrite); err != nil {
		t.Fatalf("CheckDatabasePermission(write) error = %v", err)
	}

	token, err := manager.Authenticate(ctx, Credentials{UserName: "alice", Password: "secret"}, time.Hour)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	principal, err := manager.VerifyToken(ctx, token.Token)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	if principal.UserName != "alice" {
		t.Fatalf("principal user = %q, want alice", principal.UserName)
	}
}

func TestRuntimeLocalUserManagerPersistsUsers(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	manager, err := openRuntimeUserManager(dir, UserOptions{})
	if err != nil {
		t.Fatalf("openRuntimeUserManager() error = %v", err)
	}
	if err := manager.CreateUser(ctx, User{Name: "alice"}); err != nil {
		closeErr := manager.Close()
		t.Fatalf("CreateUser() error = %v close=%v", err, closeErr)
	}
	closeRuntimeUserManager(t, manager)

	reopened, err := openRuntimeUserManager(dir, UserOptions{})
	if err != nil {
		t.Fatalf("openRuntimeUserManager(reopen) error = %v", err)
	}
	defer closeRuntimeUserManager(t, reopened)

	runtimeUser, ok, err := reopened.GetUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if !ok || runtimeUser.Name != "alice" {
		t.Fatalf("GetUser() = %#v, %v; want alice true", runtimeUser, ok)
	}
}

func TestRuntimeLocalUserManagerMapsErrors(t *testing.T) {
	manager, err := openRuntimeUserManager(t.TempDir(), UserOptions{PasswordAuthDisabled: true})
	if err != nil {
		t.Fatalf("openRuntimeUserManager() error = %v", err)
	}
	defer closeRuntimeUserManager(t, manager)

	_, err = manager.Authenticate(context.Background(), Credentials{UserName: "alice", Password: "secret"}, time.Hour)
	if !errors.Is(err, user.ErrAuthenticationDisabled) {
		t.Fatalf("Authenticate(disabled) error = %v, want ErrAuthenticationDisabled", err)
	}
}

func closeRuntimeUserManager(t *testing.T, manager interface{ Close() error }) {
	t.Helper()
	if err := manager.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
