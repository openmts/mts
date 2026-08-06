package mts

import (
	"time"
)

// DatabasePermission 表示用户对 database 的权限。
type DatabasePermission string

const (
	// DatabasePermissionRead 允许读取 database。
	DatabasePermissionRead DatabasePermission = "read"
	// DatabasePermissionWrite 允许写入 database。
	DatabasePermissionWrite DatabasePermission = "write"
	// DatabasePermissionAdmin 允许管理 database，并隐含 read/write。
	DatabasePermissionAdmin DatabasePermission = "admin"
)

// MaxUserGrantPageLimit 是用户授权聚合读取的单页上限。
const MaxUserGrantPageLimit = 200

// User 表示一个本地或外部权限系统中的用户身份。
type User struct {
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name,omitempty"`
	Role        UserRole          `json:"role,omitempty"`
	Disabled    bool              `json:"disabled,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type UserRole string

const (
	UserRoleUser  UserRole = "user"
	UserRoleAdmin UserRole = "admin"
)

// DatabaseGrant 表示用户在一个 database 上的一项权限。
type DatabaseGrant struct {
	Database   string             `json:"database"`
	Permission DatabasePermission `json:"permission"`
}

// UserGrantBundle 包含一个用户及其 database 授权。
type UserGrantBundle struct {
	User   User            `json:"user"`
	Grants []DatabaseGrant `json:"grants"`
}

// UserGrantPage 是按用户名排序的授权聚合分页结果。
type UserGrantPage struct {
	Items      []UserGrantBundle `json:"items"`
	TotalUsers int               `json:"total_users"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

type Credentials struct {
	UserName string `json:"user_name"`
	Password string `json:"password"`
}

type AuthToken struct {
	Token     string    `json:"token"`
	UserName  string    `json:"user_name"`
	Role      UserRole  `json:"role,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Principal struct {
	UserName  string    `json:"user_name"`
	Role      UserRole  `json:"role,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}
