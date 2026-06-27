package main

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"google.golang.org/grpc/metadata"

	mts "github.com/openmts/mts"
)

const (
	headerAdminToken    = "X-MTS-Admin-Token"
	headerDataToken     = "X-MTS-Data-Token"
	headerAuthorization = "Authorization"
	headerUser          = "X-MTS-User"
	metadataAdminToken  = "x-mts-admin-token"
	metadataDataToken   = "x-mts-data-token"
)

func (r *serverRuntime) requireHTTPAdmin(request *http.Request) error {
	cfg := r.currentConfig()
	token := strings.TrimSpace(cfg.Auth.AdminToken)
	provided := request.Header.Get(headerAdminToken)
	bearer := bearerToken(request.Header.Get(headerAuthorization))
	if token == "" && bearer != "" {
		return r.requireHTTPAdminUser(request.Context(), bearer)
	}
	if token == "" && cfg.Auth.RequireUser {
		return newAPIError(errorCodeUnauthenticated, "admin user bearer token is required", nil)
	}
	if token == "" {
		return nil
	}
	if provided == "" {
		provided = bearer
	}
	if constantTimeEqual(token, provided) {
		return nil
	}
	if bearer != "" {
		return r.requireHTTPAdminUser(request.Context(), bearer)
	}
	return newAPIError(errorCodeUnauthenticated, "admin token is required", nil)
}

func (r *serverRuntime) requireGRPCAdmin(ctx context.Context) error {
	cfg := r.currentConfig()
	token := strings.TrimSpace(cfg.Auth.AdminToken)
	provided := grpcMetadataValue(ctx, metadataAdminToken)
	bearer := bearerToken(grpcMetadataValue(ctx, strings.ToLower(headerAuthorization)))
	if token == "" && bearer != "" {
		return r.requireGRPCAdminUser(ctx, bearer)
	}
	if token == "" && cfg.Auth.RequireUser {
		return newAPIError(errorCodeUnauthenticated, "admin user bearer token is required", nil)
	}
	if token == "" {
		return nil
	}
	if provided == "" {
		provided = bearer
	}
	if constantTimeEqual(token, provided) {
		return nil
	}
	if bearer != "" {
		return r.requireGRPCAdminUser(ctx, bearer)
	}
	return newAPIError(errorCodeUnauthenticated, "admin token is required", nil)
}

func (r *serverRuntime) requireHTTPAdminUser(ctx context.Context, token string) error {
	principal, err := r.engine.VerifyToken(ctx, token)
	if err != nil {
		return newAPIError(errorCodeUnauthenticated, "invalid admin bearer token", err)
	}
	return r.requireUserRole(ctx, principal.UserName, mts.UserRoleAdmin)
}

func (r *serverRuntime) requireGRPCAdminUser(ctx context.Context, token string) error {
	principal, err := r.engine.VerifyToken(ctx, token)
	if err != nil {
		return newAPIError(errorCodeUnauthenticated, "invalid admin bearer token", err)
	}
	return r.requireUserRole(ctx, principal.UserName, mts.UserRoleAdmin)
}

func (r *serverRuntime) requireUserRole(ctx context.Context, userName string, role mts.UserRole) error {
	user, ok, err := r.engine.GetUser(ctx, userName)
	if err != nil {
		return err
	}
	if !ok || user.Disabled || user.Role != role {
		return mts.ErrPermissionDenied
	}
	return nil
}

func (r *serverRuntime) authorizeHTTPDatabase(
	ctx context.Context,
	request *http.Request,
	database string,
	permission mts.DatabasePermission,
) error {
	if err := r.requireHTTPDataToken(request); err != nil {
		return err
	}
	userName, err := r.authenticateHTTPDataUser(ctx, request)
	if err != nil {
		return err
	}
	return r.authorizeDatabase(ctx, userName, database, permission)
}

func (r *serverRuntime) authorizeGRPCDatabase(
	ctx context.Context,
	database string,
	permission mts.DatabasePermission,
) error {
	if err := r.requireGRPCDataToken(ctx); err != nil {
		return err
	}
	userName, err := r.authenticateGRPCDataUser(ctx)
	if err != nil {
		return err
	}
	return r.authorizeDatabase(ctx, userName, database, permission)
}

