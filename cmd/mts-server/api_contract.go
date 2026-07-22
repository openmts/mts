package main

import "google.golang.org/grpc/codes"

func apiSpec() apiSpecResponse {
	return apiSpecFromRegistry()
}

func errorCodeSpecs() errorCodesResponse {
	return errorCodesResponse{Codes: commercialErrorCodeSpecs()}
}

func commercialErrorCodeSpecs() []errorCodeSpec {
	return []errorCodeSpec{
		specClientBadRequest(),
		specAuthUnauthenticated(),
		specAuthzPermissionDenied(),
		specCapacityResourceExhausted(),
		specClientNotFound(),
		specClientAlreadyExists(),
		specServerInternal(),
	}
}

func specClientBadRequest() errorCodeSpec {
	return newErrorCodeSpec(
		errorCodeBadRequest,
		codes.InvalidArgument.String(),
		"request validation failed",
		false,
		"client",
		"Check request parameters, field types, and required fields; narrow invalid ranges",
		"/config?error_q=bad_request#config-error-codes",
	)
}

func specAuthUnauthenticated() errorCodeSpec {
	return newErrorCodeSpec(
		errorCodeUnauthenticated,
		codes.Unauthenticated.String(),
		"authentication required or invalid",
		false,
		"auth",
		"Sign in again or refresh the session; verify bearer token has not expired",
		"/account#account-session",
	)
}

func specAuthzPermissionDenied() errorCodeSpec {
	return newErrorCodeSpec(
		errorCodePermissionDenied,
		codes.PermissionDenied.String(),
		"permission denied",
		false,
		"authz",
		"Request database grant or switch to a user with the required permission",
		"/access-matrix",
	)
}

func specCapacityResourceExhausted() errorCodeSpec {
	return newErrorCodeSpec(
		errorCodeResourceExhausted,
		codes.ResourceExhausted.String(),
		"request exceeds configured limits",
		true,
		"capacity",
		"Narrow the query/write range, wait for admin heavy ops to finish, or raise limits",
		"/operations",
	)
}

func specClientNotFound() errorCodeSpec {
	return newErrorCodeSpec(
		errorCodeNotFound,
		codes.NotFound.String(),
		"resource not found",
		false,
		"client",
		"Verify database, measurement, policy, or user name exists before retrying",
		"/databases",
	)
}

func specClientAlreadyExists() errorCodeSpec {
	return newErrorCodeSpec(
		errorCodeAlreadyExists,
		codes.AlreadyExists.String(),
		"resource already exists",
		false,
		"client",
		"Use a unique name or update the existing resource instead of creating a duplicate",
		"/databases",
	)
}

func specServerInternal() errorCodeSpec {
	return newErrorCodeSpec(
		errorCodeInternal,
		codes.Internal.String(),
		"internal server error",
		true,
		"server",
		"Retry later; if it persists, check server logs and storage health",
		"/operations",
	)
}

func newErrorCodeSpec(
	code errorCode,
	grpcCode string,
	description string,
	retryable bool,
	category string,
	remediation string,
	dashboardPath string,
) errorCodeSpec {
	return errorCodeSpec{
		Code:          code,
		HTTPStatus:    httpStatusForErrorCode(code),
		GRPCCode:      grpcCode,
		Description:   description,
		Retryable:     retryable,
		Category:      category,
		Remediation:   remediation,
		DashboardPath: dashboardPath,
	}
}

func errorCodeMetaByCode(code errorCode) (errorCodeSpec, bool) {
	for _, spec := range commercialErrorCodeSpecs() {
		if spec.Code == code {
			return spec, true
		}
	}
	return errorCodeSpec{}, false
}
