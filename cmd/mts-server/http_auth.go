package main

import (
	"net/http"
	"strings"
	"time"

	mts "github.com/openmts/mts"
)

const (
	defaultAuthTTL = 12 * time.Hour
	maxAuthTTL     = 30 * 24 * time.Hour
)

func (r *serverRuntime) handleLogin(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodPost) {
		return
	}
	var req loginRequest
	if err := decodeHTTPJSON(request, &req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	ttl := defaultAuthTTL
	if req.TTLSeconds > 0 {
		ttl = time.Duration(req.TTLSeconds) * time.Second
	}
	if ttl > maxAuthTTL {
		ttl = maxAuthTTL
	}
	token, err := r.engine.Authenticate(request.Context(), mts.Credentials{
		UserName: req.UserName,
		Password: req.Password,
	}, ttl)
	if err != nil {
		writeAPIError(writer, newAPIError(errorCodeUnauthenticated, "invalid credentials", err))
		return
	}
	mustChange := false
	if user, ok, getErr := r.engine.GetUser(request.Context(), req.UserName); getErr == nil && ok {
		mustChange = userMustChangePassword(user)
	}
	r.audit.record(auditEvent{UserName: req.UserName, Action: "login"})
	writeHTTPJSON(writer, http.StatusOK, authTokenResponse{Token: token, MustChangePassword: mustChange})
}

func (r *serverRuntime) handleLogout(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodPost) {
		return
	}
	token := bearerToken(request.Header.Get(headerAuthorization))
	if token == "" {
		var req logoutRequest
		if err := decodeHTTPJSON(request, &req); err != nil {
			writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
			return
		}
		token = req.Token
	}
	if strings.TrimSpace(token) == "" {
		writeAPIError(writer, newAPIError(errorCodeUnauthenticated, "user bearer token is required", nil))
		return
	}
	principal, err := r.engine.VerifyToken(request.Context(), token)
	if err != nil {
		writeAPIError(writer, newAPIError(errorCodeUnauthenticated, "invalid user bearer token", err))
		return
	}
	if err := r.engine.RevokeToken(request.Context(), token); err != nil {
		writeAPIError(writer, newAPIError(errorCodeUnauthenticated, "failed to revoke token", err))
		return
	}
	r.audit.record(auditEvent{UserName: principal.UserName, Action: "logout"})
	writeHTTPJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (r *serverRuntime) handleChangePassword(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodPost) {
		return
	}
	var req changePasswordRequest
	if err := decodeHTTPJSON(request, &req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	token := bearerToken(request.Header.Get(headerAuthorization))
	if token == "" {
		writeAPIError(writer, newAPIError(errorCodeUnauthenticated, "user bearer token is required", nil))
		return
	}
	principal, err := r.engine.VerifyToken(request.Context(), token)
	if err != nil {
		writeAPIError(writer, newAPIError(errorCodeUnauthenticated, "invalid user bearer token", err))
		return
	}
	if strings.TrimSpace(req.UserName) != principal.UserName {
		writeAPIError(writer, mts.ErrPermissionDenied)
		return
	}
	if err := r.engine.ChangePassword(
		request.Context(),
		req.UserName,
		req.OldPassword,
		req.NewPassword,
	); err != nil {
		writeAPIError(writer, newAPIError(errorCodeUnauthenticated, "invalid credentials", err))
		return
	}
	if err := r.clearMustChangePassword(request.Context(), principal.UserName); err != nil {
		writeAPIError(writer, err)
		return
	}
	r.audit.record(auditEvent{UserName: principal.UserName, Action: "change_password"})
	writeHTTPJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (r *serverRuntime) handleSession(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodGet) {
		return
	}
	token := bearerToken(request.Header.Get(headerAuthorization))
	if strings.TrimSpace(token) == "" {
		writeAPIError(writer, newAPIError(errorCodeUnauthenticated, "user bearer token is required", nil))
		return
	}
	principal, err := r.engine.VerifyToken(request.Context(), token)
	if err != nil {
		writeAPIError(writer, newAPIError(errorCodeUnauthenticated, "invalid user bearer token", err))
		return
	}
	mustChange := false
	if user, ok, getErr := r.engine.GetUser(request.Context(), principal.UserName); getErr == nil && ok {
		mustChange = userMustChangePassword(user)
	}
	remaining := int64(0)
	if !principal.ExpiresAt.IsZero() {
		sec := int64(time.Until(principal.ExpiresAt).Seconds())
		if sec > 0 {
			remaining = sec
		}
	}
	writeHTTPJSON(writer, http.StatusOK, sessionResponse{
		OK:                 true,
		UserName:           principal.UserName,
		Role:               principal.Role,
		ExpiresAt:          principal.ExpiresAt,
		MustChangePassword: mustChange,
		RemainingSeconds:   remaining,
	})
}
