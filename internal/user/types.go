package user

import (
	"errors"
	"time"
)

var (
	ErrInvalidUser            = errors.New("invalid user")
	ErrInvalidPageLimit       = errors.New("invalid page limit")
	ErrUserNotFound           = errors.New("user not found")
	ErrUserAlreadyExists      = errors.New("user already exists")
	ErrInvalidPermission      = errors.New("invalid permission")
	ErrPermissionDenied       = errors.New("permission denied")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrAuthenticationDisabled = errors.New("authentication disabled")
	ErrUnsupportedEndpoint    = errors.New("unsupported user endpoint")
)

const MaxGrantPageLimit = 200

const EndpointLocal = "local"

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type Options struct {
	Endpoint             string
	PasswordAuthDisabled bool
}

func DefaultOptions() Options {
	return Options{Endpoint: EndpointLocal}
}

type Permission string

const (
	PermissionRead  Permission = "read"
	PermissionWrite Permission = "write"
	PermissionAdmin Permission = "admin"
)

type User struct {
	Name        string
	DisplayName string
	Role        Role
	Disabled    bool
	Metadata    map[string]string
}

type Grant struct {
	Database   string
	Permission Permission
}

type UserGrantBundle struct {
	User   User
	Grants []Grant
}

type UserGrantPage struct {
	Items      []UserGrantBundle
	TotalUsers int
	NextCursor string
}

type Credentials struct {
	UserName string
	Password string
}

type AuthToken struct {
	Token     string
	UserName  string
	Role      Role
	ExpiresAt time.Time
}

type Principal struct {
	UserName  string
	Role      Role
	ExpiresAt time.Time
}
