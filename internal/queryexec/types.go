package queryexec

import (
	"context"

	"github.com/openmts/mts/internal/model"
)

type ColumnStream interface {
	Next() bool
	Column() model.ColumnSeries
	Err() error
	Close() error
}

type ColumnDataStream interface {
	Next() bool
	ColumnData() model.ColumnData
	Err() error
	Close() error
}

type RowStream interface {
	Next() bool
	Row() model.Row
	Err() error
	Close() error
}

type ColumnReader interface {
	QueryColumnStream(ctx context.Context, query model.Query) (ColumnStream, error)
}

type RowReader interface {
	QueryRowStream(ctx context.Context, query model.Query) (RowStream, error)
}

type ColumnDecorator func(column model.ColumnData) (model.ColumnSeries, bool)
