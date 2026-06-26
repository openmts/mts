package main

import (
	"net/http"
	"strings"
	"time"

	mts "github.com/openmts/mts"
)

const defaultAuthTTL = 12 * time.Hour

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
	token, err := r.engine.Authenticate(request.Context(), mts.Credentials{
		UserName: req.UserName,
		Password: req.Password,
	}, ttl)
	if err != nil {
		writeAPIError(writer, newAPIError(errorCodeUnauthenticated, "invalid credentials", err))
		return
	}
	writeHTTPJSON(writer, http.StatusOK, authTokenResponse{Token: token})
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
	if err := r.engine.RevokeToken(request.Context(), token); err != nil {
		writeAPIError(writer, newAPIError(errorCodeUnauthenticated, "invalid user bearer token", err))
		return
	}
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
	if err := r.engine.ChangePassword(
		request.Context(),
		req.UserName,
		req.OldPassword,
		req.NewPassword,
	); err != nil {
		writeAPIError(writer, newAPIError(errorCodeUnauthenticated, "invalid credentials", err))
		return
	}
	writeHTTPJSON(writer, http.StatusOK, okResponse{OK: true})
}
