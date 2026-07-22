package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	mts "github.com/openmts/mts"
)

func TestMustChangePasswordGateAndClear(t *testing.T) {
	runtime := openTestRuntime(t)
	// 测试 helper 会清掉强制改密；此处重新打开以验证门禁。
	ctx := context.Background()
	user, ok, err := runtime.engine.GetUser(ctx, "admin")
	if err != nil || !ok {
		t.Fatalf("GetUser(admin) ok=%v err=%v", ok, err)
	}
	user.Metadata = withMustChangePassword(user.Metadata, true)
	if err := runtime.engine.UpdateUser(ctx, user); err != nil {
		t.Fatalf("UpdateUser must_change error = %v", err)
	}
	server := httptest.NewServer(runtime.httpHandler())
	t.Cleanup(server.Close)

	// bootstrap admin 登录应标记 must_change_password
	var login authTokenResponse
	postJSONWithHeaders(t, server.URL+"/api/v1/auth/login", loginRequest{
		UserName: "admin",
		Password: "admin",
	}, nil, http.StatusOK, &login)
	if !login.MustChangePassword {
		t.Fatal("expected must_change_password=true for bootstrap admin")
	}
	if login.Token.Token == "" {
		t.Fatal("empty token")
	}
	auth := map[string]string{"Authorization": "Bearer " + login.Token.Token}

	// 业务写接口应被拦截
	postJSONWithHeaders(t, server.URL+"/api/v1/data/write", writeRequest{
		Points:  []mts.Point{testPoint()},
		Options: mts.WriteOptions{Sync: true},
	}, auth, http.StatusForbidden, &errorResponse{})

	// 改密后应清除标记
	postJSONWithHeaders(t, server.URL+"/api/v1/auth/password", changePasswordRequest{
		UserName:    "admin",
		OldPassword: "admin",
		NewPassword: "admin-changed",
	}, auth, http.StatusOK, &okResponse{})

	// ChangePassword 会撤销旧 token，需要重新登录
	token := loginHTTPUser(t, server.URL, "admin", "admin-changed")
	var login2 authTokenResponse
	postJSONWithHeaders(t, server.URL+"/api/v1/auth/login", loginRequest{
		UserName: "admin",
		Password: "admin-changed",
	}, nil, http.StatusOK, &login2)
	if login2.MustChangePassword {
		t.Fatal("expected must_change_password=false after password change")
	}
	auth2 := map[string]string{"Authorization": "Bearer " + token}
	postJSONWithHeaders(t, server.URL+"/api/v1/data/write", writeRequest{
		Points:  []mts.Point{testPoint()},
		Options: mts.WriteOptions{Sync: true},
	}, auth2, http.StatusOK, &writeResponse{})
}

func TestUserMustChangePasswordHelpers(t *testing.T) {
	if userMustChangePassword(mts.User{}) {
		t.Fatal("empty user should not require change")
	}
	u := mts.User{Metadata: withMustChangePassword(nil, true)}
	if !userMustChangePassword(u) {
		t.Fatal("expected must change")
	}
	u.Metadata = withMustChangePassword(u.Metadata, false)
	if userMustChangePassword(u) {
		t.Fatal("expected cleared")
	}
}

func TestMapChangePasswordError(t *testing.T) {
	if err := mapChangePasswordError(nil); err != nil {
		t.Fatalf("nil => %v", err)
	}
	err := mapChangePasswordError(mts.ErrInvalidCredentials)
	var apiErr apiError
	if !errors.As(err, &apiErr) || apiErr.Code != errorCodeBadRequest {
		t.Fatalf("invalid credentials => %+v", err)
	}
	err = mapChangePasswordError(mts.ErrAuthenticationDisabled)
	if !errors.As(err, &apiErr) || apiErr.Code != errorCodePermissionDenied {
		t.Fatalf("auth disabled => %+v", err)
	}
	cause := errors.New("boom")
	if got := mapChangePasswordError(cause); !errors.Is(got, cause) {
		t.Fatalf("passthrough = %v", got)
	}
}

func TestValidateUserPassword(t *testing.T) {
	if err := validateUserPassword(""); err == nil {
		t.Fatal("empty password should fail")
	}
	if err := validateUserPassword("short"); err == nil {
		t.Fatal("short password should fail")
	}
	if err := validateUserPassword("admin"); err == nil {
		t.Fatal("default admin password should fail")
	}
	if err := validateUserPassword("goodpass1"); err != nil {
		t.Fatalf("valid password error = %v", err)
	}
}
