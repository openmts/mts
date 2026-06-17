package engine

import (
	"sort"

	"codeberg.org/mts/mts/internal/sstable"
)

type ReadAmplificationOptions struct {
	MaxParts   int
	MaxPages   int
	MaxSamples int
	MaxOverlap int
}

type LevelHealthSnapshot struct {
	Levels   map[int]LevelHealth
	Degraded bool
}

type LevelHealth struct {
	Level        int
	PartCount    int
	Bytes        int64
	OverlapCount int
	Score        float64
}

func ComputeLevelHealth(parts []sstable.PartMeta) LevelHealthSnapshot {
	byLevel := groupPartsByLevel(parts)
	levels := make(map[int]LevelHealth, len(byLevel))
	degraded := false
	for level, candidates := range byLevel {
		health := computeSingleLevelHealth(level, candidates)
		levels[level] = health
		if health.OverlapCount > 0 {
			degraded = true
		}
	}
	return LevelHealthSnapshot{Levels: levels, Degraded: degraded}
}

func groupPartsByLevel(parts []sstable.PartMeta) map[int][]sstable.PartMeta {
	byLevel := make(map[int][]sstable.PartMeta)
	for _, part := range parts {
		byLevel[part.Level] = append(byLevel[part.Level], part)
	}
	return byLevel
}

func computeSingleLevelHealth(level int, parts []sstable.PartMeta) LevelHealth {
	sort.Slice(parts, func(i, j int) bool {
		if parts[i].MinSeriesID != parts[j].MinSeriesID {
			return parts[i].MinSeriesID < parts[j].MinSeriesID
		}
		return parts[i].MinTime < parts[j].MinTime
	})
	health := LevelHealth{Level: level, PartCount: len(parts)}
	for index, part := range parts {
		health.Bytes += int64(part.BlockCount)
		if index > 0 && partsOverlap(parts[index-1], part) {
			health.OverlapCount++
		}
	}
	if health.PartCount > 0 {
		health.Score = float64(health.PartCount+health.OverlapCount) / float64(health.PartCount)
	}
	return health
}

func partsOverlap(left sstable.PartMeta, right sstable.PartMeta) bool {
	if !partHasHealthRange(left) || !partHasHealthRange(right) {
		return false
	}
	seriesOverlap := left.MaxSeriesID >= right.MinSeriesID && right.MaxSeriesID >= left.MinSeriesID
	timeOverlap := left.MaxTime >= right.MinTime && right.MaxTime >= left.MinTime
	return seriesOverlap && timeOverlap
}

func partHasHealthRange(part sstable.PartMeta) bool {
	return part.RowsCount > 0 || part.SeriesCount > 0 || part.BlockCount > 0
}
