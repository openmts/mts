package runtime

import (
	"github.com/openmts/mts/internal/user"
)

// UserOptions 描述运行时用户模块的接入配置。
type UserOptions struct {
	Endpoint             string
	PasswordAuthDisabled bool
}

// OpenUserManager 打开默认用户管理器运行时。
func OpenUserManager(dataDir string, opts UserOptions) (*user.Manager, error) {
	return user.OpenWithOptions(dataDir, user.Options{
		Endpoint:             opts.Endpoint,
		PasswordAuthDisabled: opts.PasswordAuthDisabled,
	})
}
