package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestRuntimeEngineUsesDefaultUserManager(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	engine := openTestRuntimeEngine(t, ctx, Options{
		Storage: model.Options{Path: dir, ShardDuration: time.Hour},
	})
	if err := engine.Users().CreateUser(ctx, User{Name: "alice"}); err != nil {
		closeErr := engine.Close(ctx)
		t.Fatalf("CreateUser() error = %v close=%v", err, closeErr)
	}
	closeTestRuntimeEngine(t, ctx, engine)

	reopened := openTestRuntimeEngine(t, ctx, Options{
		Storage: model.Options{Path: dir, ShardDuration: time.Hour},
	})
	defer closeTestRuntimeEngine(t, ctx, reopened)

	runtimeUser, ok, err := reopened.Users().GetUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if !ok || runtimeUser.Name != "alice" {
		t.Fatalf("GetUser() = %#v, %v; want alice true", runtimeUser, ok)
	}
}

func TestRuntimeEngineUsesInjectedUserManager(t *testing.T) {
	ctx := context.Background()
	manager := &recordingRuntimeUserManager{}
	engine := openTestRuntimeEngine(t, ctx, Options{
		Storage:     model.Options{Path: t.TempDir(), ShardDuration: time.Hour},
		UserManager: manager,
	})
	defer closeTestRuntimeEngine(t, ctx, engine)

	if err := engine.Users().CreateUser(ctx, User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if manager.createCalls != 1 {
		t.Fatalf("create calls = %d, want 1", manager.createCalls)
	}
	if manager.closed {
		t.Fatal("injected user manager was closed by runtime")
	}
}

func openTestRuntimeEngine(t *testing.T, ctx context.Context, opts Options) *Engine {
	t.Helper()
	engine, err := OpenEngine(ctx, opts)
	if err != nil {
		t.Fatalf("OpenEngine() error = %v", err)
	}
	return engine
}

func closeTestRuntimeEngine(t *testing.T, ctx context.Context, engine *Engine) {
	t.Helper()
	if err := engine.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

type recordingRuntimeUserManager struct {
	createCalls int
	closed      bool
}

func (m *recordingRuntimeUserManager) Close() error {
	m.closed = true
	return nil
}

func (m *recordingRuntimeUserManager) CreateUser(context.Context, User) error {
	m.createCalls++
	return nil
}

func (m *recordingRuntimeUserManager) UpdateUser(context.Context, User) error {
	return nil
}

func (m *recordingRuntimeUserManager) GetUser(context.Context, string) (User, bool, error) {
	return User{}, false, nil
}

func (m *recordingRuntimeUserManager) ListUsers(context.Context) ([]User, error) {
	return nil, nil
}

func (m *recordingRuntimeUserManager) DeleteUser(context.Context, string) error {
	return nil
}

func (m *recordingRuntimeUserManager) GrantDatabasePermission(
	context.Context,
	string,
	string,
	DatabasePermission,
) error {
	return nil
}

func (m *recordingRuntimeUserManager) RevokeDatabasePermission(
	context.Context,
	string,
	string,
	DatabasePermission,
) error {
	return nil
}

func (m *recordingRuntimeUserManager) ListDatabasePermissions(context.Context, string) ([]DatabaseGrant, error) {
	return nil, nil
}

func (m *recordingRuntimeUserManager) CheckDatabasePermission(
	context.Context,
	string,
	string,
	DatabasePermission,
) error {
	return nil
}

func (m *recordingRuntimeUserManager) SetPassword(context.Context, string, string) error {
	return nil
}

func (m *recordingRuntimeUserManager) ChangePassword(context.Context, string, string, string) error {
	return nil
}

func (m *recordingRuntimeUserManager) Authenticate(context.Context, Credentials, time.Duration) (AuthToken, error) {
	return AuthToken{}, nil
}

func (m *recordingRuntimeUserManager) VerifyToken(context.Context, string) (Principal, error) {
	return Principal{}, nil
}

func (m *recordingRuntimeUserManager) RevokeToken(context.Context, string) error {
	return nil
}
