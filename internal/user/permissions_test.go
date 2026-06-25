package user

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestManagerDatabasePermissions(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)

	if err := manager.CreateUser(ctx, User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	grantReadWritePermissions(t, ctx, manager)

	grants, err := manager.ListPermissions(ctx, "alice")
	if err != nil {
		t.Fatalf("ListPermissions() error = %v", err)
	}
	if !slices.Equal(grants, []Grant{
		{Database: "metrics", Permission: PermissionRead},
		{Database: "metrics", Permission: PermissionWrite},
	}) {
		t.Fatalf("grants = %#v, want read/write", grants)
	}
	assertPermissionAllowed(t, ctx, manager, "alice", "metrics", PermissionRead)
	assertPermissionDenied(t, ctx, manager, "alice", "metrics", PermissionAdmin)

	if err := manager.RevokePermission(
		ctx,
		"alice",
		"metrics",
		PermissionRead,
	); err != nil {
		t.Fatalf("RevokePermission(read) error = %v", err)
	}
	assertPermissionDenied(t, ctx, manager, "alice", "metrics", PermissionRead)
	grantAdminPermission(t, ctx, manager)
	for _, permission := range []Permission{
		PermissionRead,
		PermissionWrite,
		PermissionAdmin,
	} {
		assertPermissionAllowed(t, ctx, manager, "alice", "metrics", permission)
	}

	if err := manager.UpdateUser(ctx, User{Name: "alice", Disabled: true}); err != nil {
		t.Fatalf("UpdateUser(disabled) error = %v", err)
	}
	assertPermissionDenied(t, ctx, manager, "alice", "metrics", PermissionRead)
}

func TestManagerDeleteUserRemovesDatabasePermissions(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)

	if err := manager.CreateUser(ctx, User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := manager.GrantPermission(
		ctx,
		"alice",
		"metrics",
		PermissionAdmin,
	); err != nil {
		t.Fatalf("GrantPermission() error = %v", err)
	}
	if err := manager.DeleteUser(ctx, "alice"); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	if err := manager.CheckPermission(
		ctx,
		"alice",
		"metrics",
		PermissionRead,
	); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("CheckPermission(deleted user) error = %v, want ErrPermissionDenied", err)
	}
	if _, err := manager.ListPermissions(ctx, "alice"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("ListPermissions(deleted user) error = %v, want ErrUserNotFound", err)
	}
	if err := manager.CreateUser(ctx, User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser(recreate) error = %v", err)
	}
	if err := manager.CheckPermission(
		ctx,
		"alice",
		"metrics",
		PermissionRead,
	); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("CheckPermission(recreated user) error = %v, want ErrPermissionDenied", err)
	}
}

func TestManagerCheckPermissionFailsClosedForMissingUser(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)

	if err := manager.CheckPermission(
		ctx,
		"missing",
		"metrics",
		PermissionRead,
	); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("CheckPermission(missing user) error = %v, want ErrPermissionDenied", err)
	}
}

func TestManagerRejectsInvalidPermissions(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)

	err := manager.GrantPermission(ctx, "missing", "metrics", PermissionRead)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("GrantPermission(missing user) error = %v, want ErrUserNotFound", err)
	}
	if err := manager.CreateUser(ctx, User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	err = manager.GrantPermission(ctx, "alice", "", PermissionRead)
	if !errors.Is(err, ErrInvalidPermission) {
		t.Fatalf("GrantPermission(empty database) error = %v, want ErrInvalidPermission", err)
	}
	err = manager.GrantPermission(ctx, "alice", "metrics", Permission("owner"))
	if !errors.Is(err, ErrInvalidPermission) {
		t.Fatalf("GrantPermission(invalid permission) error = %v, want ErrInvalidPermission", err)
	}
	err = manager.RevokePermission(ctx, "missing", "metrics", PermissionRead)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("RevokePermission(missing user) error = %v, want ErrUserNotFound", err)
	}
	_, err = manager.ListPermissions(ctx, "missing")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("ListPermissions(missing user) error = %v, want ErrUserNotFound", err)
	}
	err = manager.CheckPermission(ctx, "alice", "", PermissionRead)
	if !errors.Is(err, ErrInvalidPermission) {
		t.Fatalf("CheckPermission(empty database) error = %v, want ErrInvalidPermission", err)
	}
}

func grantReadWritePermissions(
	t *testing.T,
	ctx context.Context,
	manager *Manager,
) {
	t.Helper()
	for _, permission := range []Permission{
		PermissionRead,
		PermissionWrite,
	} {
		if err := manager.GrantPermission(ctx, "alice", "metrics", permission); err != nil {
			t.Fatalf("GrantPermission(%s) error = %v", permission, err)
		}
	}
}

func grantAdminPermission(t *testing.T, ctx context.Context, manager *Manager) {
	t.Helper()
	if err := manager.GrantPermission(
		ctx,
		"alice",
		"metrics",
		PermissionAdmin,
	); err != nil {
		t.Fatalf("GrantPermission(admin) error = %v", err)
	}
}

func assertPermissionAllowed(
	t *testing.T,
	ctx context.Context,
	manager *Manager,
	userName string,
	database string,
	permission Permission,
) {
	t.Helper()
	if err := manager.CheckPermission(ctx, userName, database, permission); err != nil {
		t.Fatalf("CheckPermission(%s) error = %v", permission, err)
	}
}

func assertPermissionDenied(
	t *testing.T,
	ctx context.Context,
	manager *Manager,
	userName string,
	database string,
	permission Permission,
) {
	t.Helper()
	err := manager.CheckPermission(ctx, userName, database, permission)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("CheckPermission(%s) error = %v, want ErrPermissionDenied", permission, err)
	}
}
