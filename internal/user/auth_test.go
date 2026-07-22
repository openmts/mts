package user

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestManagerPasswordAuthenticationLifecycle(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)
	if err := manager.CreateUser(ctx, User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := manager.SetPassword(ctx, "alice", "secret"); err != nil {
		t.Fatalf("SetPassword() error = %v", err)
	}
	if err := manager.VerifyPassword(ctx, "alice", "bad"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("VerifyPassword(bad) error = %v, want ErrInvalidCredentials", err)
	}
	token, err := manager.Authenticate(ctx, Credentials{UserName: "alice", Password: "secret"}, time.Hour)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if token.Token == "" || token.UserName != "alice" || token.Role != RoleUser || !token.ExpiresAt.After(time.Now()) {
		t.Fatalf("token = %#v, want populated token with user role", token)
	}
	principal, err := manager.VerifyToken(ctx, token.Token)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	if principal.UserName != "alice" {
		t.Fatalf("principal = %#v, want alice", principal)
	}
	if principal.Role != RoleUser {
		t.Fatalf("principal.Role = %q, want user", principal.Role)
	}
	if principal.ExpiresAt.IsZero() || principal.ExpiresAt.Before(time.Now()) {
		t.Fatalf("principal.ExpiresAt = %v, want future", principal.ExpiresAt)
	}
	if err := manager.RevokeToken(ctx, token.Token); err != nil {
		t.Fatalf("RevokeToken() error = %v", err)
	}
	if _, err := manager.VerifyToken(ctx, token.Token); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("VerifyToken(revoked) error = %v, want ErrInvalidCredentials", err)
	}
}

func TestManagerAuthenticationFailsClosed(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)
	if err := manager.CreateUser(ctx, User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if _, err := manager.Authenticate(ctx, Credentials{UserName: "alice", Password: "secret"}, time.Hour); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Authenticate(no password) error = %v, want ErrInvalidCredentials", err)
	}
	if err := manager.SetPassword(ctx, "alice", "secret"); err != nil {
		t.Fatalf("SetPassword() error = %v", err)
	}
	if _, err := manager.Authenticate(ctx, Credentials{UserName: "alice", Password: "bad"}, time.Hour); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Authenticate(bad password) error = %v, want ErrInvalidCredentials", err)
	}
	if _, err := manager.Authenticate(ctx, Credentials{UserName: "alice", Password: "secret"}, 0); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Authenticate(zero ttl) error = %v, want ErrInvalidCredentials", err)
	}
	token, err := manager.Authenticate(ctx, Credentials{UserName: "alice", Password: "secret"}, time.Hour)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if err := manager.UpdateUser(ctx, User{Name: "alice", Disabled: true}); err != nil {
		t.Fatalf("UpdateUser(disabled) error = %v", err)
	}
	if _, err := manager.VerifyToken(ctx, token.Token); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("VerifyToken(disabled user) error = %v, want ErrInvalidCredentials", err)
	}
	if _, err := manager.Authenticate(ctx, Credentials{UserName: "alice", Password: "secret"}, time.Hour); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Authenticate(disabled user) error = %v, want ErrInvalidCredentials", err)
	}
}

func TestManagerVerifyTokenRejectsExpiredToken(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)
	if err := manager.CreateUser(ctx, User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := manager.SetPassword(ctx, "alice", "secret"); err != nil {
		t.Fatalf("SetPassword() error = %v", err)
	}
	token, err := manager.Authenticate(ctx, Credentials{UserName: "alice", Password: "secret"}, time.Nanosecond)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	time.Sleep(time.Millisecond)
	if _, err := manager.VerifyToken(ctx, token.Token); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("VerifyToken(expired) error = %v, want ErrInvalidCredentials", err)
	}
}

func TestManagerChangePasswordRevokesUserTokens(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)
	if err := manager.CreateUser(ctx, User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := manager.SetPassword(ctx, "alice", "secret"); err != nil {
		t.Fatalf("SetPassword() error = %v", err)
	}
	token, err := manager.Authenticate(ctx, Credentials{UserName: "alice", Password: "secret"}, time.Hour)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if err := manager.ChangePassword(ctx, "alice", "bad", "next"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("ChangePassword(bad old) error = %v, want ErrInvalidCredentials", err)
	}
	if err := manager.ChangePassword(ctx, "alice", "secret", "next"); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if _, err := manager.VerifyToken(ctx, token.Token); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("VerifyToken(old token) error = %v, want ErrInvalidCredentials", err)
	}
	if _, err := manager.Authenticate(ctx, Credentials{UserName: "alice", Password: "secret"}, time.Hour); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Authenticate(old password) error = %v, want ErrInvalidCredentials", err)
	}
	if _, err := manager.Authenticate(ctx, Credentials{UserName: "alice", Password: "next"}, time.Hour); err != nil {
		t.Fatalf("Authenticate(new password) error = %v", err)
	}
}

