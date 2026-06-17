package catalog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/storagefs"
)

var (
	ErrEmptyMeasurement  = errors.New("measurement is empty")
	ErrEmptyFields       = errors.New("fields are empty")
	ErrFieldTypeConflict = errors.New("field type conflict")
)

type Catalog struct {
	mu sync.RWMutex

	dir string
	wal *os.File

	nextSeriesID uint64
	nextFieldID  uint32
	seriesByKey  map[string]uint64
	seriesByTag  map[string]map[string]map[string]uint64
	series       map[uint64]Series
	fieldsByKey  map[string]uint32
	fields       map[uint32]Field
	fieldSchemas map[string][]Field
	databases    map[string]struct{}
	policies     map[string]map[string]model.RetentionPolicy

	snapshotDirtyRecords int
	seriesKeyScratch     []string
}

type Snapshot struct {
	Series map[uint64]Series
	Fields map[uint32]Field
}

func Open(dir string) (*Catalog, error) {
	if err := storagefs.MkdirAll(dir); err != nil {
		return nil, err
	}
	cat := newCatalog(dir)
	if err := cat.loadSnapshot(); err != nil {
		return nil, err
	}
	if err := cat.replayWAL(); err != nil {
		return nil, err
	}
	if err := cat.loadMetadata(); err != nil {
		return nil, err
	}
	wal, err := storagefs.OpenFile(cat.walPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, storagefs.FileMode)
	if err != nil {
		return nil, fmt.Errorf("open catalog wal: %w", err)
	}
	cat.wal = wal
	return cat, nil
}

func (c *Catalog) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.wal == nil {
		return nil
	}
	if err := c.checkpointSnapshotLocked(true); err != nil {
		return err
	}
	if err := c.wal.Close(); err != nil {
		return fmt.Errorf("close catalog wal: %w", err)
	}
	c.wal = nil
	return nil
}

