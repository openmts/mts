package queryexec_test

import (
	"context"
	"testing"

	"github.com/openmts/mts/internal/queryexec"
)

func TestPipelineProfilesOperatorsAndClosesOnLimit(t *testing.T) {
	source := queryexec.NewCountingOperator("scan", 3)
	pipeline := queryexec.NewPipeline(source, queryexec.PipelineOptions{Limit: 2})
	count := 0
	for pipeline.Next(context.Background()) {
		count++
	}
	if err := pipeline.Err(); err != nil {
		t.Fatalf("Err() = %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	profile := pipeline.Profile()
	if len(profile.Operators) != 1 || profile.Operators[0].RowsOut != 2 {
		t.Fatalf("profile = %#v, want one operator with two rows", profile)
	}
	if !source.Closed() {
		t.Fatal("source closed = false, want true after limit")
	}
}

func TestPipelineCloseAndCountingOperatorErrAreStable(t *testing.T) {
	source := queryexec.NewCountingOperator("scan", 1)
	if err := source.Err(); err != nil {
		t.Fatalf("source Err() = %v, want nil", err)
	}
	pipeline := queryexec.NewPipeline(source, queryexec.PipelineOptions{})
	if err := pipeline.Close(); err != nil {
		t.Fatalf("pipeline Close() error = %v", err)
	}
	if !source.Closed() {
		t.Fatal("source closed = false, want true")
	}
	if pipeline.Next(context.Background()) {
		t.Fatal("pipeline Next(after close) = true, want false")
	}
}
