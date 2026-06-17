package engine

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/sstable"
)

const (
	compactionReasonTombstone         = "tombstone"
	compactionReasonPartLimit         = "part_limit"
	compactionReasonSizeLimit         = "size_limit"
	compactionReasonOverlap           = "overlap"
	compactionReasonReadAmplification = "read_amplification"
	compactionReasonFull              = "full"
	compactionOutputEstimateKeepRatio = 0.8
	compactionReadAmplificationBoost  = 1000
	compactionOverlapBoost            = 100
)

type compactionPlan struct {
	level               int
	outputLevel         int
	candidates          []sstable.PartMeta
	output              compactionOutputOptions
	reason              string
	score               float64
	inputBytes          int64
	outputEstimateBytes int64
	candidateSignature  string
}

type compactionOutputOptions struct {
	maxOutputPartBytes int64
	compression        model.CompressionOptions
}

type compactionBacklogSnapshot struct {
	Levels              map[int]compactionLevelBacklog
	PendingPlans        int
	OverlapCount        int
	MaxScore            float64
	InputBytes          int64
	OutputEstimateBytes int64
	Degraded            bool
	Best                *compactionPlan
}

type compactionLevelBacklog struct {
	Level               int
	PartCount           int
	CandidateCount      int
	OverlapCount        int
	InputBytes          int64
	OutputEstimateBytes int64
	Score               float64
	Reason              string
}

func nextCompactionPlan(
	parts []sstable.PartMeta,
	tombstones []model.Tombstone,
	opts model.CompactionOptions,
) (*compactionPlan, error) {
	backlog, err := buildCompactionBacklog(parts, tombstones, opts)
	if err != nil {
		return nil, err
	}
	return backlog.Best, nil
}

func buildCompactionBacklog(
	parts []sstable.PartMeta,
	tombstones []model.Tombstone,
	opts model.CompactionOptions,
) (compactionBacklogSnapshot, error) {
	snapshot := compactionBacklogSnapshot{Levels: make(map[int]compactionLevelBacklog)}
	maxLevel := maxCompactionLevel(parts, opts.Levels)
	for level := 0; level <= maxLevel; level++ {
		candidates := partsAtLevel(parts, level)
		if len(candidates) == 0 {
			continue
		}
		levelOpts := compactionLevelOptions(opts, level)
		trigger, err := compactionTrigger(candidates, tombstones, levelOpts, opts)
		if err != nil {
			return compactionBacklogSnapshot{}, err
		}
		if !trigger.triggered {
			continue
		}
		inputBytes, err := compactionPartsSize(candidates)
		if err != nil {
			return compactionBacklogSnapshot{}, err
		}
		outputEstimateBytes := estimateCompactionOutputBytes(inputBytes, tombstones)
		outputLevel := level + 1
		plan := &compactionPlan{
			level:               level,
			outputLevel:         outputLevel,
			candidates:          candidates,
			output:              outputOptionsForLevel(opts, outputLevel),
			reason:              trigger.reason,
			score:               trigger.score,
			inputBytes:          inputBytes,
			outputEstimateBytes: outputEstimateBytes,
			candidateSignature:  compactionCandidateSignature(level, candidates),
		}
		levelBacklog := compactionLevelBacklog{
			Level:               level,
			PartCount:           len(candidates),
			CandidateCount:      len(candidates),
			OverlapCount:        trigger.overlapCount,
			InputBytes:          inputBytes,
			OutputEstimateBytes: outputEstimateBytes,
			Score:               trigger.score,
			Reason:              trigger.reason,
		}
		snapshot.Levels[level] = levelBacklog
		snapshot.PendingPlans++
		snapshot.OverlapCount += trigger.overlapCount
		snapshot.InputBytes += inputBytes
		snapshot.OutputEstimateBytes += outputEstimateBytes
		if trigger.score > snapshot.MaxScore {
			snapshot.MaxScore = trigger.score
		}
		if snapshot.Best == nil || plan.score > snapshot.Best.score ||
			(plan.score == snapshot.Best.score && plan.level < snapshot.Best.level) {
			snapshot.Best = plan
		}
	}
	snapshot.Degraded = compactionBacklogDegraded(snapshot, opts)
	return snapshot, nil
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
		level:               -1,
		outputLevel:         outputLevel,
		candidates:          append([]sstable.PartMeta(nil), parts...),
		output:              outputOptionsForLevel(opts, outputLevel),
		reason:              compactionReasonFull,
		score:               float64(len(parts)),
		inputBytes:          partMetaBytes(parts),
		outputEstimateBytes: estimateCompactionOutputBytes(partMetaBytes(parts), tombstones),
		candidateSignature:  compactionCandidateSignature(-1, parts),
	}
}

