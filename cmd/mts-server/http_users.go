package main

import (
	"net/http"

	mts "github.com/openmts/mts"
)

func (r *serverRuntime) handleUsers(writer http.ResponseWriter, request *http.Request) {
	if err := r.requireHTTPAdmin(request); err != nil {
		writeAPIError(writer, err)
		return
	}
	switch request.Method {
	case http.MethodPost:
		var user mts.User
		if err := decodeHTTPJSON(request, &user); err != nil {
			writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
			return
		}
		if err := r.engine.CreateUser(request.Context(), user); err != nil {
			writeAPIError(writer, err)
			return
		}
		writeHTTPJSON(writer, http.StatusOK, okResponse{OK: true})
	case http.MethodGet:
		users, err := r.engine.ListUsers(request.Context())
		if err != nil {
			writeAPIError(writer, err)
			return
		}
		writeHTTPJSON(writer, http.StatusOK, usersResponse{Users: users})
	default:
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "method not allowed", nil))
	}
}

func (r *serverRuntime) handleUserResource(writer http.ResponseWriter, request *http.Request) {
	if err := r.requireHTTPAdmin(request); err != nil {
		writeAPIError(writer, err)
		return
	}
	parts := splitPath(request.URL.Path, "/api/v1/users/")
	if len(parts) == 0 || parts[0] == "" {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "user name is required", nil))
		return
	}
	userName := parts[0]
	if len(parts) == 1 {
		r.handleSingleUser(writer, request, userName)
		return
	}
	if len(parts) >= 2 && parts[1] == "database-permissions" {
		r.handleDatabasePermissionResource(writer, request, userName, parts[2:])
		return
	}
	writeAPIError(writer, newAPIError(errorCodeNotFound, "user resource not found", nil))
}

func (r *serverRuntime) handleSingleUser(
	writer http.ResponseWriter,
	request *http.Request,
	userName string,
) {
	switch request.Method {
	case http.MethodGet:
		user, ok, err := r.engine.GetUser(request.Context(), userName)
		if err != nil {
			writeAPIError(writer, err)
			return
		}
		if !ok {
			writeAPIError(writer, newAPIError(errorCodeNotFound, "user not found", mts.ErrUserNotFound))
			return
		}
		writeHTTPJSON(writer, http.StatusOK, userResponse{User: user})
	case http.MethodPut:
		var user mts.User
		if err := decodeHTTPJSON(request, &user); err != nil {
			writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
			return
		}
		user.Name = userName
		if err := r.engine.UpdateUser(request.Context(), user); err != nil {
			writeAPIError(writer, err)
			return
		}
		writeHTTPJSON(writer, http.StatusOK, okResponse{OK: true})
	case http.MethodDelete:
		if err := r.engine.DeleteUser(request.Context(), userName); err != nil {
			writeAPIError(writer, err)
			return
		}
		writeHTTPJSON(writer, http.StatusOK, okResponse{OK: true})
	default:
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "method not allowed", nil))
	}
}

func (r *serverRuntime) handleDatabasePermissionResource(
	writer http.ResponseWriter,
	request *http.Request,
	userName string,
	parts []string,
) {
	if len(parts) == 0 {
		if request.Method != http.MethodGet {
			writeAPIError(writer, newAPIError(errorCodeBadRequest, "method not allowed", nil))
			return
		}
		grants, err := r.engine.ListDatabasePermissions(request.Context(), userName)
		if err != nil {
			writeAPIError(writer, err)
			return
		}
		writeHTTPJSON(writer, http.StatusOK, databasePermissionsResponse{Grants: grants})
		return
	}
	if len(parts) != 2 {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "database and permission are required", nil))
		return
	}
	database := parts[0]
	permission := mts.DatabasePermission(parts[1])
	switch request.Method {
	case http.MethodPut, http.MethodPost:
		if err := r.engine.GrantDatabasePermission(request.Context(), userName, database, permission); err != nil {
			writeAPIError(writer, err)
			return
		}
	case http.MethodDelete:
		if err := r.engine.RevokeDatabasePermission(request.Context(), userName, database, permission); err != nil {
			writeAPIError(writer, err)
			return
		}
	default:
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "method not allowed", nil))
		return
	}
	writeHTTPJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (r *serverRuntime) handleAuthzDatabaseCheck(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodPost) {
		return
	}
	if err := r.requireHTTPAdmin(request); err != nil {
		writeAPIError(writer, err)
		return
	}
	var req authzDatabaseCheckRequest
	if err := decodeHTTPJSON(request, &req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	err := r.engine.CheckUserDatabasePermission(
		request.Context(),
		req.UserName,
		req.Database,
		req.Permission,
	)
	if err != nil {
		if err == mts.ErrPermissionDenied {
			writeHTTPJSON(writer, http.StatusOK, authzDatabaseCheckResponse{Allowed: false})
			return
		}
		writeAPIError(writer, err)
		return
	}
	writeHTTPJSON(writer, http.StatusOK, authzDatabaseCheckResponse{Allowed: true})
}
