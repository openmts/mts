package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestQueryRowIteratorProjectsFieldsAtStorage(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = eng.Close(ctx) }()
	if err := eng.Write(ctx, []model.Point{{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   10,
		Fields: map[string]model.FieldValue{
			"usage": model.Float64Value(1.5),
			"temp":  model.Float64Value(30),
		},
	}}, model.WriteOptions{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := eng.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	iter, err := eng.QueryRowIterator(ctx, model.Query{
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     100,
		Fields:      []string{"usage"},
	})
	if err != nil {
		t.Fatalf("QueryRowIterator: %v", err)
	}
	defer func() { _ = iter.Close() }()
	if !iter.Next() {
		t.Fatalf("Next=false err=%v", iter.Err())
	}
	row := iter.Row()
	if len(row.Fields) != 1 {
		t.Fatalf("fields=%#v want only usage", row.Fields)
	}
	if _, ok := row.Fields["usage"]; !ok {
		t.Fatalf("missing usage: %#v", row.Fields)
	}
	if _, ok := row.Fields["temp"]; ok {
		t.Fatalf("temp should be projected out: %#v", row.Fields)
	}
}

func TestProjectedRowStreamExactFieldsDoesNotAllocate(t *testing.T) {
	source := projectionRowStream{row: model.Row{
		Fields: map[string]model.FieldValue{"usage": model.Float64Value(1.5)},
	}}
	stream := newProjectedRowStream(source, []string{"usage"})

	allocs := testing.AllocsPerRun(100, func() {
		if !stream.Next() {
			panic("projection stream unexpectedly ended")
		}
	})
	if allocs != 0 {
		t.Fatalf("exact projection allocations = %.2f, want 0", allocs)
	}
	if len(stream.Row().Fields) != 1 {
		t.Fatalf("projected fields = %#v, want usage only", stream.Row().Fields)
	}
}

type projectionRowStream struct {
	row model.Row
}

func (s projectionRowStream) Next() bool {
	return true
}

func (s projectionRowStream) Row() model.Row {
	return s.row
}

func (projectionRowStream) Err() error {
	return nil
}

func (projectionRowStream) Close() error {
	return nil
}
