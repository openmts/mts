package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	mts "github.com/openmts/mts"
)

func verifyPublicWorkflow(ctx context.Context, eng *mts.Engine) error {
	if err := verifyMetadata(ctx, eng); err != nil {
		return err
	}
	if err := verifyUserManagement(ctx, eng); err != nil {
		return err
	}
	if err := verifyRowIterator(ctx, eng); err != nil {
		return err
	}
	if err := verifyColumnIterator(ctx, eng); err != nil {
		return err
	}
	if err := verifyPrecisionQuery(ctx, eng); err != nil {
		return err
	}
	return verifyExplainAndHealth(ctx, eng)
}

func verifyMetadata(ctx context.Context, eng *mts.Engine) error {
	measurements, err := eng.ListMeasurements(ctx, databaseName)
	if err != nil {
		return fmt.Errorf("list measurements: %w", err)
	}
	if !slices.Contains(measurements, measurementName) {
		return fmt.Errorf("measurements = %v, want %q", measurements, measurementName)
	}
	fields, err := eng.ListFields(ctx, databaseName, measurementName)
	if err != nil {
		return fmt.Errorf("list fields: %w", err)
	}
	if err := assertFieldSchemas(fields); err != nil {
		return err
	}
	series, err := eng.ListSeries(ctx, databaseName, measurementName, map[string]string{"region": "east"})
	if err != nil {
		return fmt.Errorf("list series: %w", err)
	}
	if len(series) != 2 {
		return fmt.Errorf("east series count = %d, want 2", len(series))
	}
	return nil
}

func assertFieldSchemas(fields []mts.FieldSchema) error {
	want := map[string]mts.FieldType{
		"active": mts.FieldBool,
		"cores":  mts.FieldInt64,
		"state":  mts.FieldString,
		"usage":  mts.FieldFloat64,
	}
	got := make(map[string]mts.FieldType, len(fields))
	for _, field := range fields {
		got[field.Name] = field.Type
	}
	for name, fieldType := range want {
		if got[name] != fieldType {
			return fmt.Errorf("field %s type = %v, want %v", name, got[name], fieldType)
		}
	}
	return nil
}

func verifyUserManagement(ctx context.Context, eng *mts.Engine) error {
	user, ok, err := eng.GetUser(ctx, "alice")
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if !ok || user.DisplayName != "Alice" || user.Metadata["team"] != "platform" {
		return fmt.Errorf("user = %#v ok=%v, want persisted alice", user, ok)
	}
	users, err := eng.ListUsers(ctx)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}
	if len(users) != 1 || users[0].Name != "alice" {
		return fmt.Errorf("users = %#v, want alice", users)
	}
	grants, err := eng.ListDatabasePermissions(ctx, "alice")
	if err != nil {
		return fmt.Errorf("list database permissions: %w", err)
	}
	if !slices.Equal(grants, []mts.DatabaseGrant{{
		Database:   databaseName,
		Permission: mts.DatabasePermissionAdmin,
	}}) {
		return fmt.Errorf("database grants = %#v, want admin", grants)
	}
	if err := eng.CheckUserDatabasePermission(
		ctx,
		"alice",
		databaseName,
		mts.DatabasePermissionWrite,
	); err != nil {
		return fmt.Errorf("check admin implied write: %w", err)
	}
	if err := eng.CheckUserDatabasePermission(
		ctx,
		"alice",
		"other",
		mts.DatabasePermissionRead,
	); !errors.Is(err, mts.ErrPermissionDenied) {
		return fmt.Errorf("check other database error = %v, want permission denied", err)
	}
	return nil
}

func verifyRowIterator(ctx context.Context, eng *mts.Engine) error {
	query, err := mts.NewQuery().
		Select("usage", "state", "cores", "active").
		From(databaseName, retentionName, measurementName).
		WhereExpr(mts.AndExpr(
			mts.PredicateQueryExpr(mts.TagIn("host", "api-1", "api-2")),
			mts.PredicateQueryExpr(mts.FieldGTE("usage", mts.Float64Value(0.50))),
		)).
		TimeRangeTime(time.Unix(0, 0), time.Unix(0, int64(6*time.Second))).
		OrderByTimeDesc().
		Limit(2).
		Build()
	if err != nil {
		return fmt.Errorf("build row query: %w", err)
	}
	iter, err := eng.QueryRowIterator(ctx, query)
	if err != nil {
		return fmt.Errorf("query row iterator: %w", err)
	}
	return assertRows(iter)
}