type compactionTriggerDecision struct {
	triggered    bool
	reason       string
	score        float64
	overlapCount int
}

func compactionTrigger(
	parts []sstable.PartMeta,
	tombstones []model.Tombstone,
	opts model.CompactionLevelOptions,
	global model.CompactionOptions,
) (compactionTriggerDecision, error) {
	health := computeSingleLevelHealth(opts.Level, append([]sstable.PartMeta(nil), parts...))
	if len(tombstones) > 0 && len(parts) > 0 {
		return compactionTriggerDecision{
			triggered:    true,
			reason:       compactionReasonTombstone,
			score:        scoreFromRatio(len(tombstones)+len(parts), 1),
			overlapCount: health.OverlapCount,
		}, nil
	}
	if global.ReadAmplificationPartLimit > 0 && len(parts) > global.ReadAmplificationPartLimit {
		return compactionTriggerDecision{
			triggered: true,
			reason:    compactionReasonReadAmplification,
			score: compactionReadAmplificationBoost +
				scoreFromRatio(len(parts), global.ReadAmplificationPartLimit) +
				float64(health.OverlapCount)*compactionOverlapBoost,
			overlapCount: health.OverlapCount,
		}, nil
	}
	if len(parts) > opts.PartLimit {
		return compactionTriggerDecision{
			triggered:    true,
			reason:       compactionReasonPartLimit,
			score:        scoreFromRatio(len(parts), opts.PartLimit),
			overlapCount: health.OverlapCount,
		}, nil
	}
	if health.OverlapCount > 0 {
		return compactionTriggerDecision{
			triggered:    true,
			reason:       compactionReasonOverlap,
			score:        compactionOverlapBoost + float64(health.OverlapCount),
			overlapCount: health.OverlapCount,
		}, nil
	}
	if opts.SizeLimit <= 0 {
		return compactionTriggerDecision{}, nil
	}
	size, err := compactionPartsSize(parts)
	if err != nil {
		return compactionTriggerDecision{}, err
	}
	if size <= opts.SizeLimit {
		return compactionTriggerDecision{}, nil
	}
	return compactionTriggerDecision{
		triggered:    true,
		reason:       compactionReasonSizeLimit,
		score:        scoreFromRatioInt64(size, opts.SizeLimit),
		overlapCount: health.OverlapCount,
	}, nil
}

func compactionPartsSize(parts []sstable.PartMeta) (int64, error) {
	var total int64
	for _, part := range parts {
		if part.Path == "" {
			total += int64(part.BlockCount)
			continue
		}
		size, err := directorySize(part.Path)
		if err != nil {
			return 0, err
		}
		total += size
	}
	return total, nil
}

func estimateCompactionOutputBytes(inputBytes int64, tombstones []model.Tombstone) int64 {
	if inputBytes <= 0 {
		return 0
	}
	if len(tombstones) == 0 {
		return inputBytes
	}
	estimate := int64(math.Ceil(float64(inputBytes) * compactionOutputEstimateKeepRatio))
	if estimate <= 0 {
		return inputBytes
	}
	return estimate
}

func compactionBacklogDegraded(
	snapshot compactionBacklogSnapshot,
	opts model.CompactionOptions,
) bool {
	if snapshot.OverlapCount > 0 {
		return true
	}
	return opts.BacklogDegradedThreshold > 0 &&
		snapshot.PendingPlans >= opts.BacklogDegradedThreshold
}

func scoreFromRatio(value int, limit int) float64 {
	if limit <= 0 {
		return float64(value)
	}
	return float64(value) / float64(limit)
}

func scoreFromRatioInt64(value int64, limit int64) float64 {
	if limit <= 0 {
		return float64(value)
	}
	return float64(value) / float64(limit)
}

func compactionCandidateSignature(level int, parts []sstable.PartMeta) string {
	ids := make([]string, 0, len(parts))
	for _, part := range parts {
		ids = append(ids, part.ID)
	}
	sort.Strings(ids)
	var builder strings.Builder
	builder.WriteString("level:")
	builder.WriteString(strconv.Itoa(level))
	for _, id := range ids {
		builder.WriteByte('|')
		builder.WriteString(id)
	}
	return builder.String()
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
