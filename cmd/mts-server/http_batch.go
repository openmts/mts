package main

import (
	"context"
	"errors"
	"net/http"
	"strings"

	mts "github.com/openmts/mts"
)

const (
	batchStatusOK    = "ok"
	batchStatusSkip  = "skip"
	batchStatusError = "error"
	batchMaxItems    = 200
)

func (r *serverRuntime) handleBatchUserDisabled(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodPost) {
		return
	}
	if err := r.requireHTTPAdmin(request); err != nil {
		writeAPIError(writer, err)
		return
	}
	var req batchUserDisabledRequest
	if err := decodeHTTPJSON(request, &req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	if wantsBatchProgressStream(request) {
		r.streamBatchUserDisabled(writer, request, req)
		return
	}
	resp, err := r.batchUpdateUserDisabled(request.Context(), req, r.auditUser(request))
	if err != nil {
		writeAPIError(writer, err)
		return
	}
	r.recordBatchUserDisabledLast(req.Disabled, resp)
	writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToBatch(resp))
}

func (r *serverRuntime) batchUpdateUserDisabled(
	ctx context.Context,
	req batchUserDisabledRequest,
	actor string,
) (batchMutationResponse, error) {
	names := normalizeBatchNames(req.Names)
	if err := validateBatchNames(names); err != nil {
		return batchMutationResponse{}, err
	}
	out := batchMutationResponse{OK: true, Items: make([]batchItemResult, 0, len(names))}
	for _, name := range names {
		item := r.applyUserDisabled(ctx, name, req.Disabled, actor)
		out.Items = append(out.Items, item)
		switch item.Status {
		case batchStatusOK:
			out.OKCount++
		case batchStatusSkip:
			out.Skip++
		default:
			out.Fail++
			out.OK = false
		}
	}
	return out, nil
}

func (r *serverRuntime) applyUserDisabled(
	ctx context.Context,
	name string,
	disabled bool,
	actor string,
) batchItemResult {
	user, ok, err := r.engine.GetUser(ctx, name)
	if err != nil {
		return batchItemResult{Name: name, Status: batchStatusError, Message: err.Error()}
	}
	if !ok {
		return batchItemResult{Name: name, Status: batchStatusSkip, Message: "not found"}
	}
	if disabled && strings.TrimSpace(name) == strings.TrimSpace(actor) {
		return batchItemResult{Name: name, Status: batchStatusSkip, Message: "cannot disable self"}
	}
	if user.Disabled == disabled {
		return batchItemResult{Name: name, Status: batchStatusSkip, Message: "already in desired state"}
	}
	user.Disabled = disabled
	if err := r.engine.UpdateUser(ctx, user); err != nil {
		return batchItemResult{Name: name, Status: batchStatusError, Message: err.Error()}
	}
	r.audit.record(auditEvent{UserName: actor, Action: "batch_update_user_disabled", Detail: name})
	return batchItemResult{Name: name, Status: batchStatusOK}
}

func (r *serverRuntime) handleBatchDownsamplePolicies(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodPost) {
		return
	}
	if err := r.requireHTTPAdmin(request); err != nil {
		writeAPIError(writer, err)
		return
	}
	var req batchDownsampleRequest
	if err := decodeHTTPJSON(request, &req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	if wantsBatchProgressStream(request) {
		r.streamBatchDownsamplePolicies(writer, request, req)
		return
	}
	resp, err := r.batchDownsamplePolicies(request.Context(), req, r.auditUser(request))
	if err != nil {
		writeAPIError(writer, err)
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToBatch(resp))
}

func (r *serverRuntime) batchDownsamplePolicies(
	ctx context.Context,
	req batchDownsampleRequest,
	actor string,
) (batchMutationResponse, error) {
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action != "enable" && action != "disable" {
		return batchMutationResponse{}, newAPIError(errorCodeBadRequest, "action must be enable or disable", nil)
	}
	names := normalizeBatchNames(req.Names)
	if err := validateBatchNames(names); err != nil {
		return batchMutationResponse{}, err
	}
	out := batchMutationResponse{OK: true, Items: make([]batchItemResult, 0, len(names))}
	for _, name := range names {
		item := r.applyDownsampleAction(ctx, name, action, actor)
		out.Items = append(out.Items, item)
		switch item.Status {
		case batchStatusOK:
			out.OKCount++
		case batchStatusSkip:
			out.Skip++
		default:
			out.Fail++
			out.OK = false
		}
	}
	return out, nil
}

func (r *serverRuntime) applyDownsampleAction(
	ctx context.Context,
	name string,
	action string,
	actor string,
) batchItemResult {
	var err error
	if action == "enable" {
		err = r.engine.EnableDownsamplePolicy(ctx, name)
	} else {
		err = r.engine.DisableDownsamplePolicy(ctx, name)
	}
	if err != nil {
		if errors.Is(err, mts.ErrNotFound) {
			return batchItemResult{Name: name, Status: batchStatusSkip, Message: err.Error()}
		}
		return batchItemResult{Name: name, Status: batchStatusError, Message: err.Error()}
	}
	r.audit.record(auditEvent{
		UserName: actor,
		Action:   "batch_" + action + "_downsample_policy",
		Detail:   name,
	})
	return batchItemResult{Name: name, Status: batchStatusOK}
}

func normalizeBatchNames(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	out := make([]string, 0, len(names))
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func grpcBatchUpdateUserDisabled(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := r.batchUpdateUserDisabled(ctx, *req.(*batchUserDisabledRequest), r.grpcActor(ctx))
	if err != nil {
		return nil, err
	}
	return r.attachAdminOpToBatch(resp), nil
}

func grpcBatchDownsamplePolicies(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := r.batchDownsamplePolicies(ctx, *req.(*batchDownsampleRequest), r.grpcActor(ctx))
	if err != nil {
		return nil, err
	}
	return r.attachAdminOpToBatch(resp), nil
}

func (r *serverRuntime) grpcActor(ctx context.Context) string {
	source := grpcCredentialSource{ctx: ctx}
	token := source.Bearer()
	if token == "" {
		return strings.TrimSpace(source.Value(credentialKeyUser))
	}
	principal, err := r.engine.VerifyToken(ctx, token)
	if err != nil {
		return ""
	}
	return principal.UserName
}

func (r *serverRuntime) recordBatchUserDisabledLast(disabled bool, resp batchMutationResponse) {
	op := "batch_user_enable"
	if disabled {
		op = "batch_user_disable"
	}
	ok := resp.Fail == 0
	errMsg := ""
	if !ok {
		errMsg = "batch user disabled partial failure"
	}
	r.recordAdminOpLast(op, ok, errMsg)
}
