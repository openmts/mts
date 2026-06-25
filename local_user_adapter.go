package mts

import (
	"context"

	internaluser "github.com/openmts/mts/internal/user"
)

type localUserManager struct {
	inner *internaluser.Manager
}

var _ UserManager = (*localUserManager)(nil)

func openLocalUserManager(dir string) (*localUserManager, error) {
	inner, err := internaluser.Open(dir)
	if err != nil {
		return nil, err
	}
	return &localUserManager{inner: inner}, nil
}

func (m *localUserManager) Close() error {
	return m.inner.Close()
}

func (m *localUserManager) CreateUser(ctx context.Context, user User) error {
	return m.inner.CreateUser(ctx, toInternalUser(user))
}

func (m *localUserManager) UpdateUser(ctx context.Context, user User) error {
	return m.inner.UpdateUser(ctx, toInternalUser(user))
}

func (m *localUserManager) GetUser(ctx context.Context, name string) (User, bool, error) {
	user, ok, err := m.inner.GetUser(ctx, name)
	return fromInternalUser(user), ok, err
}

func (m *localUserManager) ListUsers(ctx context.Context) ([]User, error) {
	users, err := m.inner.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]User, len(users))
	for index, user := range users {
		out[index] = fromInternalUser(user)
	}
	return out, nil
}

func (m *localUserManager) DeleteUser(ctx context.Context, name string) error {
	return m.inner.DeleteUser(ctx, name)
}

func (m *localUserManager) GrantDatabasePermission(
	ctx context.Context,
	userName string,
	database string,
	permission DatabasePermission,
) error {
	return m.inner.GrantPermission(ctx, userName, database, toInternalPermission(permission))
}

func (m *localUserManager) RevokeDatabasePermission(
	ctx context.Context,
	userName string,
	database string,
	permission DatabasePermission,
) error {
	return m.inner.RevokePermission(ctx, userName, database, toInternalPermission(permission))
}

func (m *localUserManager) ListDatabasePermissions(ctx context.Context, userName string) ([]DatabaseGrant, error) {
	grants, err := m.inner.ListPermissions(ctx, userName)
	if err != nil {
		return nil, err
	}
	out := make([]DatabaseGrant, len(grants))
	for index, grant := range grants {
		out[index] = DatabaseGrant{Database: grant.Database, Permission: DatabasePermission(grant.Permission)}
	}
	return out, nil
}

func (m *localUserManager) CheckDatabasePermission(
	ctx context.Context,
	userName string,
	database string,
	permission DatabasePermission,
) error {
	return m.inner.CheckPermission(ctx, userName, database, toInternalPermission(permission))
}

func toInternalUser(user User) internaluser.User {
	return internaluser.User{
		Name:        user.Name,
		DisplayName: user.DisplayName,
		Disabled:    user.Disabled,
		Metadata:    user.Metadata,
	}
}

func fromInternalUser(user internaluser.User) User {
	return User{
		Name:        user.Name,
		DisplayName: user.DisplayName,
		Disabled:    user.Disabled,
		Metadata:    user.Metadata,
	}
}

func toInternalPermission(permission DatabasePermission) internaluser.Permission {
	return internaluser.Permission(permission)
}
