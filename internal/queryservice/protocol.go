package queryservice

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/openmts/mts/internal/model"
)

type ErrorCode string

const (
	ErrorCodeBadRequest             ErrorCode = "bad_request"
	ErrorCodeAdmissionRejected      ErrorCode = "admission_rejected"
	ErrorCodeQueueFull              ErrorCode = "queue_full"
	ErrorCodeStreamingUnsupported   ErrorCode = "streaming_unsupported"
	ErrorCodeUnauthorized           ErrorCode = "unauthorized"
	ErrorCodeLanguageUnsupported    ErrorCode = "language_unsupported"
	ErrorCodeDistributedUnsupported ErrorCode = "distributed_unsupported"
	ErrorCodeQueryFailed            ErrorCode = "query_failed"
)

type ErrorResponse struct {
	OK      bool      `json:"ok"`
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

type QueryResponse struct {
	OK      bool               `json:"ok"`
	Result  Result             `json:"result,omitempty"`
	Error   *ErrorResponse     `json:"error,omitempty"`
	Cursor  string             `json:"cursor,omitempty"`
	Stats   model.QueryStats   `json:"stats,omitempty"`
	Explain model.QueryExplain `json:"explain,omitempty"`
}

type StatsResponse struct {
	OK    bool         `json:"ok"`
	Stats ServiceStats `json:"stats"`
}

type AuditResponse struct {
	OK      bool          `json:"ok"`
	Records []AuditRecord `json:"records"`
}

type StreamRecord struct {
	Type    string              `json:"type"`
	Row     *model.Row          `json:"row,omitempty"`
	Column  *model.ColumnSeries `json:"column,omitempty"`
	Error   *ErrorResponse      `json:"error,omitempty"`
	Stats   model.QueryStats    `json:"stats,omitempty"`
	Explain model.QueryExplain  `json:"explain,omitempty"`
}

func NewHTTPHandler(service *Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/query", queryHandler(service))
	mux.HandleFunc("/query/audit", queryAuditHandler(service))
	mux.HandleFunc("/query/stats", queryStatsHandler(service))
	mux.HandleFunc("/query/stream", queryStreamHandler(service))
	return mux
}

func queryAuditHandler(service *Service) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writeProtocolError(writer, http.StatusMethodNotAllowed, ErrorCodeBadRequest, "method not allowed")
			return
		}
		writeProtocolJSON(writer, http.StatusOK, AuditResponse{OK: true, Records: service.AuditRecords()})
	}
}

func queryStatsHandler(service *Service) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writeProtocolError(writer, http.StatusMethodNotAllowed, ErrorCodeBadRequest, "method not allowed")
			return
		}
		writeProtocolJSON(writer, http.StatusOK, StatsResponse{OK: true, Stats: service.Stats()})
	}
}

func queryHandler(service *Service) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeProtocolError(writer, http.StatusMethodNotAllowed, ErrorCodeBadRequest, "method not allowed")
			return
		}
		queryRequest, ok := decodeQueryRequest(writer, request)
		if !ok {
			return
		}
		result, err := service.Query(request.Context(), queryRequest)
		if err != nil {
			writeQueryError(writer, err)
			return
		}
		writeProtocolJSON(writer, http.StatusOK, QueryResponse{
			OK:      true,
			Result:  result,
			Stats:   result.Stats,
			Explain: result.Explain,
		})
	}
}

func queryStreamHandler(service *Service) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeProtocolError(writer, http.StatusMethodNotAllowed, ErrorCodeBadRequest, "method not allowed")
			return
		}
		queryRequest, ok := decodeQueryRequest(writer, request)
		if !ok {
			return
		}
		result, err := service.QueryStream(request.Context(), queryRequest)
		if err != nil {
			writeQueryError(writer, err)
			return
		}
		writer.Header().Set("Content-Type", "application/x-ndjson")
		writer.WriteHeader(http.StatusOK)
		encoder := json.NewEncoder(writer)
		if result.Rows != nil {
			writeRowStreamRecords(encoder, result)
			return
		}
		writeColumnStreamRecords(encoder, result)
	}
}

