package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	mts "github.com/openmts/mts"
)

func (r *serverRuntime) handleUsers(writer http.ResponseWriter, request *http.Request) {
	if err := r.requireHTTPAdmin(request); err != nil {
		writeAPIError(writer, err)
		return
	}
	switch request.Method {
	case http.MethodPost:
		var req createUserRequest
		if err := decodeHTTPJSON(request, &req); err != nil {
			writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
			return
		}
		if err := r.createUserWithInitialPassword(request.Context(), req); err != nil {
			writeAPIError(writer, err)
			return
		}
		r.audit.record(auditEvent{UserName: req.Name, Action: "create_user"})
		writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToOK(okResponse{OK: true}))
	case http.MethodGet:
		users, err := r.engine.ListUsers(request.Context())
		if err != nil {
			writeAPIError(writer, err)
			return
		}
		writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToUsers(usersResponse{Users: users}))
	default:
		writeAPIError(writer, newAPIError(errorCodeBadRequest, messageMethodNotAllowed, nil))
	}
}

func (r *serverRuntime) createUserWithInitialPassword(ctx context.Context, req createUserRequest) error {
	if err := r.engine.CreateUser(ctx, req.User); err != nil {
		return err
	}
	if req.Password == "" {
		return nil
	}
	if err := validateUserPassword(req.Password); err != nil {
		rollbackErr := r.engine.DeleteUser(ctx, req.Name)
		if rollbackErr != nil {
			return rollbackErr
		}
		return err
	}
	if err := r.engine.SetPassword(ctx, req.Name, req.Password); err != nil {
		rollbackErr := r.engine.DeleteUser(ctx, req.Name)
		if rollbackErr != nil {
			return rollbackErr
		}
		return err
	}
	return nil
}

func (r *serverRuntime) handleUserResource(writer http.ResponseWriter, request *http.Request) {
	parts := splitPath(request.URL.Path, routeUsersPrefix)
	if len(parts) == 0 || parts[0] == "" {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "user name is required", nil))
		return
	}
	userName := parts[0]
	// 自身审计：登录用户可读自己的 audit；其它用户资源仍需 admin。
	if len(parts) == 2 && parts[1] == "audit" {
		r.handleUserAudit(writer, request, userName)
		return
	}
	if err := r.requireHTTPAdmin(request); err != nil {
		writeAPIError(writer, err)
		return
	}
	if len(parts) == 1 {
		r.handleSingleUser(writer, request, userName)
		return
	}
	if len(parts) >= 2 && parts[1] == "database-permissions" {
		r.handleDatabasePermissionResource(writer, request, userName, parts[2:])
		return
	}
	if len(parts) == 2 && parts[1] == "password" {
		r.handleUserPassword(writer, request, userName)
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
		r.audit.record(auditEvent{UserName: user.Name, Action: "update_user"})
		writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToOK(okResponse{OK: true}))
	case http.MethodDelete:
		if err := r.engine.DeleteUser(request.Context(), userName); err != nil {
			writeAPIError(writer, err)
			return
		}
		r.audit.record(auditEvent{UserName: userName, Action: "delete_user"})
		writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToOK(okResponse{OK: true}))
	default:
		writeAPIError(writer, newAPIError(errorCodeBadRequest, messageMethodNotAllowed, nil))
	}
}

func (r *serverRuntime) handleUserPassword(writer http.ResponseWriter, request *http.Request, userName string) {
	if !requireHTTPMethod(writer, request, http.MethodPut) {
		return
	}
	var req passwordRequest
	if err := decodeHTTPJSON(request, &req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	if err := validateUserPassword(req.Password); err != nil {
		writeAPIError(writer, err)
		return
	}
	if err := r.engine.SetPassword(request.Context(), userName, req.Password); err != nil {
		writeAPIError(writer, err)
		return
	}
	if err := r.clearMustChangePassword(request.Context(), userName); err != nil {
		writeAPIError(writer, err)
		return
	}
	r.audit.record(auditEvent{UserName: userName, Action: "set_password"})
	writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToSetPassword(setPasswordResponse{
		OK:       true,
		UserName: userName,
	}))
}

