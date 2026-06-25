package user

import "errors"

var (
	ErrInvalidUser       = errors.New("invalid user")
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrInvalidPermission = errors.New("invalid permission")
	ErrPermissionDenied  = errors.New("permission denied")
)

type Permission string

const (
	PermissionRead  Permission = "read"
	PermissionWrite Permission = "write"
	PermissionAdmin Permission = "admin"
)

type User struct {
	Name        string
	DisplayName string
	Disabled    bool
	Metadata    map[string]string
}

type Grant struct {
	Database   string
	Permission Permission
}
