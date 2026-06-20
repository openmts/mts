package queryexec

import (
	"context"
	"errors"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestFilteredColumnStreamFiltersMatchingFieldValues(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{{
		FieldName:  "usage",
		FieldType:  model.FieldFloat64,
		Timestamps: []int64{1, 2, 3},
		Values: []model.FieldValue{
			model.Float64Value(0.2),
			model.Float64Value(0.8),
			model.Float64Value(0.9),
		},
	}})
	stream := NewFilteredColumnStream(source, []model.QueryPredicate{{
		Kind:  model.QueryPredicateFieldGT,
		Name:  "usage",
		Value: model.Float64Value(0.5),
	}})
	if !stream.Next() {
		t.Fatalf("Next() = false, err = %v", stream.Err())
	}
	column := stream.Column()
	if got := column.Timestamps; len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("timestamps = %v, want [2 3]", got)
	}
	if stream.Next() {
		t.Fatal("Next() = true, want false")
	}
}

func TestFilteredRowStreamFiltersFieldValues(t *testing.T) {
	source := NewSliceRowStream([]model.Row{
		{Timestamp: 1, Fields: map[string]model.FieldValue{"usage": model.Float64Value(0.2)}},
		{Timestamp: 2, Fields: map[string]model.FieldValue{"usage": model.Float64Value(0.8)}},
	})
	stream := NewFilteredRowStream(source, []model.QueryPredicate{{
		Kind:  model.QueryPredicateFieldGTE,
		Name:  "usage",
		Value: model.Float64Value(0.8),
	}})
	if !stream.Next() {
		t.Fatalf("Next() = false, err = %v", stream.Err())
	}
	if got := stream.Row().Timestamp; got != 2 {
		t.Fatalf("timestamp = %d, want 2", got)
	}
	if stream.Next() {
		t.Fatal("Next() = true, want false")
	}
}

func TestFilteredRowStreamSupportsStringBoolAndNotEqual(t *testing.T) {
	source := NewSliceRowStream([]model.Row{
		{
			Timestamp: 1,
			Fields: map[string]model.FieldValue{
				"state": model.StringValue("bad"),
				"ready": model.BoolValue(true),
			},
		},
		{
			Timestamp: 2,
			Fields: map[string]model.FieldValue{
				"state": model.StringValue("ok"),
				"ready": model.BoolValue(true),
			},
		},
	})
	stream := NewFilteredRowStream(source, []model.QueryPredicate{
		{Kind: model.QueryPredicateFieldNe, Name: "state", Value: model.StringValue("bad")},
		{Kind: model.QueryPredicateFieldEq, Name: "ready", Value: model.BoolValue(true)},
	})
	if !stream.Next() {
		t.Fatalf("Next() = false, err = %v", stream.Err())
	}
	if got := stream.Row().Timestamp; got != 2 {
		t.Fatalf("timestamp = %d, want 2", got)
	}
}

func TestExprFilteredColumnStreamAppliesMultiFieldRowExpression(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{
		{
			SeriesID:    1,
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			FieldName:   "usage",
			FieldType:   model.FieldFloat64,
			Timestamps:  []int64{1, 2, 3},
			Values:      []model.FieldValue{model.Float64Value(0.9), model.Float64Value(0.4), model.Float64Value(0.8)},
		},
		{
			SeriesID:    1,
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			FieldName:   "temp",
			FieldType:   model.FieldInt64,
			Timestamps:  []int64{1, 2, 3},
			Values:      []model.FieldValue{model.Int64Value(40), model.Int64Value(70), model.Int64Value(80)},
		},
	})
	expr := model.QueryExpr{
		Kind: model.QueryExprAnd,
		Children: []model.QueryExpr{
			{
				Kind: model.QueryExprPredicate,
				Predicate: model.QueryPredicate{
					Kind:  model.QueryPredicateFieldGT,
					Name:  "usage",
					Value: model.Float64Value(0.5),
				},
			},
			{
				Kind: model.QueryExprPredicate,
				Predicate: model.QueryPredicate{
					Kind:  model.QueryPredicateFieldGT,
					Name:  "temp",
					Value: model.Int64Value(50),
				},
			},
		},
	}
	stream := NewExprFilteredColumnStream(source, expr)
	columns := collectColumnSeriesStream(t, stream)
	if len(columns) != 2 {
		t.Fatalf("columns = %#v, want usage and temp", columns)
	}
	for _, column := range columns {
		if len(column.Timestamps) != 1 || column.Timestamps[0] != 3 {
			t.Fatalf("column %s timestamps = %v, want [3]", column.FieldName, column.Timestamps)
		}
	}
}

