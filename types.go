package mts

import (
	"context"

	storageengine "codeberg.org/mts/mts/internal/engine"
	"codeberg.org/mts/mts/internal/model"
)

type FieldType = model.FieldType

const (
	FieldFloat64 = model.FieldFloat64
	FieldInt64   = model.FieldInt64
	FieldString  = model.FieldString
	FieldBool    = model.FieldBool
)

type FieldValue = model.FieldValue

type Point = model.Point

type Query = model.Query

type Options = model.Options

type WALOptions = model.WALOptions

type CompactionOptions = model.CompactionOptions

type WriteOptions = model.WriteOptions

type ColumnSeries = model.ColumnSeries

type Row = model.Row

func Float64Value(value float64) FieldValue {
	return model.Float64Value(value)
}

func Int64Value(value int64) FieldValue {
	return model.Int64Value(value)
}

func StringValue(value string) FieldValue {
	return model.StringValue(value)
}

func BoolValue(value bool) FieldValue {
	return model.BoolValue(value)
}

type Engine struct {
	inner *storageengine.Engine
}

func Open(ctx context.Context, opts Options) (*Engine, error) {
	inner, err := storageengine.Open(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Engine{
		inner: inner,
	}, nil
}
