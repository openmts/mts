package queryservice

import "github.com/openmts/mts/internal/model"

type Request struct {
	Query model.Query
}

type Result struct {
	Columns []model.ColumnSeries
	Rows    []model.Row
	Stats   model.QueryStats
}
