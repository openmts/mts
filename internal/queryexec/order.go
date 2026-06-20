package queryexec

import (
	"container/heap"
	"sort"

	"github.com/openmts/mts/internal/model"
)

type orderedColumnStream struct {
	source ColumnStream
	order  model.QueryOrder
}

type orderedRowStream struct {
	source      RowStream
	order       model.QueryOrder
	rows        []model.Row
	limit       int
	offset      int
	index       int
	loaded      bool
	err         error
	closed      bool
	sourceClose bool
}

func NewOrderedColumnStream(source ColumnStream, order model.QueryOrder) ColumnStream {
	if order.By != model.QueryOrderByTime || order.Direction != model.QuerySortDesc {
		return source
	}
	return &orderedColumnStream{source: source, order: order}
}

func NewOrderedRowStream(source RowStream, order model.QueryOrder, pagination ...int) RowStream {
	if order.By != model.QueryOrderByTime || order.Direction != model.QuerySortDesc {
		return source
	}
	stream := &orderedRowStream{source: source, order: order}
	if len(pagination) > 0 {
		stream.limit = pagination[0]
	}
	if len(pagination) > 1 {
		stream.offset = pagination[1]
	}
	return stream
}

func (s *orderedColumnStream) Next() bool {
	if s.source == nil || !s.source.Next() {
		return false
	}
	return true
}

func (s *orderedColumnStream) Column() model.ColumnSeries {
	if s.source == nil {
		return model.ColumnSeries{}
	}
	return reverseColumnSeriesByTime(s.source.Column())
}

func (s *orderedColumnStream) Err() error {
	if s.source == nil {
		return nil
	}
	return s.source.Err()
}

func (s *orderedColumnStream) Close() error {
	if s.source == nil {
		return nil
	}
	return s.source.Close()
}

func (s *orderedRowStream) Next() bool {
	if s.closed || s.err != nil {
		return false
	}
	if !s.loaded {
		s.load()
	}
	if s.err != nil || s.index >= len(s.rows) {
		return false
	}
	s.index++
	return true
}

func (s *orderedRowStream) Row() model.Row {
	if s.index == 0 || s.index > len(s.rows) {
		return model.Row{}
	}
	return s.rows[s.index-1]
}

func (s *orderedRowStream) Err() error {
	return s.err
}

func (s *orderedRowStream) Close() error {
	s.closed = true
	s.rows = nil
	return s.closeSource()
}

func (s *orderedRowStream) load() {
	s.loaded = true
	if s.limit > 0 {
		s.loadBounded()
		return
	}
	for s.source != nil && s.source.Next() {
		s.rows = append(s.rows, s.source.Row())
	}
	if s.source != nil {
		s.err = s.source.Err()
	}
	if s.err != nil {
		return
	}
	sort.SliceStable(s.rows, func(i int, j int) bool {
		return rowAfterTimeDesc(s.rows[i], s.rows[j])
	})
	s.err = s.closeSource()
}

func (s *orderedRowStream) loadBounded() {
	capacity := s.limit + s.offset
	if capacity <= 0 {
		s.load()
		return
	}
	rows := make(rowMinHeap, 0, capacity)
	heap.Init(&rows)
	for s.source != nil && s.source.Next() {
		row := s.source.Row()
		if rows.Len() < capacity {
			heap.Push(&rows, row)
			continue
		}
		if rowAfterTimeDesc(row, rows[0]) {
			rows[0] = row
			heap.Fix(&rows, 0)
		}
	}
	if s.source != nil {
		s.err = s.source.Err()
	}
	if s.err != nil {
		return
	}
	s.rows = []model.Row(rows)
	sort.SliceStable(s.rows, func(i int, j int) bool {
		return rowAfterTimeDesc(s.rows[i], s.rows[j])
	})
	s.err = s.closeSource()
}

func (s *orderedRowStream) closeSource() error {
	if s.sourceClose || s.source == nil {
		return nil
	}
	s.sourceClose = true
	return s.source.Close()
}

type rowMinHeap []model.Row

func (h rowMinHeap) Len() int {
	return len(h)
}

func (h rowMinHeap) Less(i int, j int) bool {
	return rowBeforeTimeDesc(h[i], h[j])
}

func (h rowMinHeap) Swap(i int, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *rowMinHeap) Push(value any) {
	*h = append(*h, value.(model.Row))
}

func (h *rowMinHeap) Pop() any {
	old := *h
	last := old[len(old)-1]
	*h = old[:len(old)-1]
	return last
}

func reverseColumnSeriesByTime(column model.ColumnSeries) model.ColumnSeries {
	for left, right := 0, len(column.Timestamps)-1; left < right; left, right = left+1, right-1 {
		column.Timestamps[left], column.Timestamps[right] = column.Timestamps[right], column.Timestamps[left]
		column.Values[left], column.Values[right] = column.Values[right], column.Values[left]
	}
	return column
}

func rowAfterTimeDesc(left model.Row, right model.Row) bool {
	if left.Timestamp != right.Timestamp {
		return left.Timestamp > right.Timestamp
	}
	return left.SeriesID < right.SeriesID
}

func rowBeforeTimeDesc(left model.Row, right model.Row) bool {
	if left.Timestamp != right.Timestamp {
		return left.Timestamp < right.Timestamp
	}
	return left.SeriesID > right.SeriesID
}