func (c *Catalog) ResolvePoint(point model.Point) (model.ResolvedPoint, error) {
	if err := validatePoint(point); err != nil {
		return model.ResolvedPoint{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.resolvePointLocked(point)
}

func (c *Catalog) ResolvePoints(points []model.Point) ([]model.ResolvedPoint, error) {
	totalFields := 0
	for _, point := range points {
		if err := validatePoint(point); err != nil {
			return nil, err
		}
		totalFields += len(point.Fields)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	arena := resolvedFieldArena{
		fields: make([]model.ResolvedField, totalFields),
	}
	cache := &resolveBatchCache{}
	resolved := make([]model.ResolvedPoint, len(points))
	changed := false
	metadataChanged := false
	for index, point := range points {
		metadataChanged = c.ensureMetadataLocked(point.Database, point.RetentionPolicy) || metadataChanged
		// ResolvePoints feeds the synchronous engine write path, so the resolved
		// point may borrow input tags instead of cloning them per sample.
		got, pointChanged, err := c.resolvePointNoSnapshotCachedLocked(point, false, &arena, cache)
		if err != nil {
			return nil, err
		}
		changed = changed || pointChanged
		resolved[index] = got
	}
	if changed {
		if err := c.checkpointSnapshotLocked(false); err != nil {
			return nil, err
		}
	}
	if metadataChanged {
		if err := c.saveMetadataLocked(); err != nil {
			return nil, err
		}
	}
	return resolved, nil
}

func (c *Catalog) resolvePointLocked(point model.Point) (model.ResolvedPoint, error) {
	metadataChanged := c.ensureMetadataLocked(point.Database, point.RetentionPolicy)
	series, err := c.resolveSeriesLocked(point.Measurement, point.Tags)
	if err != nil {
		return model.ResolvedPoint{}, err
	}
	fields, err := c.resolveFieldsLocked(point.Measurement, point.Fields)
	if err != nil {
		return model.ResolvedPoint{}, err
	}
	if metadataChanged {
		if err := c.saveMetadataLocked(); err != nil {
			return model.ResolvedPoint{}, err
		}
	}
	return resolvedPointFrom(point, series.ID, fields, true), nil
}

func (c *Catalog) ensureMetadataLocked(database string, policy string) bool {
	if strings.TrimSpace(database) == "" {
		return false
	}
	changed := false
	if _, ok := c.databases[database]; !ok {
		c.databases[database] = struct{}{}
		changed = true
	}
	if strings.TrimSpace(policy) == "" {
		return changed
	}
	if c.policies[database] == nil {
		c.policies[database] = make(map[string]model.RetentionPolicy, 1)
	}
	if _, ok := c.policies[database][policy]; !ok {
		c.policies[database][policy] = model.RetentionPolicy{Name: policy}
		changed = true
	}
	return changed
}

func (c *Catalog) resolvePointNoSnapshotCachedLocked(
	point model.Point,
	cloneResultTags bool,
	arena *resolvedFieldArena,
	cache *resolveBatchCache,
) (model.ResolvedPoint, bool, error) {
	series, seriesChanged, err := c.resolveSeriesNoSnapshotCachedLocked(point.Measurement, point.Tags, cache)
	if err != nil {
		return model.ResolvedPoint{}, false, err
	}
	fields, fieldsChanged, err := c.resolveFieldsNoSnapshotLocked(point.Measurement, point.Fields, arena)
	if err != nil {
		return model.ResolvedPoint{}, false, err
	}
	return resolvedPointFrom(point, series.ID, fields, cloneResultTags), seriesChanged || fieldsChanged, nil
}

func resolvedPointFrom(
	point model.Point,
	seriesID uint64,
	fields []model.ResolvedField,
	cloneResultTags bool,
) model.ResolvedPoint {
	tags := point.Tags
	if cloneResultTags {
		tags = cloneTags(tags)
	}
	return model.ResolvedPoint{
		Database:        point.Database,
		RetentionPolicy: point.RetentionPolicy,
		Measurement:     point.Measurement,
		Tags:            tags,
		SeriesID:        seriesID,
		Timestamp:       point.Timestamp,
		Fields:          fields,
	}
}

func (c *Catalog) MatchSeries(measurement string, tags map[string]string) []uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	matches := make([]uint64, 0)
	for id, series := range c.series {
		if series.Measurement != measurement {
			continue
		}
		if tagsMatch(series.Tags, tags) {
			matches = append(matches, id)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i] < matches[j]
	})
	return matches
}

func (c *Catalog) Series(id uint64) (Series, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	series, ok := c.series[id]
	if !ok {
		return Series{}, false
	}
	series.Tags = cloneTags(series.Tags)
	return series, true
}

func (c *Catalog) Field(id uint32) (Field, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	field, ok := c.fields[id]
	return field, ok
}

func (c *Catalog) FieldIDs(measurement string, names []string) map[uint32]struct{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ids := make(map[uint32]struct{})
	if len(names) == 0 {
		for _, field := range c.fields {
			if field.Measurement == measurement {
				ids[field.ID] = struct{}{}
			}
		}
		return ids
	}
	for _, name := range names {
		if id, ok := c.fieldsByKey[fieldKey(measurement, name)]; ok {
			ids[id] = struct{}{}
		}
	}
	return ids
}

func (c *Catalog) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Snapshot{
		Series: cloneSeriesMap(c.series),
		Fields: cloneFieldMap(c.fields),
	}
}

func cloneSeriesMap(series map[uint64]Series) map[uint64]Series {
	out := make(map[uint64]Series, len(series))
	for id, item := range series {
		item.Tags = cloneTags(item.Tags)
		out[id] = item
	}
	return out
}

func cloneFieldMap(fields map[uint32]Field) map[uint32]Field {
	out := make(map[uint32]Field, len(fields))
	for id, item := range fields {
		out[id] = item
	}
	return out
}

func newCatalog(dir string) *Catalog {
	return &Catalog{
		dir:          filepath.Clean(dir),
		nextSeriesID: 1,
		nextFieldID:  1,
		seriesByKey:  make(map[string]uint64),
		seriesByTag:  make(map[string]map[string]map[string]uint64),
		series:       make(map[uint64]Series),
		fieldsByKey:  make(map[string]uint32),
		fields:       make(map[uint32]Field),
		fieldSchemas: make(map[string][]Field),
		databases:    make(map[string]struct{}),
		policies:     make(map[string]map[string]model.RetentionPolicy),
	}
}

func validatePoint(point model.Point) error {
	if strings.TrimSpace(point.Measurement) == "" {
		return ErrEmptyMeasurement
	}
	if len(point.Fields) == 0 {
		return ErrEmptyFields
	}
	return nil
}
