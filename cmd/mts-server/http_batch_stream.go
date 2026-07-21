package main

import (
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
	out := batchMutationResponse{OK: true, Items: make([]batchItemResult, 0, len(names))}
	total := len(names)
	for index, name := range names {
		if err := request.Context().Err(); err != nil {
			_ = writeBatchProgressEvent(enc, flusher, batchProgressEvent{
				Type:    "error",
				Message: err.Error(),
			})
			return
		}
		item := r.applyUserDisabled(request.Context(), name, req.Disabled, actor)
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
	_ = writeBatchProgressEvent(enc, flusher, batchProgressEvent{
		Type:    "summary",
		OK:      out.OK,
		OKCount: out.OKCount,
		Skip:    out.Skip,
		Fail:    out.Fail,
		Items:   out.Items,
		Index:   total,
		Total:   total,
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
	out := batchMutationResponse{OK: true, Items: make([]batchItemResult, 0, len(names))}
	total := len(names)
	for index, name := range names {
		if err := request.Context().Err(); err != nil {
			_ = writeBatchProgressEvent(enc, flusher, batchProgressEvent{
				Type:    "error",
				Message: err.Error(),
			})
			return
		}
		item := r.applyDownsampleAction(request.Context(), name, action, actor)
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
	_ = writeBatchProgressEvent(enc, flusher, batchProgressEvent{
		Type:    "summary",
		OK:      out.OK,
		OKCount: out.OKCount,
		Skip:    out.Skip,
		Fail:    out.Fail,
		Items:   out.Items,
		Index:   total,
		Total:   total,
	})
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
	Type    string            `json:"type"`
	Index   int               `json:"index,omitempty"`
	Total   int               `json:"total,omitempty"`
	Name    string            `json:"name,omitempty"`
	Status  string            `json:"status,omitempty"`
	Message string            `json:"message,omitempty"`
	OK      bool              `json:"ok,omitempty"`
	OKCount int               `json:"ok_count,omitempty"`
	Skip    int               `json:"skip_count,omitempty"`
	Fail    int               `json:"fail_count,omitempty"`
	Items   []batchItemResult `json:"items,omitempty"`
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