func assertRows(iter mts.RowIterator) (err error) {
	defer func() {
		err = errors.Join(err, iter.Close())
	}()
	wantTimestamps := []int64{int64(5 * time.Second), int64(3 * time.Second)}
	index := 0
	for iter.Next() {
		row := iter.Row()
		if index >= len(wantTimestamps) {
			return fmt.Errorf("too many rows")
		}
		if row.Timestamp != wantTimestamps[index] {
			return fmt.Errorf("row[%d] timestamp = %d, want %d", index, row.Timestamp, wantTimestamps[index])
		}
		if index == 0 && row.Fields["state"].String != "warm" {
			return fmt.Errorf("row[0] state = %q, want warm", row.Fields["state"].String)
		}
		if index == 1 && row.Fields["state"].String != "hot" {
			return fmt.Errorf("row[1] state = %q, want hot", row.Fields["state"].String)
		}
		index++
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("row iterator error: %w", err)
	}
	if index != len(wantTimestamps) {
		return fmt.Errorf("row count = %d, want %d", index, len(wantTimestamps))
	}
	return nil
}

func verifyColumnIterator(ctx context.Context, eng *mts.Engine) error {
	query, err := mts.NewQuery().
		Select("usage").
		From(databaseName, retentionName, measurementName).
		Where(mts.TagEq("host", "api-1")).
		TimeRange(0, int64(6*time.Second)).
		Build()
	if err != nil {
		return fmt.Errorf("build column query: %w", err)
	}
	iter, err := eng.QueryColumnIterator(ctx, query)
	if err != nil {
		return fmt.Errorf("query column iterator: %w", err)
	}
	return assertUsageColumn(iter)
}

func assertUsageColumn(iter mts.ColumnIterator) (err error) {
	defer func() {
		err = errors.Join(err, iter.Close())
	}()
	values := make([]float64, 0, 2)
	for iter.Next() {
		column := iter.Column()
		if column.FieldName != "usage" {
			return fmt.Errorf("column field = %q, want usage", column.FieldName)
		}
		for _, value := range column.Values {
			values = append(values, value.Float64)
		}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("column iterator error: %w", err)
	}
	if !slices.Equal(values, []float64{0.40, 0.90}) {
		return fmt.Errorf("usage values = %v, want [0.4 0.9]", values)
	}
	return nil
}

func verifyPrecisionQuery(ctx context.Context, eng *mts.Engine) error {
	query, err := mts.NewQuery().
		Select("usage").
		From(databaseName, retentionName, measurementName).
		Where(mts.TagEq("host", "api-2")).
		Precision(mts.PrecisionMillisecond).
		TimeRange(5_000, 5_001).
		Build()
	if err != nil {
		return fmt.Errorf("build precision query: %w", err)
	}
	rows, err := eng.QueryRows(ctx, query)
	if err != nil {
		return fmt.Errorf("query precision rows: %w", err)
	}
	if len(rows) != 1 {
		return fmt.Errorf("precision row count = %d, want 1", len(rows))
	}
	if rows[0].Timestamp != 5_000 {
		return fmt.Errorf("precision row timestamp = %d, want 5000", rows[0].Timestamp)
	}
	return nil
}

func verifyExplainAndHealth(ctx context.Context, eng *mts.Engine) error {
	query, err := mts.NewQuery().
		From(databaseName, retentionName, measurementName).
		Aggregate(mts.AggregateCount, "usage").
		GroupByTime(time.Second).
		Build()
	if err != nil {
		return fmt.Errorf("build explain query: %w", err)
	}
	result, err := eng.QueryWithExplain(ctx, query)
	if err != nil {
		return fmt.Errorf("query with explain: %w", err)
	}
	if len(result.Columns) == 0 || result.Explain.Measurement != measurementName {
		return fmt.Errorf("explain result = %#v, want columns and measurement", result)
	}
	health := eng.HealthSnapshot()
	if !health.Healthy || !health.Ready {
		return fmt.Errorf("health = %#v, want healthy and ready", health)
	}
	return nil
}
