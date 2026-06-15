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
	series       map[uint64]Series
	fieldsByKey  map[string]uint32
	fields       map[uint32]Field
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
	wal, err := os.OpenFile(cat.walPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, storagefs.FileMode)
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
	for _, point := range points {
		if err := validatePoint(point); err != nil {
			return nil, err
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	resolved := make([]model.ResolvedPoint, 0, len(points))
	changed := false
	for _, point := range points {
		got, pointChanged, err := c.resolvePointNoSnapshotLocked(point)
		if err != nil {
			return nil, err
		}
		changed = changed || pointChanged
		resolved = append(resolved, got)
	}
	if changed {
		if err := c.saveSnapshotLocked(); err != nil {
			return nil, err
		}
	}
	return resolved, nil
}

func (c *Catalog) resolvePointLocked(point model.Point) (model.ResolvedPoint, error) {
	series, err := c.resolveSeriesLocked(point.Measurement, point.Tags)
	if err != nil {
		return model.ResolvedPoint{}, err
	}
	fields, err := c.resolveFieldsLocked(point.Measurement, point.Fields)
	if err != nil {
		return model.ResolvedPoint{}, err
	}
	return model.ResolvedPoint{
		Database:        point.Database,
		RetentionPolicy: point.RetentionPolicy,
		Measurement:     point.Measurement,
		Tags:            cloneTags(point.Tags),
		SeriesID:        series.ID,
		Timestamp:       point.Timestamp,
		Fields:          fields,
	}, nil
}

func (c *Catalog) resolvePointNoSnapshotLocked(
	point model.Point,
) (model.ResolvedPoint, bool, error) {
	series, seriesChanged, err := c.resolveSeriesNoSnapshotLocked(point.Measurement, point.Tags)
	if err != nil {
		return model.ResolvedPoint{}, false, err
	}
	fields, fieldsChanged, err := c.resolveFieldsNoSnapshotLocked(point.Measurement, point.Fields)
	if err != nil {
		return model.ResolvedPoint{}, false, err
	}
	return model.ResolvedPoint{
		Database:        point.Database,
		RetentionPolicy: point.RetentionPolicy,
		Measurement:     point.Measurement,
		Tags:            cloneTags(point.Tags),
		SeriesID:        series.ID,
		Timestamp:       point.Timestamp,
		Fields:          fields,
	}, seriesChanged || fieldsChanged, nil
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

func newCatalog(dir string) *Catalog {
	return &Catalog{
		dir:          filepath.Clean(dir),
		nextSeriesID: 1,
		nextFieldID:  1,
		seriesByKey:  make(map[string]uint64),
		series:       make(map[uint64]Series),
		fieldsByKey:  make(map[string]uint32),
		fields:       make(map[uint32]Field),
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
