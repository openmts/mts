package mts

import (
	"context"
	"time"

	"github.com/openmts/mts/internal/runtime"
)

func runtimePermission(permission DatabasePermission) runtime.DatabasePermission {
	return runtime.DatabasePermission(permission)
}

func fromRuntimeGrant(grant runtime.DatabaseGrant) DatabaseGrant {
	return DatabaseGrant{
		Database:   grant.Database,
		Permission: DatabasePermission(grant.Permission),
	}
}

func toRuntimeCredentials(credentials Credentials) runtime.Credentials {
	return runtime.Credentials{
		UserName: credentials.UserName,
		Password: credentials.Password,
	}
}

type runtimeUserManagerAdapter struct {
	inner UserManager
}

var _ runtime.UserManager = runtimeUserManagerAdapter{}

func newRuntimeUserManagerAdapter(inner UserManager) runtime.UserManager {
	if inner == nil {
		return nil
	}
	return runtimeUserManagerAdapter{inner: inner}
}

func (a runtimeUserManagerAdapter) CreateUser(ctx context.Context, user runtime.User) error {
	return a.inner.CreateUser(ctx, fromRuntimeUser(user))
}

func (a runtimeUserManagerAdapter) UpdateUser(ctx context.Context, user runtime.User) error {
	return a.inner.UpdateUser(ctx, fromRuntimeUser(user))
}

func (a runtimeUserManagerAdapter) GetUser(ctx context.Context, name string) (runtime.User, bool, error) {
	user, ok, err := a.inner.GetUser(ctx, name)
	return toRuntimeUser(user), ok, err
}

func (a runtimeUserManagerAdapter) ListUsers(ctx context.Context) ([]runtime.User, error) {
	users, err := a.inner.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]runtime.User, len(users))
	for index, user := range users {
		out[index] = toRuntimeUser(user)
	}
	return out, nil
}

func (a runtimeUserManagerAdapter) DeleteUser(ctx context.Context, name string) error {
	return a.inner.DeleteUser(ctx, name)
}

func (a runtimeUserManagerAdapter) GrantDatabasePermission(
	ctx context.Context,
	userName string,
	database string,
	permission runtime.DatabasePermission,
) error {
	return a.inner.GrantDatabasePermission(ctx, userName, database, DatabasePermission(permission))
}

func (a runtimeUserManagerAdapter) RevokeDatabasePermission(
	ctx context.Context,
	userName string,
	database string,
	permission runtime.DatabasePermission,
) error {
	return a.inner.RevokeDatabasePermission(ctx, userName, database, DatabasePermission(permission))
}

func (a runtimeUserManagerAdapter) ListDatabasePermissions(
	ctx context.Context,
	userName string,
) ([]runtime.DatabaseGrant, error) {
	grants, err := a.inner.ListDatabasePermissions(ctx, userName)
	if err != nil {
		return nil, err
	}
	out := make([]runtime.DatabaseGrant, len(grants))
	for index, grant := range grants {
		out[index] = runtime.DatabaseGrant{
			Database:   grant.Database,
			Permission: runtime.DatabasePermission(grant.Permission),
		}
	}
	return out, nil
}

func (a runtimeUserManagerAdapter) CheckDatabasePermission(
	ctx context.Context,
	userName string,
	database string,
	permission runtime.DatabasePermission,
) error {
	return a.inner.CheckDatabasePermission(ctx, userName, database, DatabasePermission(permission))
}

func (a runtimeUserManagerAdapter) SetPassword(ctx context.Context, userName string, password string) error {
	return a.inner.SetPassword(ctx, userName, password)
}

func (a runtimeUserManagerAdapter) ChangePassword(
	ctx context.Context,
	userName string,
	oldPassword string,
	newPassword string,
) error {
	return a.inner.ChangePassword(ctx, userName, oldPassword, newPassword)
}

func (a runtimeUserManagerAdapter) Authenticate(
	ctx context.Context,
	credentials runtime.Credentials,
	ttl time.Duration,
) (runtime.AuthToken, error) {
	token, err := a.inner.Authenticate(ctx, Credentials{
		UserName: credentials.UserName,
		Password: credentials.Password,
	}, ttl)
	if err != nil {
		return runtime.AuthToken{}, err
	}
	return runtime.AuthToken{
		Token:     token.Token,
		UserName:  token.UserName,
		ExpiresAt: token.ExpiresAt,
	}, nil
}

func (a runtimeUserManagerAdapter) VerifyToken(ctx context.Context, token string) (runtime.Principal, error) {
	principal, err := a.inner.VerifyToken(ctx, token)
	if err != nil {
		return runtime.Principal{}, err
	}
	return runtime.Principal{UserName: principal.UserName}, nil
}

func (a runtimeUserManagerAdapter) RevokeToken(ctx context.Context, token string) error {
	return a.inner.RevokeToken(ctx, token)
}

func toRuntimeUser(user User) runtime.User {
	return runtime.User{
		Name:        user.Name,
		DisplayName: user.DisplayName,
		Role:        runtime.UserRole(user.Role),
		Disabled:    user.Disabled,
		Metadata:    user.Metadata,
	}
}

func fromRuntimeUser(user runtime.User) User {
	return User{
		Name:        user.Name,
		DisplayName: user.DisplayName,
		Role:        UserRole(user.Role),
		Disabled:    user.Disabled,
		Metadata:    user.Metadata,
	}
}