func (r *serverRuntime) requireHTTPDataToken(request *http.Request) error {
	cfg := r.currentConfig()
	if len(cfg.Auth.DataTokens) == 0 {
		return nil
	}
	provided := request.Header.Get(headerDataToken)
	if provided == "" {
		provided = bearerToken(request.Header.Get(headerAuthorization))
	}
	if !matchesAnyToken(cfg.Auth.DataTokens, provided) {
		return newAPIError(errorCodeUnauthenticated, "data token is required", nil)
	}
	return nil
}

func (r *serverRuntime) requireGRPCDataToken(ctx context.Context) error {
	cfg := r.currentConfig()
	if len(cfg.Auth.DataTokens) == 0 {
		return nil
	}
	provided := grpcMetadataValue(ctx, metadataDataToken)
	if provided == "" {
		provided = bearerToken(grpcMetadataValue(ctx, strings.ToLower(headerAuthorization)))
	}
	if !matchesAnyToken(cfg.Auth.DataTokens, provided) {
		return newAPIError(errorCodeUnauthenticated, "data token is required", nil)
	}
	return nil
}

func (r *serverRuntime) authenticateHTTPDataUser(ctx context.Context, request *http.Request) (string, error) {
	cfg := r.currentConfig()
	token := bearerToken(request.Header.Get(headerAuthorization))
	if token == "" {
		if cfg.Auth.RequireUser {
			return "", newAPIError(errorCodeUnauthenticated, "user bearer token is required", nil)
		}
		return strings.TrimSpace(request.Header.Get(headerUser)), nil
	}
	principal, err := r.engine.VerifyToken(ctx, token)
	if err != nil {
		return "", newAPIError(errorCodeUnauthenticated, "invalid user bearer token", err)
	}
	return principal.UserName, nil
}

func (r *serverRuntime) authenticateGRPCDataUser(ctx context.Context) (string, error) {
	cfg := r.currentConfig()
	token := bearerToken(grpcMetadataValue(ctx, strings.ToLower(headerAuthorization)))
	if token == "" {
		if cfg.Auth.RequireUser {
			return "", newAPIError(errorCodeUnauthenticated, "user bearer token is required", nil)
		}
		return grpcMetadataValue(ctx, "x-mts-user"), nil
	}
	principal, err := r.engine.VerifyToken(ctx, token)
	if err != nil {
		return "", newAPIError(errorCodeUnauthenticated, "invalid user bearer token", err)
	}
	return principal.UserName, nil
}

func (r *serverRuntime) authorizeDatabase(
	ctx context.Context,
	userName string,
	database string,
	permission mts.DatabasePermission,
) error {
	userName = strings.TrimSpace(userName)
	if userName == "" {
		if r.currentConfig().Auth.RequireUser {
			return newAPIError(errorCodeUnauthenticated, "user identity is required", nil)
		}
		return nil
	}
	if strings.TrimSpace(database) == "" {
		database = r.config.Engine.DefaultDatabase
	}
	user, ok, err := r.engine.GetUser(ctx, userName)
	if err != nil {
		return err
	}
	if !ok {
		return mts.ErrPermissionDenied
	}
	if user.Disabled {
		return mts.ErrPermissionDenied
	}
	if user.Role == mts.UserRoleAdmin {
		return nil
	}
	if err := r.engine.CheckUserDatabasePermission(ctx, userName, database, permission); err != nil {
		return err
	}
	return nil
}

func matchesAnyToken(tokens []string, provided string) bool {
	for _, token := range tokens {
		if constantTimeEqual(token, provided) {
			return true
		}
	}
	return false
}

func bearerToken(value string) string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(value), "bearer ") {
		return ""
	}
	return strings.TrimSpace(value[len("bearer "):])
}

func constantTimeEqual(want string, got string) bool {
	want = strings.TrimSpace(want)
	got = strings.TrimSpace(got)
	if want == "" || got == "" || len(want) != len(got) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}

func grpcMetadataValue(ctx context.Context, key string) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (r *serverRuntime) auditUser(request *http.Request) string {
	userName, _ := r.authenticateHTTPDataUser(request.Context(), request)
	return userName
}
