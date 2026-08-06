package user

import (
	"context"
	"time"
)

// UserStore 管理用户资料生命周期。
type UserStore interface {
	CreateUser(context.Context, User) error
	UpdateUser(context.Context, User) error
	GetUser(context.Context, string) (User, bool, error)
	ListUsers(context.Context) ([]User, error)
	DeleteUser(context.Context, string) error
}

// PermissionStore 管理用户到 database 的授权关系。
type PermissionStore interface {
	GrantPermission(context.Context, string, string, Permission) error
	RevokePermission(context.Context, string, string, Permission) error
	ListPermissions(context.Context, string) ([]Grant, error)
	CheckPermission(context.Context, string, string, Permission) error
}

// GrantPageStore 读取用户与授权的一致分页快照。
type GrantPageStore interface {
	ListUserGrantPage(context.Context, string, int) (UserGrantPage, error)
}

// CredentialStore 管理密码凭证。
type CredentialStore interface {
	SetPassword(context.Context, string, string) error
	ChangePassword(context.Context, string, string, string) error
	VerifyPassword(context.Context, string, string) error
}

// TokenStore 管理认证 token 生命周期。
type TokenStore interface {
	VerifyToken(context.Context, string) (Principal, error)
	RevokeToken(context.Context, string) error
}

// Authenticator 负责将凭证交换为 token。
type Authenticator interface {
	Authenticate(context.Context, Credentials, time.Duration) (AuthToken, error)
}

// Authorizer 负责执行 DB 级授权判断。
type Authorizer interface {
	CheckPermission(context.Context, string, string, Permission) error
}
