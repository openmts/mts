package runtime

import (
	"context"
	"time"

	"github.com/openmts/mts/internal/user"
)

type DatabasePermission string

const (
	DatabasePermissionRead  DatabasePermission = "read"
	DatabasePermissionWrite DatabasePermission = "write"
	DatabasePermissionAdmin DatabasePermission = "admin"
)

type UserRole string

const (
	UserRoleUser  UserRole = "user"
	UserRoleAdmin UserRole = "admin"
)

type User struct {
	Name        string
	DisplayName string
	Role        UserRole
	Disabled    bool
	Metadata    map[string]string
}

type DatabaseGrant struct {
	Database   string
	Permission DatabasePermission
}

type Credentials struct {
	UserName string
	Password string
}

type AuthToken struct {
	Token     string
	UserName  string
	Role      UserRole
	ExpiresAt time.Time
}

type Principal struct {
	UserName  string
	Role      UserRole
	ExpiresAt time.Time
}

type UserManager interface {
	CreateUser(context.Context, User) error
	UpdateUser(context.Context, User) error
	GetUser(context.Context, string) (User, bool, error)
	ListUsers(context.Context) ([]User, error)
	DeleteUser(context.Context, string) error
	GrantDatabasePermission(context.Context, string, string, DatabasePermission) error
	RevokeDatabasePermission(context.Context, string, string, DatabasePermission) error
	ListDatabasePermissions(context.Context, string) ([]DatabaseGrant, error)
	CheckDatabasePermission(context.Context, string, string, DatabasePermission) error
	SetPassword(context.Context, string, string) error
	ChangePassword(context.Context, string, string, string) error
	Authenticate(context.Context, Credentials, time.Duration) (AuthToken, error)
	VerifyToken(context.Context, string) (Principal, error)
	RevokeToken(context.Context, string) error
}

type localUserManager struct {
	inner localUserBackend
}

var _ UserManager = (*localUserManager)(nil)

type localUserBackend interface {
	Close() error
	user.UserStore
	user.PermissionStore
	user.CredentialStore
	user.Authenticator
	user.TokenStore
}

func openRuntimeUserManager(dir string, opts UserOptions) (*localUserManager, error) {
	inner, err := OpenUserManager(dir, opts)
	if err != nil {
		return nil, err
	}
	return &localUserManager{inner: inner}, nil
}

func (m *localUserManager) Close() error {
	return m.inner.Close()
}

func (m *localUserManager) CreateUser(ctx context.Context, runtimeUser User) error {
	return m.inner.CreateUser(ctx, toInternalUser(runtimeUser))
}

func (m *localUserManager) UpdateUser(ctx context.Context, runtimeUser User) error {
	return m.inner.UpdateUser(ctx, toInternalUser(runtimeUser))
}

func (m *localUserManager) GetUser(ctx context.Context, name string) (User, bool, error) {
	runtimeUser, ok, err := m.inner.GetUser(ctx, name)
	return fromInternalUser(runtimeUser), ok, err
}

func (m *localUserManager) ListUsers(ctx context.Context) ([]User, error) {
	users, err := m.inner.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]User, len(users))
	for index, runtimeUser := range users {
		out[index] = fromInternalUser(runtimeUser)
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
		out[index] = DatabaseGrant{
			Database:   grant.Database,
			Permission: DatabasePermission(grant.Permission),
		}
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

func (m *localUserManager) SetPassword(ctx context.Context, userName string, password string) error {
	return m.inner.SetPassword(ctx, userName, password)
}

func (m *localUserManager) ChangePassword(
	ctx context.Context,
	userName string,
	oldPassword string,
	newPassword string,
) error {
	return m.inner.ChangePassword(ctx, userName, oldPassword, newPassword)
}

func (m *localUserManager) Authenticate(
	ctx context.Context,
	credentials Credentials,
	ttl time.Duration,
) (AuthToken, error) {
	token, err := m.inner.Authenticate(ctx, user.Credentials{
		UserName: credentials.UserName,
		Password: credentials.Password,
	}, ttl)
	if err != nil {
		return AuthToken{}, err
	}
	return AuthToken{
		Token:     token.Token,
		UserName:  token.UserName,
		Role:      UserRole(token.Role),
		ExpiresAt: token.ExpiresAt,
	}, nil
}

func (m *localUserManager) VerifyToken(ctx context.Context, token string) (Principal, error) {
	principal, err := m.inner.VerifyToken(ctx, token)
	if err != nil {
		return Principal{}, err
	}
	return Principal{
		UserName:  principal.UserName,
		Role:      UserRole(principal.Role),
		ExpiresAt: principal.ExpiresAt,
	}, nil
}

func (m *localUserManager) RevokeToken(ctx context.Context, token string) error {
	return m.inner.RevokeToken(ctx, token)
}

func toInternalUser(runtimeUser User) user.User {
	return user.User{
		Name:        runtimeUser.Name,
		DisplayName: runtimeUser.DisplayName,
		Role:        user.Role(runtimeUser.Role),
		Disabled:    runtimeUser.Disabled,
		Metadata:    runtimeUser.Metadata,
	}
}

func fromInternalUser(runtimeUser user.User) User {
	return User{
		Name:        runtimeUser.Name,
		DisplayName: runtimeUser.DisplayName,
		Role:        UserRole(runtimeUser.Role),
		Disabled:    runtimeUser.Disabled,
		Metadata:    runtimeUser.Metadata,
	}
}

func toInternalPermission(permission DatabasePermission) user.Permission {
	return user.Permission(permission)
}
