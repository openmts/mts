package querylang

import (
	"strings"

	"github.com/openmts/mts/internal/model"
)

func FromModelQuery(query model.Query, defaults Defaults) (QuerySpec, error) {
	spec := QuerySpec{
		Scope: Scope{
			Database:        firstNonEmpty(query.Database, defaults.Database),
			RetentionPolicy: firstNonEmpty(query.RetentionPolicy, defaults.RetentionPolicy),
		},
		Measurement: query.Measurement,
		Tags:        cloneTags(query.Tags),
		Fields:      append([]string(nil), query.Fields...),
		TimeRange: TimeRange{
			Start: query.StartTime,
			End:   query.EndTime,
		},
		Aggregates: convertAggregates(query.Aggregates),
		Window:     query.Window,
		Limit:      query.Limit,
		Offset:     query.Offset,
		Output:     Output{Kind: defaults.Output},
	}
	if spec.Output.Kind == 0 {
		spec.Output.Kind = OutputColumns
	}
	if err := validate(spec); err != nil {
		return QuerySpec{}, err
	}
	return spec, nil
}

func validate(spec QuerySpec) error {
	if strings.TrimSpace(spec.Measurement) == "" {
		return newError(ErrInvalidMeasurement, "measurement is required")
	}
	if spec.TimeRange.End != 0 && spec.TimeRange.Start > spec.TimeRange.End {
		return newError(ErrInvalidTimeRange, "start time must be less than or equal to end time")
	}
	return nil
}

func convertAggregates(specs []model.AggregateSpec) []Aggregate {
	out := make([]Aggregate, 0, len(specs))
	for _, spec := range specs {
		out = append(out, Aggregate{
			Field:    spec.Field,
			Function: normalizeFunction(spec.Function),
		})
	}
	return out
}

func normalizeFunction(function string) string {
	normalized := strings.ToLower(strings.TrimSpace(function))
	if normalized == "mean" {
		return "avg"
	}
	return normalized
}

func cloneTags(tags map[string]string) map[string]string {
	if len(tags) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(tags))
	for key, value := range tags {
		out[key] = value
	}
	return out
}

func firstNonEmpty(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
