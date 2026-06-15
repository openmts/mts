package memtable

import (
	"sort"
	"sync"

	"codeberg.org/mts/mts/internal/model"
)

type Query struct {
	SeriesIDs map[uint64]struct{}
	FieldIDs  map[uint32]struct{}
	Start     int64
	End       int64
}

type MemTable struct {
	mu          sync.RWMutex
	data        tableData
	sampleCount int
}

type Snapshot struct {
	data        tableData
	sampleCount int
}

type sampleEntry struct {
	fieldType model.FieldType
	sample    model.VersionedSample
}

type tableData map[uint64]map[uint32]map[int64]sampleEntry

func New() *MemTable {
	return &MemTable{
		data: make(tableData),
	}
}

func (m *MemTable) Apply(point model.ResolvedPoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, field := range point.Fields {
		if m.applyField(point, field) {
			m.sampleCount++
		}
	}
	return nil
}

func (m *MemTable) SampleCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sampleCount
}

func (m *MemTable) SnapshotAndReset() *Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := &Snapshot{
		data:        cloneData(m.data),
		sampleCount: m.sampleCount,
	}
	m.data = make(tableData)
	m.sampleCount = 0
	return snapshot
}

func (m *MemTable) Snapshot() *Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &Snapshot{
		data:        cloneData(m.data),
		sampleCount: m.sampleCount,
	}
}

func (m *MemTable) Query(query Query) []model.ColumnData {
	return m.Snapshot().Query(query)
}

func (s *Snapshot) Query(query Query) []model.ColumnData {
	if s == nil || query.End < query.Start {
		return []model.ColumnData{}
	}
	columns := make([]model.ColumnData, 0)
	for seriesID, fields := range s.data {
		if !containsSeries(query.SeriesIDs, seriesID) {
			continue
		}
		columns = append(columns, queryFields(seriesID, fields, query)...)
	}
	sortColumns(columns)
	return columns
}

func (s *Snapshot) SampleCount() int {
	if s == nil {
		return 0
	}
	return s.sampleCount
}

func (m *MemTable) applyField(point model.ResolvedPoint, field model.ResolvedField) bool {
	fields, ok := m.data[point.SeriesID]
	if !ok {
		fields = make(map[uint32]map[int64]sampleEntry)
		m.data[point.SeriesID] = fields
	}
	samples, ok := fields[field.FieldID]
	if !ok {
		samples = make(map[int64]sampleEntry)
		fields[field.FieldID] = samples
	}
	current, exists := samples[point.Timestamp]
	if exists && current.sample.WriteSeq >= point.WriteSeq {
		return false
	}
	samples[point.Timestamp] = sampleEntry{
		fieldType: field.Type,
		sample: model.VersionedSample{
			Timestamp: point.Timestamp,
			WriteSeq:  point.WriteSeq,
			Value:     field.Value,
		},
	}
	return !exists
}

func queryFields(
	seriesID uint64,
	fields map[uint32]map[int64]sampleEntry,
	query Query,
) []model.ColumnData {
	columns := make([]model.ColumnData, 0, len(fields))
	for fieldID, samples := range fields {
		if !containsField(query.FieldIDs, fieldID) {
			continue
		}
		column := querySamples(seriesID, fieldID, samples, query)
		if len(column.Samples) > 0 {
			columns = append(columns, column)
		}
	}
	return columns
}

func querySamples(
	seriesID uint64,
	fieldID uint32,
	samples map[int64]sampleEntry,
	query Query,
) model.ColumnData {
	column := model.ColumnData{
		SeriesID: seriesID,
		FieldID:  fieldID,
		Samples:  make([]model.VersionedSample, 0),
	}
	for timestamp, entry := range samples {
		if timestamp < query.Start || timestamp > query.End {
			continue
		}
		column.FieldType = entry.fieldType
		column.Samples = append(column.Samples, entry.sample)
	}
	sort.Slice(column.Samples, func(i, j int) bool {
		return column.Samples[i].Timestamp < column.Samples[j].Timestamp
	})
	return column
}

func sortColumns(columns []model.ColumnData) {
	sort.Slice(columns, func(i, j int) bool {
		if columns[i].SeriesID != columns[j].SeriesID {
			return columns[i].SeriesID < columns[j].SeriesID
		}
		return columns[i].FieldID < columns[j].FieldID
	})
}

func containsSeries(filter map[uint64]struct{}, seriesID uint64) bool {
	if len(filter) == 0 {
		return true
	}
	_, ok := filter[seriesID]
	return ok
}

func containsField(filter map[uint32]struct{}, fieldID uint32) bool {
	if len(filter) == 0 {
		return true
	}
	_, ok := filter[fieldID]
	return ok
}

func cloneData(src tableData) tableData {
	dst := make(tableData, len(src))
	for seriesID, fields := range src {
		dst[seriesID] = cloneFields(fields)
	}
	return dst
}

func cloneFields(src map[uint32]map[int64]sampleEntry) map[uint32]map[int64]sampleEntry {
	dst := make(map[uint32]map[int64]sampleEntry, len(src))
	for fieldID, samples := range src {
		dst[fieldID] = cloneSamples(samples)
	}
	return dst
}

func cloneSamples(src map[int64]sampleEntry) map[int64]sampleEntry {
	dst := make(map[int64]sampleEntry, len(src))
	for timestamp, sample := range src {
		dst[timestamp] = sample
	}
	return dst
}