func (r *serverRuntime) handleDatabasePermissionResource(
	writer http.ResponseWriter,
	request *http.Request,
	userName string,
	parts []string,
) {
	if len(parts) == 0 {
		if request.Method != http.MethodGet {
			writeAPIError(writer, newAPIError(errorCodeBadRequest, messageMethodNotAllowed, nil))
			return
		}
		grants, err := r.engine.ListDatabasePermissions(request.Context(), userName)
		if err != nil {
			writeAPIError(writer, err)
			return
		}
		writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToPermissions(databasePermissionsResponse{Grants: grants}))
		return
	}
	if len(parts) != 2 {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "database and permission are required", nil))
		return
	}
	database := parts[0]
	permission := mts.DatabasePermission(parts[1])
	if permission != mts.DatabasePermissionRead && permission != mts.DatabasePermissionWrite && permission != mts.DatabasePermissionAdmin {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "invalid permission: must be read, write, or admin", nil))
		return
	}
	switch request.Method {
	case http.MethodPut, http.MethodPost:
		if err := r.engine.GrantDatabasePermission(request.Context(), userName, database, permission); err != nil {
			writeAPIError(writer, err)
			return
		}
		r.audit.record(auditEvent{UserName: userName, Action: "grant_database_permission", Database: database, Detail: string(permission)})
	case http.MethodDelete:
		if err := r.engine.RevokeDatabasePermission(request.Context(), userName, database, permission); err != nil {
			writeAPIError(writer, err)
			return
		}
		r.audit.record(auditEvent{UserName: userName, Action: "revoke_database_permission", Database: database, Detail: string(permission)})
	default:
		writeAPIError(writer, newAPIError(errorCodeBadRequest, messageMethodNotAllowed, nil))
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToOK(okResponse{OK: true}))
}

func (r *serverRuntime) handleUserAudit(writer http.ResponseWriter, request *http.Request, userName string) {
	if !requireHTTPMethod(writer, request, http.MethodGet) {
		return
	}
	if err := r.requireHTTPAdmin(request); err != nil {
		self, authErr := r.authenticateHTTPDataUser(request.Context(), request)
		if authErr != nil || strings.TrimSpace(self) == "" {
			writeAPIError(writer, err)
			return
		}
		if strings.TrimSpace(userName) != strings.TrimSpace(self) {
			writeAPIError(writer, mts.ErrPermissionDenied)
			return
		}
	}
	req := auditListRequest{UserName: strings.TrimSpace(userName)}
	q := request.URL.Query()
	if v := strings.TrimSpace(q.Get("action")); v != "" {
		req.Action = v
	}
	if v := q.Get("since_unix"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			req.SinceUnix = n
		}
	}
	if v := q.Get("until_unix"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			req.UntilUnix = n
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			req.Limit = n
		}
	}
	events := r.audit.listFiltered(req)
	writeHTTPJSON(writer, http.StatusOK, userAuditResponse{Events: events})
}

func (r *serverRuntime) handleAuthzDatabaseCheck(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodPost) {
		return
	}
	var req authzDatabaseCheckRequest
	if err := decodeHTTPJSON(request, &req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	// admin 可检查任意用户；普通登录用户仅可自检。
	if err := r.requireHTTPAdmin(request); err != nil {
		self, authErr := r.authenticateHTTPDataUser(request.Context(), request)
		if authErr != nil || strings.TrimSpace(self) == "" {
			writeAPIError(writer, err)
			return
		}
		req.UserName = strings.TrimSpace(req.UserName)
		if req.UserName == "" {
			req.UserName = self
		} else if req.UserName != self {
			writeAPIError(writer, mts.ErrPermissionDenied)
			return
		}
	}
	if strings.TrimSpace(req.UserName) == "" {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "user_name is required", nil))
		return
	}
	err := r.engine.CheckUserDatabasePermission(
		request.Context(),
		req.UserName,
		req.Database,
		req.Permission,
	)
	if err != nil {
		if errors.Is(err, mts.ErrPermissionDenied) {
			writeHTTPJSON(writer, http.StatusOK, authzDatabaseCheckResponse{Allowed: false})
			return
		}
		writeAPIError(writer, err)
		return
	}
	writeHTTPJSON(writer, http.StatusOK, authzDatabaseCheckResponse{Allowed: true})
}
