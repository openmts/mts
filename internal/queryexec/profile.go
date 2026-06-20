package queryexec

import "time"

type OperatorProfile struct {
	ID                string
	Kind              string
	RowsOut           int
	ColumnsOut        int
	SamplesOut        int
	BytesOut          int64
	StartedUnixNanos  int64
	FinishedUnixNanos int64
	Duration          time.Duration
	Error             string
}

type Profile struct {
	Operators []OperatorProfile
}
