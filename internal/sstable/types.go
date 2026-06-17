package sstable

import (
	"context"
	"os"

	"codeberg.org/mts/mts/internal/model"
)

const (
	metadataFile    = "metadata.bin"
	metaindexFile   = "metaindex.bin"
	indexFile       = "index.bin"
	seriesIndexFile = "series_index.bin"
	timestampsFile  = "timestamps.bin"
	valuesFile      = "values.bin"
	stringsFile     = "strings.bin"
	manifestFile    = "MANIFEST.bin"
)

type Query struct {
	Context   context.Context
	Budget    model.QueryBudget
	Stats     *model.QueryStats
	Boundary  model.QueryBoundaryMode
	SeriesIDs map[uint64]struct{}
	FieldIDs  map[uint32]struct{}
	Start     int64
	End       int64
}

type PartMeta struct {
	ID          string `json:"id"`
	Level       int    `json:"level"`
	MinTime     int64  `json:"min_time"`
	MaxTime     int64  `json:"max_time"`
	MinSeriesID uint64 `json:"min_series_id"`
	MaxSeriesID uint64 `json:"max_series_id"`
	RowsCount   int    `json:"rows_count"`
	SeriesCount int    `json:"series_count"`
	BlockCount  int    `json:"block_count"`
	MaxWriteSeq uint64 `json:"max_write_seq"`
	Path        string `json:"path"`
}

type Manifest struct {
	Sequence uint64     `json:"sequence"`
	Parts    []PartMeta `json:"parts"`
}

type metadata struct {
	Part           PartMeta `json:"part"`
	IndexRef       blockRef `json:"index_ref"`
	MetaIndexRef   blockRef `json:"metaindex_ref"`
	SeriesIndexRef blockRef `json:"series_index_ref"`
	Components     []string `json:"components"`
	CreatedUnix    int64    `json:"created_unix"`
}

type blockRef struct {
	Offset int64 `json:"offset"`
	Size   int64 `json:"size"`
}

type indexRow struct {
	SeriesID uint64      `json:"series_id"`
	MinTime  int64       `json:"min_time"`
	MaxTime  int64       `json:"max_time"`
	TimeRef  blockRef    `json:"time_ref"`
	Columns  []columnRef `json:"columns"`
}

type columnRef struct {
	FieldID   uint32          `json:"field_id"`
	FieldType model.FieldType `json:"field_type"`
	ValueRef  blockRef        `json:"value_ref"`
}

type metaIndexRow struct {
	MinSeriesID uint64   `json:"min_series_id"`
	MaxSeriesID uint64   `json:"max_series_id"`
	MinTime     int64    `json:"min_time"`
	MaxTime     int64    `json:"max_time"`
	FieldIDs    []uint32 `json:"field_ids"`
	IndexRef    blockRef `json:"index_ref"`
}

type seriesIndexRow struct {
	SeriesID uint64
	MinTime  int64
	MaxTime  int64
	FieldIDs []uint32
	IndexRef blockRef
}

type timeBlock struct {
	Encoding   string  `json:"encoding"`
	MinTime    int64   `json:"min_time"`
	MaxTime    int64   `json:"max_time"`
	Timestamps []int64 `json:"timestamps"`
}

type valueBlock struct {
	Encoding  string                  `json:"encoding"`
	FieldID   uint32                  `json:"field_id"`
	FieldType model.FieldType         `json:"field_type"`
	Samples   []model.VersionedSample `json:"samples"`
}

type valuePageRef struct {
	MinTime int64
	MaxTime int64
	Ref     blockRef
}

type valuePageIndex struct {
	FieldID   uint32
	FieldType model.FieldType
	Count     int
	Pages     []valuePageRef
}

type Part struct {
	path       string
	metadata   metadata
	metaRows   []metaIndexRow
	seriesRows []seriesIndexRow
	files      *partReadFiles
	stats      *readStats
}

type partReadFiles struct {
	index      *os.File
	timestamps *os.File
	values     *os.File
}

type readStats struct {
	TimeBlocksRead  int
	ValueBlocksRead int
	ValuePagesRead  int
	IndexRowsRead   int
}
