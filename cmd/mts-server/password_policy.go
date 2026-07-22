package main

import (
	"context"
	"errors"
	"strings"

	mts "github.com/openmts/mts"
)

// 用户 metadata 键：标记是否仍使用 bootstrap 默认密码，需强制修改。
const userMetaMustChangePassword = "must_change_password"

func userMustChangePassword(user mts.User) bool {
	if user.Metadata == nil {
		return false
	}
	v := strings.TrimSpace(strings.ToLower(user.Metadata[userMetaMustChangePassword]))
	return v == "1" || v == "true" || v == "yes"
}

func withMustChangePassword(meta map[string]string, required bool) map[string]string {
	out := make(map[string]string, len(meta)+1)
	for k, v := range meta {
		out[k] = v
	}
	if required {
		out[userMetaMustChangePassword] = "1"
	} else {
		delete(out, userMetaMustChangePassword)
	}
	return out
}

func (r *serverRuntime) clearMustChangePassword(ctx context.Context, userName string) error {
	user, ok, err := r.engine.GetUser(ctx, userName)
	if err != nil {
		return err
	}
	if !ok || !userMustChangePassword(user) {
		return nil
	}
	user.Metadata = withMustChangePassword(user.Metadata, false)
	return r.engine.UpdateUser(ctx, user)
}

func (r *serverRuntime) enforcePasswordChangeGate(ctx context.Context, userName string, path string) error {
	userName = strings.TrimSpace(userName)
	if userName == "" {
		return nil
	}
	// 允许改密、登出与只读会话自检类路径；其余 API 在仍使用默认密码时拒绝。
	if passwordChangeAllowedPath(path) {
		return nil
	}
	user, ok, err := r.engine.GetUser(ctx, userName)
	if err != nil {
		return err
	}
	if !ok || !userMustChangePassword(user) {
		return nil
	}
	return newAPIError(
		errorCodePermissionDenied,
		"password change required: default bootstrap password must be changed before continuing",
		nil,
	)
}

func passwordChangeAllowedPath(path string) bool {
	switch strings.TrimSpace(path) {
	case routeAuthPassword, routeAuthLogout, routeAuthLogin, routeAuthSession:
		return true
	default:
		// health/metrics/ready 无用户 token 时不走此门禁
		return false
	}
}

// mapChangePasswordError 将改密失败映射为业务错误码。
// 旧密码错误属于请求校验失败（bad_request），不得用 unauthenticated，避免客户端误清仍有效的会话。
func mapChangePasswordError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, mts.ErrAuthenticationDisabled) {
		return newAPIError(errorCodePermissionDenied, "password authentication disabled", err)
	}
	if errors.Is(err, mts.ErrInvalidCredentials) {
		return newAPIError(errorCodeBadRequest, "invalid credentials", err)
	}
	return err
}

const minUserPasswordLength = 8
const forbiddenDefaultPassword = "admin"

// validateUserPassword 与 Dashboard passwordPolicy 对齐：最短 8 位且禁止默认 admin。
func validateUserPassword(password string) error {
	password = strings.TrimSpace(password)
	if password == "" {
		return newAPIError(errorCodeBadRequest, "password is required", mts.ErrInvalidCredentials)
	}
	if password == forbiddenDefaultPassword {
		return newAPIError(errorCodeBadRequest, "default password admin is not allowed", mts.ErrInvalidCredentials)
	}
	if len(password) < minUserPasswordLength {
		return newAPIError(errorCodeBadRequest, "password must be at least 8 characters", mts.ErrInvalidCredentials)
	}
	return nil
}
