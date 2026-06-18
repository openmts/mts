package queryservice_test

import (
	"context"
	"errors"
	"testing"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryservice"
)

func TestServiceRejectsWhenAdmissionIsFull(t *testing.T) {
	service := queryservice.New(queryservice.Options{MaxConcurrent: 1}, blockingExecutor{})
	release, err := service.Admit(context.Background())
	if err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	defer release()
	_, err = service.Query(context.Background(), queryservice.Request{Query: model.Query{Measurement: "cpu"}})
	if !errors.Is(err, queryservice.ErrAdmissionRejected) {
		t.Fatalf("Query() error = %v, want admission rejected", err)
	}
}

func TestCompatExecutorUsesRowsForRawQueriesAndColumnsForAggregates(t *testing.T) {
	reader := fakeReader{
		rows:    []model.Row{{Measurement: "cpu"}},
		columns: []model.ColumnSeries{{Measurement: "cpu"}},
	}
	executor := queryservice.NewCompatExecutor(reader)
	raw, err := executor.Query(context.Background(), model.Query{Measurement: "cpu"})
	if err != nil {
		t.Fatalf("Query(raw) error = %v", err)
	}
	if len(raw.Rows) != 1 || len(raw.Columns) != 0 {
		t.Fatalf("raw result = %#v, want rows only", raw)
	}
	aggregated, err := executor.Query(context.Background(), model.Query{
		Measurement: "cpu",
		Aggregates:  []model.AggregateSpec{{Field: "usage", Function: "avg"}},
	})
	if err != nil {
		t.Fatalf("Query(aggregate) error = %v", err)
	}
	if len(aggregated.Columns) != 1 || len(aggregated.Rows) != 0 {
		t.Fatalf("aggregate result = %#v, want columns only", aggregated)
	}
}

type blockingExecutor struct{}

func (blockingExecutor) Query(context.Context, model.Query) (queryservice.Result, error) {
	return queryservice.Result{}, nil
}

type fakeReader struct {
	rows    []model.Row
	columns []model.ColumnSeries
}

func (f fakeReader) QueryColumns(context.Context, model.Query) ([]model.ColumnSeries, error) {
	return append([]model.ColumnSeries(nil), f.columns...), nil
}

func (f fakeReader) QueryRows(context.Context, model.Query) ([]model.Row, error) {
	return append([]model.Row(nil), f.rows...), nil
}
