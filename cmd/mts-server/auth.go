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
	metadataUser        = "x-mts-user"
)

func (r *serverRuntime) requireHTTPAdmin(request *http.Request) error {
	token := strings.TrimSpace(r.config.Auth.AdminToken)
	if token == "" {
		return nil
	}
	provided := request.Header.Get(headerAdminToken)
	if provided == "" {
		provided = bearerToken(request.Header.Get(headerAuthorization))
	}
	if !constantTimeEqual(token, provided) {
		return newAPIError(errorCodeUnauthenticated, "admin token is required", nil)
	}
	return nil
}

func (r *serverRuntime) requireGRPCAdmin(ctx context.Context) error {
	token := strings.TrimSpace(r.config.Auth.AdminToken)
	if token == "" {
		return nil
	}
	provided := grpcMetadataValue(ctx, metadataAdminToken)
	if provided == "" {
		provided = bearerToken(grpcMetadataValue(ctx, strings.ToLower(headerAuthorization)))
	}
	if !constantTimeEqual(token, provided) {
		return newAPIError(errorCodeUnauthenticated, "admin token is required", nil)
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
	return r.authorizeDatabase(ctx, request.Header.Get(headerUser), database, permission)
}

func (r *serverRuntime) authorizeGRPCDatabase(
	ctx context.Context,
	database string,
	permission mts.DatabasePermission,
) error {
	if err := r.requireGRPCDataToken(ctx); err != nil {
		return err
	}
	return r.authorizeDatabase(ctx, grpcMetadataValue(ctx, metadataUser), database, permission)
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
