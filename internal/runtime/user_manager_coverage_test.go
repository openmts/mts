package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestRuntimeLocalUserManagerCoversUserGrantAndTokenLifecycle(t *testing.T) {
	ctx := context.Background()
	manager, err := openRuntimeUserManager(t.TempDir(), UserOptions{})
	if err != nil {
		t.Fatalf("openRuntimeUserManager() error = %v", err)
	}
	defer closeRuntimeUserManager(t, manager)

	user := User{
		Name:        "alice",
		DisplayName: "Alice",
		Role:        UserRoleUser,
		Metadata:    map[string]string{"team": "ops"},
	}
	if err := manager.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	user.DisplayName = "Alice Updated"
	user.Role = UserRoleAdmin
	if err := manager.UpdateUser(ctx, user); err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}

	users, err := manager.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 1 || users[0].DisplayName != "Alice Updated" || users[0].Role != UserRoleAdmin {
		t.Fatalf("ListUsers() = %#v, want updated alice", users)
	}

	if err := manager.GrantDatabasePermission(ctx, "alice", "metrics", DatabasePermissionAdmin); err != nil {
		t.Fatalf("GrantDatabasePermission() error = %v", err)
	}
	grants, err := manager.ListDatabasePermissions(ctx, "alice")
	if err != nil {
		t.Fatalf("ListDatabasePermissions() error = %v", err)
	}
	if len(grants) != 1 || grants[0].Database != "metrics" || grants[0].Permission != DatabasePermissionAdmin {
		t.Fatalf("ListDatabasePermissions() = %#v, want metrics admin", grants)
	}

	page, err := manager.ListUserGrantPage(ctx, "", 10)
	if err != nil {
		t.Fatalf("ListUserGrantPage() error = %v", err)
	}
	if page.TotalUsers != 1 || len(page.Items) != 1 || len(page.Items[0].Grants) != 1 {
		t.Fatalf("ListUserGrantPage() = %#v, want one user and grant", page)
	}
	page.Items[0].User.Metadata["team"] = "mutated"
	stored, ok, err := manager.GetUser(ctx, "alice")
	if err != nil || !ok {
		t.Fatalf("GetUser() = %#v, %v, %v; want user", stored, ok, err)
	}
	if stored.Metadata["team"] != "ops" {
		t.Fatalf("stored metadata = %#v, want independent copy", stored.Metadata)
	}

	if err := manager.RevokeDatabasePermission(ctx, "alice", "metrics", DatabasePermissionAdmin); err != nil {
		t.Fatalf("RevokeDatabasePermission() error = %v", err)
	}
	grants, err = manager.ListDatabasePermissions(ctx, "alice")
	if err != nil || len(grants) != 0 {
		t.Fatalf("ListDatabasePermissions(after revoke) = %#v, %v; want empty", grants, err)
	}

	if err := manager.SetPassword(ctx, "alice", "secret12"); err != nil {
		t.Fatalf("SetPassword() error = %v", err)
	}
	if err := manager.ChangePassword(ctx, "alice", "secret12", "secret34"); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if _, err := manager.Authenticate(ctx, Credentials{UserName: "alice", Password: "secret12"}, time.Hour); err == nil {
		t.Fatal("Authenticate(old password) error = nil, want error")
	}
	token, err := manager.Authenticate(ctx, Credentials{UserName: "alice", Password: "secret34"}, time.Hour)
	if err != nil {
		t.Fatalf("Authenticate(new password) error = %v", err)
	}
	if err := manager.RevokeToken(ctx, token.Token); err != nil {
		t.Fatalf("RevokeToken() error = %v", err)
	}
	if _, err := manager.VerifyToken(ctx, token.Token); err == nil {
		t.Fatal("VerifyToken(revoked) error = nil, want error")
	}

	if err := manager.DeleteUser(ctx, "alice"); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	users, err = manager.ListUsers(ctx)
	if err != nil || len(users) != 0 {
		t.Fatalf("ListUsers(after delete) = %#v, %v; want empty", users, err)
	}
}

func TestRuntimeEngineStorageReturnsUnderlyingEngine(t *testing.T) {
	ctx := context.Background()
	engine := openTestRuntimeEngine(t, ctx, Options{
		Storage: model.Options{Path: t.TempDir(), ShardDuration: time.Hour},
	})
	defer closeTestRuntimeEngine(t, ctx, engine)

	if engine.Storage() == nil || engine.Storage() != engine.storage {
		t.Fatalf("Storage() = %p, want underlying %p", engine.Storage(), engine.storage)
	}
}

func TestOpenRuntimeUserManagerRejectsFilePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users-file")
	if err := os.WriteFile(path, []byte("not a directory"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if manager, err := openRuntimeUserManager(path, UserOptions{}); err == nil {
		closeErr := manager.Close()
		t.Fatalf("openRuntimeUserManager(file) error = nil, want error close=%v", closeErr)
	}
}
