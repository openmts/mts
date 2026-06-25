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
	headerAuthorization = "Authorization"
	headerUser          = "X-MTS-User"
	metadataAdminToken  = "x-mts-admin-token"
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
	return r.authorizeDatabase(ctx, request.Header.Get(headerUser), database, permission)
}

func (r *serverRuntime) authorizeGRPCDatabase(
	ctx context.Context,
	database string,
	permission mts.DatabasePermission,
) error {
	return r.authorizeDatabase(ctx, grpcMetadataValue(ctx, metadataUser), database, permission)
}

func (r *serverRuntime) authorizeDatabase(
	ctx context.Context,
	userName string,
	database string,
	permission mts.DatabasePermission,
) error {
	userName = strings.TrimSpace(userName)
	if userName == "" {
		return nil
	}
	if strings.TrimSpace(database) == "" {
		database = r.config.Engine.DefaultDatabase
	}
	if err := r.engine.CheckUserDatabasePermission(ctx, userName, database, permission); err != nil {
		return err
	}
	return nil
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
