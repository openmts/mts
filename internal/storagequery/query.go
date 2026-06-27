package storagequery

import (
	"context"

	"github.com/openmts/mts/internal/model"
)

// Query 是 MemTable 与 SSTable 共用的列扫描契约。
type Query struct {
	Context         context.Context
	Budget          model.QueryBudget
	Stats           *model.QueryStats
	Boundary        model.QueryBoundaryMode
	SeriesIDs       map[uint64]struct{}
	FieldIDs        map[uint32]struct{}
	FieldPredicates map[uint32][]model.QueryPredicate
	Start           int64
	End             int64
}
