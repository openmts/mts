package main

import "google.golang.org/grpc/codes"

func apiSpec() apiSpecResponse {
	return apiSpecResponse{Version: "v1", Namespaces: []apiNamespace{
		{Name: "data", BasePath: "/api/v1/data", Endpoints: []apiEndpoint{
			{Method: "POST", Path: "/api/v1/data/write", Auth: "data token optional; when require_user is true, user bearer token and DB write permission are required", Description: "write point batch"},
			{Method: "POST", Path: "/api/v1/data/write/typed", Auth: "data token optional; when require_user is true, user bearer token and DB write permission are required", Description: "write typed column batch"},
			{Method: "POST", Path: "/api/v1/data/query/rows", Auth: "data token optional; when require_user is true, user bearer token and DB read permission are required", Description: "query row result"},
			{Method: "POST", Path: "/api/v1/data/query/columns", Auth: "data token optional; when require_user is true, user bearer token and DB read permission are required", Description: "query column result"},
			{Method: "POST", Path: "/api/v1/data/query/explain", Auth: "data token optional; when require_user is true, user bearer token and DB read permission are required", Description: "query with execution explain"},
			{Method: "POST", Path: "/api/v1/data/query/stream", Auth: "data token optional; when require_user is true, user bearer token and DB read permission are required", Description: "query NDJSON stream"},
		}},
		{Name: "auth", BasePath: "/api/v1/auth", Endpoints: []apiEndpoint{
			{Method: "POST", Path: "/api/v1/auth/login", Auth: "user password", Description: "issue user bearer token"},
			{Method: "POST", Path: "/api/v1/auth/logout", Auth: "user bearer token", Description: "revoke user bearer token"},
			{Method: "POST", Path: "/api/v1/auth/password", Auth: "user password", Description: "change user password"},
		}},
		{Name: "admin", BasePath: "/api/v1/admin", Endpoints: []apiEndpoint{
			{Method: "GET", Path: "/api/v1/admin/config", Auth: "admin token", Description: "read effective config"},
			{Method: "POST", Path: "/api/v1/admin/config/validate", Auth: "admin token", Description: "validate config payload"},
			{Method: "POST", Path: "/api/v1/admin/config/reload", Auth: "admin token", Description: "reload hot fields"},
			{Method: "GET|POST", Path: "/api/v1/admin/databases", Auth: "admin token", Description: "list or create databases"},
			{Method: "GET", Path: "/api/v1/admin/api-spec", Auth: "admin token", Description: "read API contract"},
			{Method: "GET", Path: "/api/v1/admin/error-codes", Auth: "admin token", Description: "read error code contract"},
			{Method: "POST", Path: "/api/v1/admin/storage/validate", Auth: "admin token", Description: "validate local storage state"},
			{Method: "POST", Path: "/api/v1/admin/storage/snapshot", Auth: "admin token", Description: "write local manifest snapshot"},
			{Method: "GET", Path: "/api/v1/admin/storage/export", Auth: "admin token", Description: "export server metadata summary"},
		}},
		{Name: "users", BasePath: "/api/v1/users", Endpoints: []apiEndpoint{
			{Method: "POST", Path: "/api/v1/users", Auth: "admin token or admin user bearer token", Description: "create user, optional initial password"},
			{Method: "PUT", Path: "/api/v1/users/{name}/password", Auth: "admin token or admin user bearer token", Description: "set user password"},
			{Method: "GET", Path: "/api/v1/users/{name}/audit", Auth: "admin token or admin user bearer token", Description: "read user audit events"},
			{Method: "POST", Path: "/api/v1/users/{name}/database-permissions/{database}/{permission}", Auth: "admin token or admin user bearer token", Description: "grant DB permission"},
		}},
	}}
}

func errorCodeSpecs() errorCodesResponse {
	return errorCodesResponse{Codes: []errorCodeSpec{
		{Code: errorCodeBadRequest, HTTPStatus: httpStatusForErrorCode(errorCodeBadRequest), GRPCCode: codes.InvalidArgument.String(), Description: "request validation failed"},
		{Code: errorCodeUnauthenticated, HTTPStatus: httpStatusForErrorCode(errorCodeUnauthenticated), GRPCCode: codes.Unauthenticated.String(), Description: "authentication required or invalid"},
		{Code: errorCodePermissionDenied, HTTPStatus: httpStatusForErrorCode(errorCodePermissionDenied), GRPCCode: codes.PermissionDenied.String(), Description: "permission denied"},
		{Code: errorCodeResourceExhausted, HTTPStatus: httpStatusForErrorCode(errorCodeResourceExhausted), GRPCCode: codes.ResourceExhausted.String(), Description: "request exceeds configured limits"},
		{Code: errorCodeNotFound, HTTPStatus: httpStatusForErrorCode(errorCodeNotFound), GRPCCode: codes.NotFound.String(), Description: "resource not found"},
		{Code: errorCodeAlreadyExists, HTTPStatus: httpStatusForErrorCode(errorCodeAlreadyExists), GRPCCode: codes.AlreadyExists.String(), Description: "resource already exists"},
		{Code: errorCodeInternal, HTTPStatus: httpStatusForErrorCode(errorCodeInternal), GRPCCode: codes.Internal.String(), Description: "internal server error"},
	}}
}
