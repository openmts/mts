package mts_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

func TestUserManagementRejectsCorruptLocalFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "users.bin"), []byte("not-envelope"), 0600); err != nil {
		t.Fatalf("WriteFile(corrupt users.bin) error = %v", err)
	}
	if eng, err := mts.Open(t.Context(), mts.DefaultOptions(dir)); err == nil {
		_ = eng.Close(t.Context())
		t.Fatal("Open(corrupt users.bin) error = nil, want error")
	}
}

func TestEngineUsesDefaultUserManager(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	eng, err := mts.Open(ctx, mts.DefaultOptions(dir))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.CreateUser(ctx, mts.User{Name: "alice"}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("CreateUser() error = %v close=%v", err, closeErr)
	}
	if err := eng.GrantDatabasePermission(
		ctx,
		"alice",
		"metrics",
		mts.DatabasePermissionRead,
	); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("GrantDatabasePermission() error = %v close=%v", err, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := mts.Open(ctx, mts.DefaultOptions(dir))
	if err != nil {
		t.Fatalf("Open(reopen) error = %v", err)
	}
	defer closeEngine(t, reopened)
	if err := reopened.CheckUserDatabasePermission(
		ctx,
		"alice",
		"metrics",
		mts.DatabasePermissionRead,
	); err != nil {
		t.Fatalf("CheckUserDatabasePermission(reopen) error = %v", err)
	}
}

func TestEngineUsesInjectedUserManager(t *testing.T) {
	ctx := context.Background()
	manager := &recordingUserManager{}
	opts := mts.DefaultOptions(t.TempDir())
	opts.UserManager = manager
	eng, err := mts.Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer closeEngine(t, eng)

	if err := eng.CreateUser(ctx, mts.User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if !manager.created {
		t.Fatal("injected user manager was not used")
	}
	if _, err := os.Stat(filepath.Join(opts.Path, "users.bin")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("default users.bin stat error = %v, want os.ErrNotExist", err)
	}
}

func TestEngineUserManagementWrappers(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.DefaultOptions(t.TempDir()))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer closeEngine(t, eng)

	if err := eng.CreateUser(ctx, mts.User{Name: "bob", DisplayName: "Bob"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := eng.UpdateUser(ctx, mts.User{Name: "bob", DisplayName: "Robert"}); err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	assertEngineUser(t, ctx, eng)
	assertEnginePermissions(t, ctx, eng)
	assertEngineAuthentication(t, ctx, eng)

	if err := eng.DeleteUser(ctx, "bob"); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	if _, ok, err := eng.GetUser(ctx, "bob"); err != nil || ok {
		t.Fatalf("GetUser(deleted) error = %v ok=%v, want missing user", err, ok)
	}
}

func assertEngineUser(t *testing.T, ctx context.Context, eng *mts.Engine) {
	t.Helper()
	user, ok, err := eng.GetUser(ctx, "bob")
	if err != nil || !ok {
		t.Fatalf("GetUser() error = %v ok=%v", err, ok)
	}
	if user.DisplayName != "Robert" {
		t.Fatalf("DisplayName = %q, want Robert", user.DisplayName)
	}
	users, err := eng.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 1 || users[0].Name != "bob" {
		t.Fatalf("ListUsers() = %#v, want bob only", users)
	}
}

func assertEnginePermissions(t *testing.T, ctx context.Context, eng *mts.Engine) {
	t.Helper()
	grantEnginePermissions(t, ctx, eng)
	grants, err := eng.ListDatabasePermissions(ctx, "bob")
	if err != nil {
		t.Fatalf("ListDatabasePermissions() error = %v", err)
	}
	if len(grants) != 2 || grants[0].Permission != mts.DatabasePermissionRead {
		t.Fatalf("ListDatabasePermissions() = %#v, want read/write", grants)
	}
	if err := eng.RevokeDatabasePermission(ctx, "bob", "metrics", mts.DatabasePermissionRead); err != nil {
		t.Fatalf("RevokeDatabasePermission() error = %v", err)
	}
	if err := eng.CheckUserDatabasePermission(ctx, "bob", "metrics", mts.DatabasePermissionRead); !errors.Is(err, mts.ErrPermissionDenied) {
		t.Fatalf("CheckUserDatabasePermission(read) error = %v, want permission denied", err)
	}
	if err := eng.CheckUserDatabasePermission(ctx, "bob", "metrics", mts.DatabasePermissionWrite); err != nil {
		t.Fatalf("CheckUserDatabasePermission(write) error = %v", err)
	}
}

func grantEnginePermissions(t *testing.T, ctx context.Context, eng *mts.Engine) {
	t.Helper()
	if err := eng.GrantDatabasePermission(ctx, "bob", "metrics", mts.DatabasePermissionRead); err != nil {
		t.Fatalf("GrantDatabasePermission(read) error = %v", err)
	}
	if err := eng.GrantDatabasePermission(ctx, "bob", "metrics", mts.DatabasePermissionWrite); err != nil {
		t.Fatalf("GrantDatabasePermission(write) error = %v", err)
	}
}

func assertEngineAuthentication(t *testing.T, ctx context.Context, eng *mts.Engine) {
	t.Helper()
	if err := eng.SetPassword(ctx, "bob", "secret"); err != nil {
		t.Fatalf("SetPassword() error = %v", err)
	}
	token, err := eng.Authenticate(ctx, mts.Credentials{UserName: "bob", Password: "secret"}, time.Hour)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	principal, err := eng.VerifyToken(ctx, token.Token)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	if principal.UserName != "bob" {
		t.Fatalf("principal = %#v, want bob", principal)
	}
	if err := eng.ChangePassword(ctx, "bob", "secret", "next"); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if _, err := eng.Authenticate(ctx, mts.Credentials{UserName: "bob", Password: "secret"}, time.Hour); !errors.Is(err, mts.ErrInvalidCredentials) {
		t.Fatalf("Authenticate(old password) error = %v, want ErrInvalidCredentials", err)
	}
}

func closeEngine(t *testing.T, engine *mts.Engine) {
	t.Helper()
	if err := engine.Close(t.Context()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

type recordingUserManager struct {
	created bool
}

func (m *recordingUserManager) CreateUser(_ context.Context, _ mts.User) error {
	m.created = true
	return nil
}

func (m *recordingUserManager) UpdateUser(context.Context, mts.User) error {
	return nil
}

func (m *recordingUserManager) GetUser(context.Context, string) (mts.User, bool, error) {
	return mts.User{}, false, nil
}

func (m *recordingUserManager) ListUsers(context.Context) ([]mts.User, error) {
	return nil, nil
}

func (m *recordingUserManager) DeleteUser(context.Context, string) error {
	return nil
}

func (m *recordingUserManager) GrantDatabasePermission(
	context.Context,
	string,
	string,
	mts.DatabasePermission,
) error {
	return nil
}

func (m *recordingUserManager) RevokeDatabasePermission(
	context.Context,
	string,
	string,
	mts.DatabasePermission,
) error {
	return nil
}

func (m *recordingUserManager) ListDatabasePermissions(
	context.Context,
	string,
) ([]mts.DatabaseGrant, error) {
	return nil, nil
}

func (m *recordingUserManager) CheckDatabasePermission(
	context.Context,
	string,
	string,
	mts.DatabasePermission,
) error {
	return nil
}

func (m *recordingUserManager) SetPassword(context.Context, string, string) error {
	return nil
}

func (m *recordingUserManager) ChangePassword(context.Context, string, string, string) error {
	return nil
}

func (m *recordingUserManager) Authenticate(context.Context, mts.Credentials, time.Duration) (mts.AuthToken, error) {
	return mts.AuthToken{}, nil
}

func (m *recordingUserManager) VerifyToken(context.Context, string) (mts.Principal, error) {
	return mts.Principal{}, nil
}

func (m *recordingUserManager) RevokeToken(context.Context, string) error {
	return nil
}
