package main

import "google.golang.org/grpc/codes"

func apiSpec() apiSpecResponse {
	return apiSpecFromRegistry()
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
