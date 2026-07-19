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
