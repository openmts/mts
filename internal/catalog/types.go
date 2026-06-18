package catalog

import "github.com/openmts/mts/internal/model"

type Series struct {
	ID          uint64            `json:"id"`
	Measurement string            `json:"measurement"`
	Tags        map[string]string `json:"tags"`
}

type Field struct {
	ID          uint32          `json:"id"`
	Measurement string          `json:"measurement"`
	Name        string          `json:"name"`
	Type        model.FieldType `json:"type"`
}

type snapshot struct {
	NextSeriesID uint64   `json:"next_series_id"`
	NextFieldID  uint32   `json:"next_field_id"`
	Series       []Series `json:"series"`
	Fields       []Field  `json:"fields"`
}

type walEntry struct {
	Type   string  `json:"type"`
	Series *Series `json:"series,omitempty"`
	Field  *Field  `json:"field,omitempty"`
}
