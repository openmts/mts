package engine

import (
	"sort"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/sstable"
)

type compactionPlan struct {
	level       int
	outputLevel int
	candidates  []sstable.PartMeta
	output      compactionOutputOptions
}

type compactionOutputOptions struct {
	maxOutputPartBytes int64
	compression        model.CompressionOptions
}

func nextCompactionPlan(
	parts []sstable.PartMeta,
	tombstones []model.Tombstone,
	opts model.CompactionOptions,
) (*compactionPlan, error) {
	maxLevel := maxCompactionLevel(parts, opts.Levels)
	for level := 0; level <= maxLevel; level++ {
		candidates := partsAtLevel(parts, level)
		if len(candidates) == 0 {
			continue
		}
		levelOpts := compactionLevelOptions(opts, level)
		triggered, err := compactionTriggered(candidates, tombstones, levelOpts)
		if err != nil {
			return nil, err
		}
		if !triggered {
			continue
		}
		outputLevel := level + 1
		return &compactionPlan{
			level:       level,
			outputLevel: outputLevel,
			candidates:  candidates,
			output:      outputOptionsForLevel(opts, outputLevel),
		}, nil
	}
	return nil, nil
}

func fullCompactionPlan(
	parts []sstable.PartMeta,
	tombstones []model.Tombstone,
	opts model.CompactionOptions,
) *compactionPlan {
	if len(parts) == 0 || (len(parts) == 1 && len(tombstones) == 0) {
		return nil
	}
	outputLevel := maxCompactionLevel(parts, nil) + 1
	return &compactionPlan{
		level:       -1,
		outputLevel: outputLevel,
		candidates:  append([]sstable.PartMeta(nil), parts...),
		output:      outputOptionsForLevel(opts, outputLevel),
	}
}

func compactionTriggered(
	parts []sstable.PartMeta,
	tombstones []model.Tombstone,
	opts model.CompactionLevelOptions,
) (bool, error) {
	if len(tombstones) > 0 && len(parts) > 0 {
		return true, nil
	}
	if len(parts) > opts.PartLimit {
		return true, nil
	}
	if computeSingleLevelHealth(opts.Level, append([]sstable.PartMeta(nil), parts...)).OverlapCount > 0 {
		return true, nil
	}
	if opts.SizeLimit <= 0 {
		return false, nil
	}
	size, err := compactionPartsSize(parts)
	if err != nil {
		return false, err
	}
	return size > opts.SizeLimit, nil
}

func compactionPartsSize(parts []sstable.PartMeta) (int64, error) {
	var total int64
	for _, part := range parts {
		size, err := directorySize(part.Path)
		if err != nil {
			return 0, err
		}
		total += size
	}
	return total, nil
}

func maxCompactionLevel(parts []sstable.PartMeta, levels []model.CompactionLevelOptions) int {
	maxLevel := 0
	for _, part := range parts {
		if part.Level > maxLevel {
			maxLevel = part.Level
		}
	}
	for _, level := range levels {
		if level.Level > maxLevel {
			maxLevel = level.Level
		}
	}
	return maxLevel
}

func partsAtLevel(parts []sstable.PartMeta, level int) []sstable.PartMeta {
	out := make([]sstable.PartMeta, 0)
	for _, part := range parts {
		if part.Level == level {
			out = append(out, part)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MinTime == out[j].MinTime {
			return out[i].ID < out[j].ID
		}
		return out[i].MinTime < out[j].MinTime
	})
	return out
}

func compactionLevelOptions(
	opts model.CompactionOptions,
	level int,
) model.CompactionLevelOptions {
	for _, candidate := range opts.Levels {
		if candidate.Level == level {
			return candidate
		}
	}
	return model.CompactionLevelOptions{
		Level:              level,
		PartLimit:          defaultLevelPartLimit,
		MaxOutputPartBytes: opts.MaxOutputPartBytes,
	}
}

func outputOptionsForLevel(
	opts model.CompactionOptions,
	level int,
) compactionOutputOptions {
	levelOpts := compactionLevelOptions(opts, level)
	return compactionOutputOptions{
		maxOutputPartBytes: levelOpts.MaxOutputPartBytes,
		compression:        levelOpts.Compression,
	}
}
