package main

import (
	"net/http"
	"strconv"
	"strings"

	mts "github.com/openmts/mts"
)

const defaultAccessGrantsPageLimit = 100

type accessGrantsPageRequest struct {
	Cursor string
	Limit  int
}

func (r *serverRuntime) handleAccessGrants(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	pageRequest, err := parseAccessGrantsPageRequest(request)
	if err != nil {
		writeAPIError(writer, err)
		return
	}
	page, err := r.engine.ListUserGrantPage(request.Context(), pageRequest.Cursor, pageRequest.Limit)
	if err != nil {
		writeAPIError(writer, err)
		return
	}
	response := accessGrantsResponse{
		Items:      page.Items,
		TotalUsers: page.TotalUsers,
		NextCursor: page.NextCursor,
		Path:       request.URL.Path,
	}
	writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToAccessGrants(response))
}

func parseAccessGrantsPageRequest(request *http.Request) (accessGrantsPageRequest, error) {
	query := request.URL.Query()
	pageRequest := accessGrantsPageRequest{
		Cursor: query.Get("cursor"),
		Limit:  defaultAccessGrantsPageLimit,
	}
	if pageRequest.Cursor != strings.TrimSpace(pageRequest.Cursor) {
		return accessGrantsPageRequest{}, newAPIError(
			errorCodeBadRequest,
			"cursor must not contain surrounding whitespace",
			nil,
		)
	}
	if rawLimit := query.Get("limit"); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil {
			return accessGrantsPageRequest{}, newAPIError(
				errorCodeBadRequest,
				"limit must be an integer between 1 and 200",
				err,
			)
		}
		pageRequest.Limit = limit
	}
	if pageRequest.Limit < 1 || pageRequest.Limit > mts.MaxUserGrantPageLimit {
		return accessGrantsPageRequest{}, newAPIError(
			errorCodeBadRequest,
			"limit must be between 1 and 200",
			mts.ErrInvalidPageLimit,
		)
	}
	return pageRequest, nil
}

func (r *serverRuntime) attachAdminOpToAccessGrants(response accessGrantsResponse) accessGrantsResponse {
	busy, operation, started := r.adminHeavyState()
	response.AdminOpBusy = busy
	response.Op = operation
	response.StartedAtUnix = started
	response.Last = r.lastAdminHeavySnapshot()
	return response
}
