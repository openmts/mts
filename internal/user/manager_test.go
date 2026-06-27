package user

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestManagerUserCRUD(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)

	user := User{
		Name:        "alice",
		DisplayName: "Alice",
		Role:        RoleAdmin,
		Metadata:    map[string]string{"team": "platform"},
	}
	if err := manager.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := manager.CreateUser(ctx, user); !errors.Is(err, ErrUserAlreadyExists) {
		t.Fatalf("CreateUser(duplicate) error = %v, want ErrUserAlreadyExists", err)
	}

	got, ok, err := manager.GetUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if !ok || got.Name != "alice" || got.DisplayName != "Alice" || got.Role != RoleAdmin || got.Metadata["team"] != "platform" {
		t.Fatalf("GetUser() = %#v ok=%v, want alice", got, ok)
	}
	got.Metadata["team"] = "mutated"
	got, ok, err = manager.GetUser(ctx, "alice")
	if err != nil || !ok {
		t.Fatalf("GetUser(after mutation) error = %v ok=%v", err, ok)
	}
	if got.Metadata["team"] != "platform" {
		t.Fatalf("metadata leaked mutation = %q, want platform", got.Metadata["team"])
	}

	if err := manager.CreateUser(ctx, User{Name: "bob"}); err != nil {
		t.Fatalf("CreateUser(bob) error = %v", err)
	}
	bob, ok, err := manager.GetUser(ctx, "bob")
	if err != nil || !ok {
		t.Fatalf("GetUser(bob) error = %v ok=%v", err, ok)
	}
	if bob.Role != RoleUser {
		t.Fatalf("bob role = %q, want user", bob.Role)
	}
	users, err := manager.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	names := userNames(users)
	if !slices.Equal(names, []string{"alice", "bob"}) {
		t.Fatalf("users = %v, want [alice bob]", names)
	}

	if err := manager.UpdateUser(ctx, User{
		Name:        "alice",
		DisplayName: "Alice Updated",
		Disabled:    true,
		Metadata:    map[string]string{"team": "storage"},
	}); err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	updated, ok, err := manager.GetUser(ctx, "alice")
	if err != nil || !ok {
		t.Fatalf("GetUser(updated) error = %v ok=%v", err, ok)
	}
	if updated.DisplayName != "Alice Updated" || !updated.Disabled || updated.Metadata["team"] != "storage" {
		t.Fatalf("updated user = %#v, want updated fields", updated)
	}

	if err := manager.DeleteUser(ctx, "alice"); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	_, ok, err = manager.GetUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUser(deleted) error = %v", err)
	}
	if ok {
		t.Fatal("GetUser(deleted) ok = true, want false")
	}
}

func TestManagerListUsersReturnsClones(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)

	if err := manager.CreateUser(ctx, User{
		Name:     "alice",
		Metadata: map[string]string{"team": "platform"},
	}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	users, err := manager.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	users[0].Metadata["team"] = "mutated"

	got, ok, err := manager.GetUser(ctx, "alice")
	if err != nil || !ok {
		t.Fatalf("GetUser() error = %v ok=%v", err, ok)
	}
	if got.Metadata["team"] != "platform" {
		t.Fatalf("metadata leaked from ListUsers() = %q, want platform", got.Metadata["team"])
	}
}

func TestManagerPersistsUsersFileWithPrivatePermissions(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	manager, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer closeUserManager(t, manager)

	if err := manager.CreateUser(ctx, User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	assertPathMode(t, dir, 0700)
	assertPathMode(t, filepath.Join(dir, "users.bin"), 0600)
}

func TestManagerCanceledContextDoesNotMutateState(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)
	if err := manager.CreateUser(ctx, User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()

	assertCanceled(t, manager.CreateUser(canceled, User{Name: "bob"}))
	assertCanceled(t, manager.UpdateUser(canceled, User{Name: "alice", Disabled: true}))
	_, _, err := manager.GetUser(canceled, "alice")
	assertCanceled(t, err)
	_, err = manager.ListUsers(canceled)
	assertCanceled(t, err)
	assertCanceled(t, manager.GrantPermission(
		canceled,
		"alice",
		"metrics",
		PermissionRead,
	))
	assertCanceled(t, manager.RevokePermission(
		canceled,
		"alice",
		"metrics",
		PermissionRead,
	))
	_, err = manager.ListPermissions(canceled, "alice")
	assertCanceled(t, err)
	assertCanceled(t, manager.CheckPermission(
		canceled,
		"alice",
		"metrics",
		PermissionRead,
	))
	assertCanceled(t, manager.DeleteUser(canceled, "alice"))

	user, ok, err := manager.GetUser(ctx, "alice")
	if err != nil || !ok {
		t.Fatalf("GetUser(after canceled operations) error = %v ok=%v", err, ok)
	}
	if user.Disabled {
		t.Fatal("user was mutated by canceled UpdateUser")
	}
	if _, ok, err := manager.GetUser(ctx, "bob"); err != nil || ok {
		t.Fatalf("GetUser(canceled create) error = %v ok=%v, want missing bob", err, ok)
	}
}

func TestManagerRejectsInvalidUsers(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)

	if err := manager.CreateUser(ctx, User{Name: " "}); !errors.Is(err, ErrInvalidUser) {
		t.Fatalf("CreateUser(empty) error = %v, want ErrInvalidUser", err)
	}
	if err := manager.CreateUser(ctx, User{Name: "bad-role", Role: Role("owner")}); !errors.Is(err, ErrInvalidUser) {
		t.Fatalf("CreateUser(invalid role) error = %v, want ErrInvalidUser", err)
	}
	if err := manager.UpdateUser(ctx, User{Name: "missing"}); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("UpdateUser(missing) error = %v, want ErrUserNotFound", err)
	}
	if err := manager.DeleteUser(ctx, "missing"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("DeleteUser(missing) error = %v, want ErrUserNotFound", err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := manager.CreateUser(canceled, User{Name: "canceled"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateUser(canceled) error = %v, want context.Canceled", err)
	}
}

func openTestUserManager(t *testing.T) *Manager {
	t.Helper()
	manager, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	return manager
}

func closeUserManager(t *testing.T, manager interface{ Close() error }) {
	t.Helper()
	if err := manager.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func userNames(users []User) []string {
	names := make([]string, len(users))
	for index, user := range users {
		names[index] = user.Name
	}
	return names
}

func assertPathMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode(%s) = %v, want %v", path, got, want)
	}
}

func assertCanceled(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
