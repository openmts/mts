package querylang

import (
	"time"

	"github.com/openmts/mts/internal/model"
)

type Defaults struct {
	Database        string
	RetentionPolicy string
	Output          OutputKind
}

type Scope struct {
	Database        string
	RetentionPolicy string
}

type TimeRange struct {
	Start int64
	End   int64
}

type Aggregate struct {
	Field    string
	Function string
}

type OutputKind uint8

const (
	OutputColumns OutputKind = iota + 1
	OutputRows
	OutputAggregates
	OutputExplain
	OutputProfile
)

type Output struct {
	Kind OutputKind
}

type QuerySpec struct {
	Scope       Scope
	Measurement string
	Tags        map[string]string
	Fields      []string
	TimeRange   TimeRange
	Aggregates  []Aggregate
	Window      time.Duration
	Limit       int
	Offset      int
	Output      Output
}

func (s QuerySpec) ToModelQuery() model.Query {
	aggregates := make([]model.AggregateSpec, 0, len(s.Aggregates))
	for _, aggregate := range s.Aggregates {
		aggregates = append(aggregates, model.AggregateSpec{
			Field:    aggregate.Field,
			Function: aggregate.Function,
		})
	}
	return model.Query{
		Database:        s.Scope.Database,
		RetentionPolicy: s.Scope.RetentionPolicy,
		Measurement:     s.Measurement,
		Tags:            cloneTags(s.Tags),
		Fields:          append([]string(nil), s.Fields...),
		StartTime:       s.TimeRange.Start,
		EndTime:         s.TimeRange.End,
		Aggregates:      aggregates,
		Window:          s.Window,
		Limit:           s.Limit,
		Offset:          s.Offset,
	}
}
