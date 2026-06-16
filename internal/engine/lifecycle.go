package engine

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"time"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/sstable"
	"codeberg.org/mts/mts/internal/storagefs"
)

func (e *Engine) Compact(_ context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, shard := range e.shards {
		if err := shard.Compact(); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) ApplyRetention(_ context.Context, now time.Time) error {
	if e.opts.Retention <= 0 {
		return nil
	}
	cutoff := now.UnixNano() - int64(e.opts.Retention)
	e.mu.Lock()
	defer e.mu.Unlock()
	for id, shard := range e.shards {
		if shard.opts.End >= cutoff {
			continue
		}
		shard.lifecycleMu.Lock()
		if err := shard.closeLocked(); err != nil {
			shard.lifecycleMu.Unlock()
			return err
		}
		if err := storagefs.RemoveAll(shard.opts.Dir); err != nil {
			shard.lifecycleMu.Unlock()
			return err
		}
		shard.lifecycleMu.Unlock()
		delete(e.shards, id)
	}
	return nil
}

func (s *Shard) Compact() error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if err := s.flushLocked(); err != nil {
		return err
	}
	if len(s.parts) <= 1 {
		return nil
	}
	return s.compactPartsLocked(s.manifest.Parts, 1)
}

func (s *Shard) maybeCompactLocked() error {
	if !s.opts.Compaction.Enabled {
		return nil
	}
	candidates, err := s.level0CompactionCandidates()
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}
	return s.compactPartsLocked(candidates, 1)
}

func (s *Shard) compactPartsLocked(candidates []sstable.PartMeta, outputLevel int) error {
	columns, err := s.queryPartCandidates(candidates)
	if err != nil {
		return err
	}
	if len(columns) == 0 {
		return nil
	}
	meta, err := sstable.WritePart(s.opts.Dir, outputLevel, s.nextPartID(), columns)
	if err != nil {
		return err
	}
	part, err := sstable.OpenPart(meta.Path)
	if err != nil {
		return err
	}
	keptParts, keptMeta := s.keepUnselectedParts(candidates)
	nextManifest := sstable.Manifest{Parts: append(keptMeta, meta)}
	if err := sstable.WriteManifest(s.opts.Dir, nextManifest); err != nil {
		closeErr := part.Close()
		return errors.Join(err, closeErr)
	}
	oldParts := s.parts
	s.parts = append(keptParts, part)
	s.manifest = nextManifest
	if err := closeSelectedParts(oldParts, candidates); err != nil {
		return err
	}
	return removeOldParts(candidates, meta.ID)
}

func (s *Shard) level0CompactionCandidates() ([]sstable.PartMeta, error) {
	candidates := make([]sstable.PartMeta, 0)
	var size int64
	for _, part := range s.manifest.Parts {
		if part.Level != 0 {
			continue
		}
		candidates = append(candidates, part)
		if s.opts.Compaction.Level0SizeLimit > 0 {
			partBytes, err := directorySize(part.Path)
			if err != nil {
				return nil, err
			}
			size += partBytes
		}
	}
	limit := s.opts.Compaction.Level0PartLimit
	if limit <= 0 {
		limit = 4
	}
	if len(candidates) > limit {
		return candidates, nil
	}
	if s.opts.Compaction.Level0SizeLimit > 0 && size > s.opts.Compaction.Level0SizeLimit {
		return candidates, nil
	}
	return nil, nil
}

func (s *Shard) queryPartCandidates(candidates []sstable.PartMeta) ([]model.ColumnData, error) {
	selected := partIDSet(candidates)
	columns := make([]model.ColumnData, 0)
	for _, part := range s.parts {
		if _, ok := selected[part.Meta().ID]; !ok {
			continue
		}
		got, err := part.Query(sstable.Query{Start: s.opts.Start, End: s.opts.End})
		if err != nil {
			return nil, err
		}
		columns = append(columns, got...)
	}
	return mergeColumnData(columns), nil
}

func (s *Shard) keepUnselectedParts(candidates []sstable.PartMeta) ([]*sstable.Part, []sstable.PartMeta) {
	selected := partIDSet(candidates)
	keptParts := make([]*sstable.Part, 0, len(s.parts))
	keptMeta := make([]sstable.PartMeta, 0, len(s.manifest.Parts))
	for _, part := range s.parts {
		if _, ok := selected[part.Meta().ID]; !ok {
			keptParts = append(keptParts, part)
		}
	}
	for _, meta := range s.manifest.Parts {
		if _, ok := selected[meta.ID]; !ok {
			keptMeta = append(keptMeta, meta)
		}
	}
	return keptParts, keptMeta
}

func closeSelectedParts(parts []*sstable.Part, selectedMeta []sstable.PartMeta) error {
	selected := partIDSet(selectedMeta)
	var err error
	for _, part := range parts {
		if _, ok := selected[part.Meta().ID]; ok {
			err = errors.Join(err, part.Close())
		}
	}
	return err
}

func partIDSet(parts []sstable.PartMeta) map[string]struct{} {
	out := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		out[part.ID] = struct{}{}
	}
	return out
}

func directorySize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}

func removeOldParts(parts []sstable.PartMeta, keepID string) error {
	for _, part := range parts {
		if part.ID == keepID {
			continue
		}
		if err := storagefs.RemoveAll(part.Path); err != nil {
			return err
		}
	}
	return nil
}