func decodeQueryRequest(writer http.ResponseWriter, request *http.Request) (Request, bool) {
	defer func() {
		_ = request.Body.Close()
	}()
	var queryRequest Request
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&queryRequest); err != nil {
		writeProtocolError(writer, http.StatusBadRequest, ErrorCodeBadRequest, err.Error())
		return Request{}, false
	}
	return queryRequest, true
}

func writeRowStreamRecords(encoder *json.Encoder, result StreamResult) {
	defer func() {
		_ = result.Rows.Close()
	}()
	for result.Rows.Next() {
		row := result.Rows.Row()
		if err := encoder.Encode(StreamRecord{Type: "row", Row: &row}); err != nil {
			return
		}
	}
	if err := result.Rows.Err(); err != nil {
		_ = encoder.Encode(StreamRecord{
			Type:  "error",
			Error: errorPayload(err),
		})
		return
	}
	_ = encoder.Encode(StreamRecord{Type: "end", Stats: result.Stats, Explain: result.Explain})
}

func writeColumnStreamRecords(encoder *json.Encoder, result StreamResult) {
	if result.Columns == nil {
		_ = encoder.Encode(StreamRecord{Type: "end", Stats: result.Stats, Explain: result.Explain})
		return
	}
	defer func() {
		_ = result.Columns.Close()
	}()
	for result.Columns.Next() {
		column := result.Columns.Column()
		if err := encoder.Encode(StreamRecord{Type: "column", Column: &column}); err != nil {
			return
		}
	}
	if err := result.Columns.Err(); err != nil {
		_ = encoder.Encode(StreamRecord{
			Type:  "error",
			Error: errorPayload(err),
		})
		return
	}
	_ = encoder.Encode(StreamRecord{Type: "end", Stats: result.Stats, Explain: result.Explain})
}

func writeQueryError(writer http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, ErrAdmissionRejected) {
		status = http.StatusTooManyRequests
	}
	if errors.Is(err, ErrQueueFull) {
		status = http.StatusTooManyRequests
	}
	if errors.Is(err, ErrStreamingUnsupported) {
		status = http.StatusNotImplemented
	}
	if errors.Is(err, ErrUnauthorized) {
		status = http.StatusForbidden
	}
	if errors.Is(err, ErrUnsupportedQueryLanguage) ||
		errors.Is(err, ErrDistributedUnsupported) {
		status = http.StatusNotImplemented
	}
	payload := errorPayload(err)
	writeProtocolJSON(writer, status, QueryResponse{OK: false, Error: payload})
}

func writeProtocolError(
	writer http.ResponseWriter,
	status int,
	code ErrorCode,
	message string,
) {
	writeProtocolJSON(writer, status, QueryResponse{
		OK:    false,
		Error: &ErrorResponse{OK: false, Code: code, Message: message},
	})
}

func writeProtocolJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func errorPayload(err error) *ErrorResponse {
	return &ErrorResponse{OK: false, Code: errorCode(err), Message: err.Error()}
}

func errorCode(err error) ErrorCode {
	code := ErrorCodeQueryFailed
	if errors.Is(err, ErrAdmissionRejected) {
		code = ErrorCodeAdmissionRejected
	}
	if errors.Is(err, ErrQueueFull) {
		code = ErrorCodeQueueFull
	}
	if errors.Is(err, ErrStreamingUnsupported) {
		code = ErrorCodeStreamingUnsupported
	}
	if errors.Is(err, ErrUnauthorized) {
		code = ErrorCodeUnauthorized
	}
	if errors.Is(err, ErrUnsupportedQueryLanguage) {
		code = ErrorCodeLanguageUnsupported
	}
	if errors.Is(err, ErrDistributedUnsupported) {
		code = ErrorCodeDistributedUnsupported
	}
	return code
}
