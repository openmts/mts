package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

func (r *serverRuntime) streamBatchUserDisabled(
	writer http.ResponseWriter,
	request *http.Request,
	req batchUserDisabledRequest,
) {
	names := normalizeBatchNames(req.Names)
	if err := validateBatchNames(names); err != nil {
		writeAPIError(writer, err)
		return
	}
	actor := r.auditUser(request)
	enc, flusher, ok := beginBatchProgressStream(writer)
	if !ok {
		return
	}
	streamBatchItems(request.Context(), enc, flusher, names, func(ctx context.Context, name string) batchItemResult {
		return r.applyUserDisabled(ctx, name, req.Disabled, actor)
	}, func(event *batchProgressEvent) {
		if event != nil && event.Type == "summary" && !event.Cancelled {
			r.recordBatchUserDisabledLast(req.Disabled, batchMutationResponse{
				OK:      event.OK,
				OKCount: event.OKCount,
				Skip:    event.Skip,
				Fail:    event.Fail,
				Items:   event.Items,
			})
		}
		r.attachBatchProgressSummary(event)
	})
}

func (r *serverRuntime) streamBatchDownsamplePolicies(
	writer http.ResponseWriter,
	request *http.Request,
	req batchDownsampleRequest,
) {
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action != "enable" && action != "disable" {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "action must be enable or disable", nil))
		return
	}
	names := normalizeBatchNames(req.Names)
	if err := validateBatchNames(names); err != nil {
		writeAPIError(writer, err)
		return
	}
	actor := r.auditUser(request)
	enc, flusher, ok := beginBatchProgressStream(writer)
	if !ok {
		return
	}
	streamBatchItems(request.Context(), enc, flusher, names, func(ctx context.Context, name string) batchItemResult {
		return r.applyDownsampleAction(ctx, name, action, actor)
	}, r.attachBatchProgressSummary)
}

func streamBatchItems(
	ctx context.Context,
	enc *json.Encoder,
	flusher http.Flusher,
	names []string,
	apply func(context.Context, string) batchItemResult,
	attachSummary func(*batchProgressEvent),
) {
	out := batchMutationResponse{OK: true, Items: make([]batchItemResult, 0, len(names))}
	total := len(names)
	for index, name := range names {
		if err := ctx.Err(); err != nil {
			// 客户端取消/服务端超时：立即停止后续写，并推送已处理部分 summary。
			event := batchProgressEvent{
				Type:      "summary",
				OK:        out.OK && out.Fail == 0,
				OKCount:   out.OKCount,
				Skip:      out.Skip,
				Fail:      out.Fail,
				Items:     out.Items,
				Index:     len(out.Items),
				Total:     total,
				Cancelled: true,
				Message:   err.Error(),
			}
			if attachSummary != nil {
				attachSummary(&event)
			}
			_ = writeBatchProgressEvent(enc, flusher, event)
			return
		}
		item := apply(ctx, name)
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
		_ = writeBatchProgressEvent(enc, flusher, batchProgressEvent{
			Type:    "item",
			Index:   index + 1,
			Total:   total,
			Name:    item.Name,
			Status:  item.Status,
			Message: item.Message,
		})
	}
	event := batchProgressEvent{
		Type:    "summary",
		OK:      out.OK,
		OKCount: out.OKCount,
		Skip:    out.Skip,
		Fail:    out.Fail,
		Items:   out.Items,
		Index:   total,
		Total:   total,
	}
	if attachSummary != nil {
		attachSummary(&event)
	}
	_ = writeBatchProgressEvent(enc, flusher, event)
}

func validateBatchNames(names []string) error {
	if len(names) == 0 {
		return newAPIError(errorCodeBadRequest, "names is required", nil)
	}
	if len(names) > batchMaxItems {
		return newAPIError(errorCodeBadRequest, "too many names", nil)
	}
	return nil
}

func wantsBatchProgressStream(request *http.Request) bool {
	if strings.EqualFold(request.URL.Query().Get("stream"), "1") {
		return true
	}
	accept := strings.ToLower(request.Header.Get("Accept"))
	return strings.Contains(accept, contentTypeNDJSON) || strings.Contains(accept, "ndjson")
}

// batchProgressEvent 批量写进度 NDJSON 事件（item/summary/error）。
type batchProgressEvent struct {
	Type          string                `json:"type"`
	Index         int                   `json:"index,omitempty"`
	Total         int                   `json:"total,omitempty"`
	Name          string                `json:"name,omitempty"`
	Status        string                `json:"status,omitempty"`
	Message       string                `json:"message,omitempty"`
	OK            bool                  `json:"ok,omitempty"`
	OKCount       int                   `json:"ok_count,omitempty"`
	Skip          int                   `json:"skip_count,omitempty"`
	Fail          int                   `json:"fail_count,omitempty"`
	Items         []batchItemResult     `json:"items,omitempty"`
	Cancelled     bool                  `json:"cancelled,omitempty"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

func (r *serverRuntime) attachBatchProgressSummary(event *batchProgressEvent) {
	if r == nil || event == nil {
		return
	}
	busy, op, started := r.adminHeavyState()
	event.AdminOpBusy = busy
	event.Op = op
	event.StartedAtUnix = started
	event.Last = r.lastAdminHeavySnapshot()
}

func beginBatchProgressStream(writer http.ResponseWriter) (*json.Encoder, http.Flusher, bool) {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		writeAPIError(writer, newAPIError(errorCodeInternal, "streaming unsupported", nil))
		return nil, nil, false
	}
	writer.Header().Set("Content-Type", contentTypeNDJSON)
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusOK)
	flusher.Flush()
	return json.NewEncoder(writer), flusher, true
}

func writeBatchProgressEvent(enc *json.Encoder, flusher http.Flusher, event batchProgressEvent) error {
	if err := enc.Encode(event); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
