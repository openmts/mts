package engine

import (
	"context"
	"time"

	"codeberg.org/mts/mts/internal/memtable"
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
		if err := shard.Close(); err != nil {
			return err
		}
		if err := storagefs.RemoveAll(shard.opts.Dir); err != nil {
			return err
		}
		delete(e.shards, id)
	}
	return nil
}

func (s *Shard) Compact() error {
	if err := s.Flush(); err != nil {
		return err
	}
	if len(s.parts) <= 1 {
		return nil
	}
	columns, err := s.Query(memtable.Query{
		Start: s.opts.Start,
		End:   s.opts.End,
	})
	if err != nil {
		return err
	}
	meta, err := sstable.WritePart(s.opts.Dir, 1, s.nextPartID(), columns)
	if err != nil {
		return err
	}
	part, err := sstable.OpenPart(meta.Path)
	if err != nil {
		return err
	}
	oldParts := s.manifest.Parts
	s.parts = []*sstable.Part{part}
	s.manifest = sstable.Manifest{Parts: []sstable.PartMeta{meta}}
	if err := sstable.WriteManifest(s.opts.Dir, s.manifest); err != nil {
		return err
	}
	return removeOldParts(oldParts, meta.ID)
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
