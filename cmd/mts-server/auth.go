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
	headerAdminToken        = "X-MTS-Admin-Token"
	headerDataToken         = "X-MTS-Data-Token"
	headerAuthorization     = "Authorization"
	headerUser              = "X-MTS-User"
	headerAdminOpBusy       = "X-MTS-Admin-Op-Busy"
	headerAdminOp           = "X-MTS-Admin-Op"
	metadataAdminToken      = "x-mts-admin-token"
	metadataDataToken       = "x-mts-data-token"
	metadataUser            = "x-mts-user"
	metadataAdminOpBusy     = "x-mts-admin-op-busy"
	metadataAdminOp         = "x-mts-admin-op"
	credentialKeyAdminToken = "admin_token"
	credentialKeyDataToken  = "data_token"
	credentialKeyUser       = "user"
)

type credentialSource interface {
	Context() context.Context
	Value(key string) string
	Bearer() string
}

type httpCredentialSource struct {
	request *http.Request
}

func (s httpCredentialSource) Context() context.Context {
	return s.request.Context()
}

func (s httpCredentialSource) Value(key string) string {
	switch key {
	case credentialKeyAdminToken:
		return s.request.Header.Get(headerAdminToken)
	case credentialKeyDataToken:
		return s.request.Header.Get(headerDataToken)
	case credentialKeyUser:
		return strings.TrimSpace(s.request.Header.Get(headerUser))
	default:
		return ""
	}
}

func (s httpCredentialSource) Bearer() string {
	return bearerToken(s.request.Header.Get(headerAuthorization))
}

type grpcCredentialSource struct {
	ctx context.Context
}

func (s grpcCredentialSource) Context() context.Context {
	return s.ctx
}

func (s grpcCredentialSource) Value(key string) string {
	switch key {
	case credentialKeyAdminToken:
		return grpcMetadataValue(s.ctx, metadataAdminToken)
	case credentialKeyDataToken:
		return grpcMetadataValue(s.ctx, metadataDataToken)
	case credentialKeyUser:
		return strings.TrimSpace(grpcMetadataValue(s.ctx, metadataUser))
	default:
		return ""
	}
}

func (s grpcCredentialSource) Bearer() string {
	return bearerToken(grpcMetadataValue(s.ctx, strings.ToLower(headerAuthorization)))
}

func (r *serverRuntime) requireHTTPAdmin(request *http.Request) error {
	return r.requireAdmin(httpCredentialSource{request: request})
}

func (r *serverRuntime) requireGRPCAdmin(ctx context.Context) error {
	return r.requireAdmin(grpcCredentialSource{ctx: ctx})
}

func (r *serverRuntime) requireAdmin(source credentialSource) error {
	cfg := r.currentConfig()
	token := strings.TrimSpace(cfg.Auth.AdminToken)
	provided := source.Value(credentialKeyAdminToken)
	bearer := source.Bearer()
	if token == "" && bearer != "" {
		return r.requireAdminUser(source.Context(), bearer)
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
		return r.requireAdminUser(source.Context(), bearer)
	}
	return newAPIError(errorCodeUnauthenticated, "admin token is required", nil)
}

func (r *serverRuntime) requireAdminUser(ctx context.Context, token string) error {
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
	return r.authorizeSourceDatabase(httpCredentialSource{request: request}, database, permission)
}

func (r *serverRuntime) authorizeGRPCDatabase(
	ctx context.Context,
	database string,
	permission mts.DatabasePermission,
) error {
	return r.authorizeSourceDatabase(grpcCredentialSource{ctx: ctx}, database, permission)
}

func (r *serverRuntime) authorizeSourceDatabase(
	source credentialSource,
	database string,
	permission mts.DatabasePermission,
) error {
	if err := r.requireDataToken(source); err != nil {
		return err
	}
	userName, err := r.authenticateDataUser(source)
	if err != nil {
		return err
	}
	return r.authorizeDatabase(source.Context(), userName, database, permission)
}

func (r *serverRuntime) requireDataToken(source credentialSource) error {
	cfg := r.currentConfig()
	if len(cfg.Auth.DataTokens) == 0 {
		return nil
	}
	provided := source.Value(credentialKeyDataToken)
	if provided == "" {
		provided = source.Bearer()
	}
	if !matchesAnyToken(cfg.Auth.DataTokens, provided) {
		return newAPIError(errorCodeUnauthenticated, "data token is required", nil)
	}
	return nil
}

func (r *serverRuntime) authenticateHTTPDataUser(ctx context.Context, request *http.Request) (string, error) {
	return r.authenticateDataUser(httpCredentialSource{request: request})
}

func (r *serverRuntime) authenticateDataUser(source credentialSource) (string, error) {
	cfg := r.currentConfig()
	token := source.Bearer()
	if token == "" {
		if cfg.Auth.RequireUser {
			return "", newAPIError(errorCodeUnauthenticated, "user bearer token is required", nil)
		}
		return source.Value(credentialKeyUser), nil
	}
	principal, err := r.engine.VerifyToken(source.Context(), token)
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
	if !strings.HasPrefix(strings.ToLower(value), bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(value[len(bearerPrefix):])
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