func TestManagerAuthenticationPersistsTokenAndPassword(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	manager, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := manager.CreateUser(ctx, User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := manager.SetPassword(ctx, "alice", "secret"); err != nil {
		t.Fatalf("SetPassword() error = %v", err)
	}
	token, err := manager.Authenticate(ctx, Credentials{UserName: "alice", Password: "secret"}, time.Hour)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	closeUserManager(t, manager)

	reopened, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(reopen) error = %v", err)
	}
	defer closeUserManager(t, reopened)
	if err := reopened.VerifyPassword(ctx, "alice", "secret"); err != nil {
		t.Fatalf("VerifyPassword(reopen) error = %v", err)
	}
	if _, err := reopened.VerifyToken(ctx, token.Token); err != nil {
		t.Fatalf("VerifyToken(reopen) error = %v", err)
	}
}

func TestManagerPasswordAuthDisabled(t *testing.T) {
	ctx := context.Background()
	manager, err := OpenWithOptions(t.TempDir(), Options{PasswordAuthDisabled: true})
	if err != nil {
		t.Fatalf("OpenWithOptions() error = %v", err)
	}
	defer closeUserManager(t, manager)
	if err := manager.CreateUser(ctx, User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := manager.SetPassword(ctx, "alice", "secret"); !errors.Is(err, ErrAuthenticationDisabled) {
		t.Fatalf("SetPassword(disabled) error = %v, want ErrAuthenticationDisabled", err)
	}
	if _, err := manager.Authenticate(ctx, Credentials{UserName: "alice", Password: "secret"}, time.Hour); !errors.Is(err, ErrAuthenticationDisabled) {
		t.Fatalf("Authenticate(disabled) error = %v, want ErrAuthenticationDisabled", err)
	}
	if _, err := manager.VerifyToken(ctx, "token"); !errors.Is(err, ErrAuthenticationDisabled) {
		t.Fatalf("VerifyToken(disabled) error = %v, want ErrAuthenticationDisabled", err)
	}
	if err := manager.ChangePassword(ctx, "alice", "old", "new"); !errors.Is(err, ErrAuthenticationDisabled) {
		t.Fatalf("ChangePassword(disabled) error = %v, want ErrAuthenticationDisabled", err)
	}
	if err := manager.RevokeToken(ctx, "token"); !errors.Is(err, ErrAuthenticationDisabled) {
		t.Fatalf("RevokeToken(disabled) error = %v, want ErrAuthenticationDisabled", err)
	}
}

func TestManagerOpenWithOptions(t *testing.T) {
	manager, err := OpenWithOptions(t.TempDir(), Options{Endpoint: " local "})
	if err != nil {
		t.Fatalf("OpenWithOptions(local) error = %v", err)
	}
	if manager.Options().Endpoint != EndpointLocal {
		t.Fatalf("Options().Endpoint = %q, want local", manager.Options().Endpoint)
	}
	closeUserManager(t, manager)

	if _, err := OpenWithOptions(t.TempDir(), Options{Endpoint: "ldap"}); !errors.Is(err, ErrUnsupportedEndpoint) {
		t.Fatalf("OpenWithOptions(ldap) error = %v, want ErrUnsupportedEndpoint", err)
	}
}

func TestManagerSetPasswordRejectsInvalidInput(t *testing.T) {
	ctx := context.Background()
	manager := openTestUserManager(t)
	if err := manager.SetPassword(ctx, "missing", "secret"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("SetPassword(missing) error = %v, want ErrUserNotFound", err)
	}
	if err := manager.CreateUser(ctx, User{Name: "alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := manager.SetPassword(ctx, "alice", " "); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("SetPassword(empty) error = %v, want ErrInvalidCredentials", err)
	}
	if err := manager.RevokeToken(ctx, " "); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("RevokeToken(empty) error = %v, want ErrInvalidCredentials", err)
	}
}
