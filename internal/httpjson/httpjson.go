package httpjson

import (
	"encoding/json"
	"net/http"
)

const ContentTypeJSON = "application/json"

func DecodeStrict(request *http.Request, value any) error {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}

func Write(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", ContentTypeJSON)
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func WriteRaw(writer http.ResponseWriter, status int, contentType string, payload []byte) {
	writer.Header().Set("Content-Type", contentType)
	writer.WriteHeader(status)
	_, _ = writer.Write(payload)
}
