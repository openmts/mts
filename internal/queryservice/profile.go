package queryservice

import (
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryexec"
)

type profileRecorder struct {
	profile queryexec.Profile
}

func (r *profileRecorder) record(
	id string,
	start time.Time,
	err error,
	apply func(*queryexec.OperatorProfile),
) *queryexec.OperatorProfile {
	entry := queryexec.OperatorProfile{
		ID:                id,
		Kind:              id,
		StartedUnixNanos:  start.UnixNano(),
		FinishedUnixNanos: time.Now().UnixNano(),
		Duration:          time.Since(start),
	}
	if err != nil {
		entry.Error = err.Error()
	}
	if apply != nil {
		apply(&entry)
	}
	r.profile.Operators = append(r.profile.Operators, entry)
	return &r.profile.Operators[len(r.profile.Operators)-1]
}

func countColumnSamples(columns []model.ColumnSeries) int {
	total := 0
	for _, column := range columns {
		total += len(column.Values)
	}
	return total
}
