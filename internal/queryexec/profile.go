package queryexec

import "time"

type OperatorProfile struct {
	ID       string
	RowsOut  int
	Duration time.Duration
	Error    string
}

type Profile struct {
	Operators []OperatorProfile
}
