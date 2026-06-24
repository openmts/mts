package mts_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	mts "github.com/openmts/mts"
)

func TestLocalUserManagerDatabasePermissions(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)

	if err := manager.CreateUser(ctx, mts.User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	grantReadWritePermissions(t, ctx, manager)

	grants, err := manager.ListDatabasePermissions(ctx, "alice")
	if err != nil {
		t.Fatalf("ListDatabasePermissions() error = %v", err)
	}
	if !slices.Equal(grants, []mts.DatabaseGrant{
		{Database: "metrics", Permission: mts.DatabasePermissionRead},
		{Database: "metrics", Permission: mts.DatabasePermissionWrite},
	}) {
		t.Fatalf("grants = %#v, want read/write", grants)
	}
	assertPermissionAllowed(t, ctx, manager, "alice", "metrics", mts.DatabasePermissionRead)
	assertPermissionDenied(t, ctx, manager, "alice", "metrics", mts.DatabasePermissionAdmin)

	if err := manager.RevokeDatabasePermission(
		ctx,
		"alice",
		"metrics",
		mts.DatabasePermissionRead,
	); err != nil {
		t.Fatalf("RevokeDatabasePermission(read) error = %v", err)
	}
	assertPermissionDenied(t, ctx, manager, "alice", "metrics", mts.DatabasePermissionRead)
	grantAdminPermission(t, ctx, manager)
	for _, permission := range []mts.DatabasePermission{
		mts.DatabasePermissionRead,
		mts.DatabasePermissionWrite,
		mts.DatabasePermissionAdmin,
	} {
		assertPermissionAllowed(t, ctx, manager, "alice", "metrics", permission)
	}

	if err := manager.UpdateUser(ctx, mts.User{Name: "alice", Disabled: true}); err != nil {
		t.Fatalf("UpdateUser(disabled) error = %v", err)
	}
	assertPermissionDenied(t, ctx, manager, "alice", "metrics", mts.DatabasePermissionRead)
}

func TestLocalUserManagerDeleteUserRemovesDatabasePermissions(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)

	if err := manager.CreateUser(ctx, mts.User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := manager.GrantDatabasePermission(
		ctx,
		"alice",
		"metrics",
		mts.DatabasePermissionAdmin,
	); err != nil {
		t.Fatalf("GrantDatabasePermission() error = %v", err)
	}
	if err := manager.DeleteUser(ctx, "alice"); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	if err := manager.CheckDatabasePermission(
		ctx,
		"alice",
		"metrics",
		mts.DatabasePermissionRead,
	); !errors.Is(err, mts.ErrPermissionDenied) {
		t.Fatalf("CheckDatabasePermission(deleted user) error = %v, want ErrPermissionDenied", err)
	}
	if _, err := manager.ListDatabasePermissions(ctx, "alice"); !errors.Is(err, mts.ErrUserNotFound) {
		t.Fatalf("ListDatabasePermissions(deleted user) error = %v, want ErrUserNotFound", err)
	}
	if err := manager.CreateUser(ctx, mts.User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser(recreate) error = %v", err)
	}
	if err := manager.CheckDatabasePermission(
		ctx,
		"alice",
		"metrics",
		mts.DatabasePermissionRead,
	); !errors.Is(err, mts.ErrPermissionDenied) {
		t.Fatalf("CheckDatabasePermission(recreated user) error = %v, want ErrPermissionDenied", err)
	}
}

func TestLocalUserManagerCheckPermissionFailsClosedForMissingUser(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)

	if err := manager.CheckDatabasePermission(
		ctx,
		"missing",
		"metrics",
		mts.DatabasePermissionRead,
	); !errors.Is(err, mts.ErrPermissionDenied) {
		t.Fatalf("CheckDatabasePermission(missing user) error = %v, want ErrPermissionDenied", err)
	}
}

func TestLocalUserManagerRejectsInvalidPermissions(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)

	err := manager.GrantDatabasePermission(ctx, "missing", "metrics", mts.DatabasePermissionRead)
	if !errors.Is(err, mts.ErrUserNotFound) {
		t.Fatalf("GrantDatabasePermission(missing user) error = %v, want ErrUserNotFound", err)
	}
	if err := manager.CreateUser(ctx, mts.User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	err = manager.GrantDatabasePermission(ctx, "alice", "", mts.DatabasePermissionRead)
	if !errors.Is(err, mts.ErrInvalidPermission) {
		t.Fatalf("GrantDatabasePermission(empty database) error = %v, want ErrInvalidPermission", err)
	}
	err = manager.GrantDatabasePermission(ctx, "alice", "metrics", mts.DatabasePermission("owner"))
	if !errors.Is(err, mts.ErrInvalidPermission) {
		t.Fatalf("GrantDatabasePermission(invalid permission) error = %v, want ErrInvalidPermission", err)
	}
	err = manager.RevokeDatabasePermission(ctx, "missing", "metrics", mts.DatabasePermissionRead)
	if !errors.Is(err, mts.ErrUserNotFound) {
		t.Fatalf("RevokeDatabasePermission(missing user) error = %v, want ErrUserNotFound", err)
	}
	_, err = manager.ListDatabasePermissions(ctx, "missing")
	if !errors.Is(err, mts.ErrUserNotFound) {
		t.Fatalf("ListDatabasePermissions(missing user) error = %v, want ErrUserNotFound", err)
	}
	err = manager.CheckDatabasePermission(ctx, "alice", "", mts.DatabasePermissionRead)
	if !errors.Is(err, mts.ErrInvalidPermission) {
		t.Fatalf("CheckDatabasePermission(empty database) error = %v, want ErrInvalidPermission", err)
	}
}

func grantReadWritePermissions(
	t *testing.T,
	ctx context.Context,
	manager *mts.LocalUserManager,
) {
	t.Helper()
	for _, permission := range []mts.DatabasePermission{
		mts.DatabasePermissionRead,
		mts.DatabasePermissionWrite,
	} {
		if err := manager.GrantDatabasePermission(ctx, "alice", "metrics", permission); err != nil {
			t.Fatalf("GrantDatabasePermission(%s) error = %v", permission, err)
		}
	}
}

func grantAdminPermission(t *testing.T, ctx context.Context, manager *mts.LocalUserManager) {
	t.Helper()
	if err := manager.GrantDatabasePermission(
		ctx,
		"alice",
		"metrics",
		mts.DatabasePermissionAdmin,
	); err != nil {
		t.Fatalf("GrantDatabasePermission(admin) error = %v", err)
	}
}

func assertPermissionAllowed(
	t *testing.T,
	ctx context.Context,
	manager *mts.LocalUserManager,
	userName string,
	database string,
	permission mts.DatabasePermission,
) {
	t.Helper()
	if err := manager.CheckDatabasePermission(ctx, userName, database, permission); err != nil {
		t.Fatalf("CheckDatabasePermission(%s) error = %v", permission, err)
	}
}

func assertPermissionDenied(
	t *testing.T,
	ctx context.Context,
	manager *mts.LocalUserManager,
	userName string,
	database string,
	permission mts.DatabasePermission,
) {
	t.Helper()
	err := manager.CheckDatabasePermission(ctx, userName, database, permission)
	if !errors.Is(err, mts.ErrPermissionDenied) {
		t.Fatalf("CheckDatabasePermission(%s) error = %v, want ErrPermissionDenied", permission, err)
	}
}
