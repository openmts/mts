package engine

import (
	"errors"

	"codeberg.org/mts/mts/internal/memtable"
	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/queryexec"
	"codeberg.org/mts/mts/internal/sstable"
)

type shardColumnDataStream struct {
	inner      queryexec.ColumnDataStream
	tombstones []model.Tombstone
	current    model.ColumnData
	err        error
	closed     bool
	unlock     func()
}

func (s *Shard) ScanColumns(query memtable.Query) (queryexec.ColumnDataStream, error) {
	s.lifecycleMu.RLock()
	streams, err := s.openColumnStreamsLocked(query)
	if err != nil {
		s.lifecycleMu.RUnlock()
		return nil, err
	}
	return &shardColumnDataStream{
		inner:      queryexec.WithContextColumnDataStream(query.Context, queryexec.MergeColumnDataStreams(streams...)),
		tombstones: s.tombstones,
		unlock:     s.lifecycleMu.RUnlock,
	}, nil
}

func (s *Shard) openColumnStreamsLocked(query memtable.Query) ([]queryexec.ColumnDataStream, error) {
	if query.Budget.MaxParts > 0 && len(s.parts) > query.Budget.MaxParts {
		return nil, queryexec.NewReadBudgetError("parts", len(s.parts), query.Budget.MaxParts)
	}
	streams := []queryexec.ColumnDataStream{s.mem.ScanColumns(query)}
	partQuery := sstable.Query{
		Context:   query.Context,
		Budget:    query.Budget,
		SeriesIDs: query.SeriesIDs,
		FieldIDs:  query.FieldIDs,
		Start:     query.Start,
		End:       query.End,
	}
	for _, part := range s.parts {
		stream, err := part.ScanColumns(partQuery)
		if err != nil {
			return nil, errors.Join(err, closeColumnDataStreams(streams))
		}
		streams = append(streams, stream)
	}
	return streams, nil
}

func (s *shardColumnDataStream) Next() bool {
	if s.closed || s.err != nil {
		return false
	}
	for s.inner.Next() {
		column := s.inner.ColumnData()
		column.Samples = filterTombstonedSamples(column, s.tombstones)
		if len(column.Samples) == 0 {
			continue
		}
		s.current = column
		return true
	}
	s.err = s.inner.Err()
	closeErr := s.Close()
	if s.err == nil {
		s.err = closeErr
	}
	return false
}

func (s *shardColumnDataStream) ColumnData() model.ColumnData {
	return s.current
}

func (s *shardColumnDataStream) Err() error {
	return s.err
}

func (s *shardColumnDataStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	closeErr := s.inner.Close()
	if s.unlock != nil {
		s.unlock()
		s.unlock = nil
	}
	return closeErr
}

func collectColumnDataStream(stream queryexec.ColumnDataStream) ([]model.ColumnData, error) {
	columns := make([]model.ColumnData, 0)
	for stream.Next() {
		columns = append(columns, stream.ColumnData())
	}
	err := stream.Err()
	closeErr := stream.Close()
	return columns, errors.Join(err, closeErr)
}

func closeColumnDataStreams(streams []queryexec.ColumnDataStream) error {
	var err error
	for _, stream := range streams {
		err = errors.Join(err, stream.Close())
	}
	return err
}