func TestExprFilteredColumnStreamReturnsSourceForEmptyExpression(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{{
		FieldName:  "usage",
		Timestamps: []int64{1},
		Values:     []model.FieldValue{model.Float64Value(1)},
	}})
	stream := NewExprFilteredColumnStream(source, model.QueryExpr{})
	if !stream.Next() {
		t.Fatalf("Next() = false, err = %v", stream.Err())
	}
	if got := stream.Column().FieldName; got != "usage" {
		t.Fatalf("field = %q, want usage", got)
	}
}

func TestExprFilteredRowStreamAppliesBooleanTagAndFieldExpression(t *testing.T) {
	source := NewSliceRowStream([]model.Row{
		{
			Timestamp: 5,
			Tags:      map[string]string{"host": "a", "region": "west"},
			Fields:    map[string]model.FieldValue{"usage": model.Float64Value(0.9)},
		},
		{
			Timestamp: 15,
			Tags:      map[string]string{"host": "b"},
			Fields:    map[string]model.FieldValue{"usage": model.Float64Value(0.4)},
		},
		{
			Timestamp: 25,
			Tags:      map[string]string{"host": "c", "region": "east"},
			Fields:    map[string]model.FieldValue{"usage": model.Float64Value(0.7)},
		},
	})
	expr := model.QueryExpr{
		Kind: model.QueryExprOr,
		Children: []model.QueryExpr{
			{
				Kind: model.QueryExprAnd,
				Children: []model.QueryExpr{
					{
						Kind: model.QueryExprPredicate,
						Predicate: model.QueryPredicate{
							Kind:         model.QueryPredicateTagIn,
							Name:         "host",
							StringValues: []string{"a", "z"},
						},
					},
					{
						Kind: model.QueryExprPredicate,
						Predicate: model.QueryPredicate{
							Kind:  model.QueryPredicateFieldGT,
							Name:  "usage",
							Value: model.Float64Value(0.5),
						},
					},
				},
			},
			{
				Kind: model.QueryExprNot,
				Children: []model.QueryExpr{{
					Kind: model.QueryExprPredicate,
					Predicate: model.QueryPredicate{
						Kind: model.QueryPredicateTagExists,
						Name: "region",
					},
				}},
			},
		},
	}
	stream := NewExprFilteredRowStream(source, expr)
	var timestamps []int64
	for stream.Next() {
		timestamps = append(timestamps, stream.Row().Timestamp)
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Err() = %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if len(timestamps) != 2 || timestamps[0] != 5 || timestamps[1] != 15 {
		t.Fatalf("timestamps = %v, want [5 15]", timestamps)
	}
}

func TestExprFilteredRowStreamCoversTagNeTimeRangeAndEmptyNot(t *testing.T) {
	source := NewSliceRowStream([]model.Row{
		{Timestamp: 1, Tags: map[string]string{"host": "a"}},
		{Timestamp: 5, Tags: map[string]string{"host": "b"}},
		{Timestamp: 9, Tags: map[string]string{}},
	})
	expr := model.QueryExpr{
		Kind: model.QueryExprAnd,
		Children: []model.QueryExpr{
			{
				Kind: model.QueryExprPredicate,
				Predicate: model.QueryPredicate{
					Kind:  model.QueryPredicateTimeRange,
					Start: 5,
					End:   10,
				},
			},
			{
				Kind: model.QueryExprPredicate,
				Predicate: model.QueryPredicate{
					Kind:         model.QueryPredicateTagNe,
					Name:         "host",
					StringValues: []string{"a"},
				},
			},
			{Kind: model.QueryExprNot},
		},
	}
	stream := NewExprFilteredRowStream(source, expr)
	var timestamps []int64
	for stream.Next() {
		timestamps = append(timestamps, stream.Row().Timestamp)
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Err() = %v", err)
	}
	if len(timestamps) != 2 || timestamps[0] != 5 || timestamps[1] != 9 {
		t.Fatalf("timestamps = %v, want [5 9]", timestamps)
	}

	source = NewSliceRowStream([]model.Row{{Timestamp: 1}})
	if got := NewExprFilteredRowStream(source, model.QueryExpr{}); got != source {
		t.Fatal("empty expression should return source row stream")
	}
	stream = NewExprFilteredRowStream(NewSliceRowStream([]model.Row{{Timestamp: 1}}), model.QueryExpr{
		Kind: model.QueryExprKind(99),
	})
	if stream.Next() {
		t.Fatal("unknown expression matched row, want no rows")
	}
}

func TestContextStreamsCloseOnCancellationAndExposeInnerValues(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	rows := NewSliceRowStream([]model.Row{{Timestamp: 1}})
	rowStream := WithContextRowStream(ctx, rows)
	if !rowStream.Next() || rowStream.Row().Timestamp != 1 {
		t.Fatalf("row stream first row = %#v err=%v, want timestamp 1", rowStream.Row(), rowStream.Err())
	}
	cancel()
	if rowStream.Next() {
		t.Fatal("rowStream.Next() after cancel = true, want false")
	}
	if !errors.Is(rowStream.Err(), context.Canceled) {
		t.Fatalf("rowStream.Err() = %v, want context canceled", rowStream.Err())
	}
	if err := rowStream.Close(); err != nil {
		t.Fatalf("rowStream.Close() error = %v", err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	columns := NewSliceColumnSeriesStream([]model.ColumnSeries{{FieldName: "usage"}})
	columnStream := WithContextColumnStream(ctx, columns)
	if !columnStream.Next() || columnStream.Column().FieldName != "usage" {
		t.Fatalf("column stream first column = %#v err=%v, want usage", columnStream.Column(), columnStream.Err())
	}
	cancel()
	if columnStream.Next() {
		t.Fatal("columnStream.Next() after cancel = true, want false")
	}
	if !errors.Is(columnStream.Err(), context.Canceled) {
		t.Fatalf("columnStream.Err() = %v, want context canceled", columnStream.Err())
	}
	if err := columnStream.Close(); err != nil {
		t.Fatalf("columnStream.Close() error = %v", err)
	}

	rowSource := NewSliceRowStream([]model.Row{{Timestamp: 7}})
	var nilCtx context.Context
	if got := WithContextRowStream(nilCtx, rowSource); got != rowSource {
		t.Fatal("WithContextRowStream(nil) did not return source")
	}
	columnSource := NewSliceColumnSeriesStream([]model.ColumnSeries{{FieldName: "usage"}})
	if got := WithContextColumnStream(nilCtx, columnSource); got != columnSource {
		t.Fatal("WithContextColumnStream(nil) did not return source")
	}
}

func TestBudgetAndPaginationStreamsCloseAndReportErrors(t *testing.T) {
	rowBudget := NewBudgetRowStream(
		NewSliceRowStream([]model.Row{{Timestamp: 1}, {Timestamp: 2}}),
		model.QueryBudget{MaxSamples: 1},
	)
	if !rowBudget.Next() {
		t.Fatalf("rowBudget.Next(first) = false err=%v", rowBudget.Err())
	}
	if rowBudget.Row().Timestamp != 1 {
		t.Fatalf("rowBudget.Row() = %#v, want timestamp 1", rowBudget.Row())
	}
	if rowBudget.Next() {
		t.Fatal("rowBudget.Next(over budget) = true, want false")
	}
	if !errors.Is(rowBudget.Err(), ErrReadBudgetExceeded) {
		t.Fatalf("rowBudget.Err() = %v, want read budget exceeded", rowBudget.Err())
	}
	if err := rowBudget.Close(); err != nil {
		t.Fatalf("rowBudget.Close() error = %v", err)
	}

	columnBudget := NewBudgetColumnStream(
		NewSliceColumnSeriesStream([]model.ColumnSeries{{
			Values: []model.FieldValue{model.Int64Value(1), model.Int64Value(2)},
		}}),
		model.QueryBudget{MaxSamples: 1},
	)
	if columnBudget.Next() {
		t.Fatal("columnBudget.Next(over budget) = true, want false")
	}
	if !errors.Is(columnBudget.Err(), ErrReadBudgetExceeded) {
		t.Fatalf("columnBudget.Err() = %v, want read budget exceeded", columnBudget.Err())
	}
	if err := columnBudget.Close(); err != nil {
		t.Fatalf("columnBudget.Close() error = %v", err)
	}

	paged := NewPaginatedRowStream(NewSliceRowStream([]model.Row{
		{Timestamp: 1},
		{Timestamp: 2},
		{Timestamp: 3},
	}), 1, 1)
	if !paged.Next() || paged.Row().Timestamp != 2 {
		t.Fatalf("paged row = %#v err=%v, want timestamp 2", paged.Row(), paged.Err())
	}
	if paged.Next() {
		t.Fatal("paged.Next(after limit) = true, want false")
	}
	if err := paged.Close(); err != nil {
		t.Fatalf("paged.Close() error = %v", err)
	}
}

func TestFilteredStreamsExposeErrCloseAndNilInputs(t *testing.T) {
	columnStream := NewFilteredColumnStream(nil, []model.QueryPredicate{{
		Kind:  model.QueryPredicateFieldEq,
		Name:  "ready",
		Value: model.BoolValue(true),
	}})
	if columnStream.Next() || columnStream.Column().FieldName != "" || columnStream.Err() != nil {
		t.Fatal("nil filtered column stream returned data or error")
	}
	if err := columnStream.Close(); err != nil {
		t.Fatalf("nil filtered column Close() error = %v", err)
	}

	rowStream := NewFilteredRowStream(nil, []model.QueryPredicate{{
		Kind:  model.QueryPredicateFieldEq,
		Name:  "ready",
		Value: model.BoolValue(true),
	}})
	if rowStream.Next() || rowStream.Row().Timestamp != 0 || rowStream.Err() != nil {
		t.Fatal("nil filtered row stream returned data or error")
	}
	if err := rowStream.Close(); err != nil {
		t.Fatalf("nil filtered row Close() error = %v", err)
	}

	if !valueMatchesPredicate(model.BoolValue(false), model.QueryPredicate{
		Kind:  model.QueryPredicateFieldLTE,
		Value: model.BoolValue(true),
	}) {
		t.Fatal("bool false <= true = false, want true")
	}
	if valueMatchesPredicate(model.BoolValue(true), model.QueryPredicate{
		Kind:  model.QueryPredicateFieldLT,
		Value: model.BoolValue(false),
	}) {
		t.Fatal("bool true < false = true, want false")
	}
}

func TestPipelineRecordsSourceErrorsAndContextCancellation(t *testing.T) {
	sourceErr := errors.New("source failed")
	pipeline := NewPipeline(&errorOperator{id: "scan", err: sourceErr}, PipelineOptions{})
	if pipeline.Next(context.Background()) {
		t.Fatal("pipeline.Next(error source) = true, want false")
	}
	if !errors.Is(pipeline.Err(), sourceErr) {
		t.Fatalf("pipeline.Err() = %v, want source error", pipeline.Err())
	}
	profile := pipeline.Profile()
	if len(profile.Operators) != 1 || profile.Operators[0].Error == "" {
		t.Fatalf("profile = %#v, want recorded operator error", profile)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cancelled := NewPipeline(NewCountingOperator("scan", 1), PipelineOptions{})
	if cancelled.Next(ctx) {
		t.Fatal("pipeline.Next(cancelled) = true, want false")
	}
	if !errors.Is(cancelled.Err(), context.Canceled) {
		t.Fatalf("cancelled.Err() = %v, want context canceled", cancelled.Err())
	}
}

func TestProjectedColumnStreamKeepsSelectedFields(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{
		{FieldName: "temp", Timestamps: []int64{1}, Values: []model.FieldValue{model.Int64Value(60)}},
		{FieldName: "usage", Timestamps: []int64{1}, Values: []model.FieldValue{model.Float64Value(0.8)}},
	})
	stream := NewProjectedColumnStream(source, []string{"usage"})
	columns := collectColumnSeriesStream(t, stream)
	if len(columns) != 1 || columns[0].FieldName != "usage" {
		t.Fatalf("columns = %#v, want usage only", columns)
	}
}

func TestProjectedColumnStreamReturnsSourceWithoutProjection(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{{
		FieldName:  "usage",
		Timestamps: []int64{1},
		Values:     []model.FieldValue{model.Float64Value(0.8)},
	}})
	stream := NewProjectedColumnStream(source, nil)
	if !stream.Next() {
		t.Fatalf("Next() = false, err = %v", stream.Err())
	}
	if got := stream.Column().FieldName; got != "usage" {
		t.Fatalf("field = %q, want usage", got)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestProjectedColumnStreamHandlesNilSourceAndNoMatch(t *testing.T) {
	stream := NewProjectedColumnStream(nil, []string{"usage"})
	if stream.Next() || stream.Column().FieldName != "" || stream.Err() != nil {
		t.Fatal("nil projected stream returned data or error")
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("nil projected Close() error = %v", err)
	}

	stream = NewProjectedColumnStream(NewSliceColumnSeriesStream([]model.ColumnSeries{{
		FieldName: "temp",
	}}), []string{"usage"})
	if stream.Next() {
		t.Fatal("projected no-match Next() = true, want false")
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("projected no-match Err() = %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("projected no-match Close() error = %v", err)
	}
}

func collectColumnSeriesStream(t *testing.T, stream ColumnStream) []model.ColumnSeries {
	t.Helper()
	defer func() {
		if err := stream.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	columns := make([]model.ColumnSeries, 0)
	for stream.Next() {
		columns = append(columns, stream.Column())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Err() = %v", err)
	}
	return columns
}

type errorOperator struct {
	id  string
	err error
}

func (o *errorOperator) ID() string {
	return o.id
}

func (o *errorOperator) Next(context.Context) (Record, bool) {
	return Record{}, false
}

func (o *errorOperator) Err() error {
	return o.err
}

func (o *errorOperator) Close() error {
	return nil
}
