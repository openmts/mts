package runtime

import "github.com/openmts/mts/internal/user"

var (
	ErrInvalidUser             = user.ErrInvalidUser
	ErrUserNotFound            = user.ErrUserNotFound
	ErrUserAlreadyExists       = user.ErrUserAlreadyExists
	ErrInvalidPermission       = user.ErrInvalidPermission
	ErrPermissionDenied        = user.ErrPermissionDenied
	ErrInvalidCredentials      = user.ErrInvalidCredentials
	ErrAuthenticationDisabled  = user.ErrAuthenticationDisabled
	ErrUnsupportedUserEndpoint = user.ErrUnsupportedEndpoint
)
