package engine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/openmts/mts/internal/collections"
	"github.com/openmts/mts/internal/model"
)

type DownsampleWindowResult struct {
	PolicyName    string
	WindowStart   int64
	WindowEnd     int64
	PointsWritten int
}

type DownsampleRunResult struct {
	PolicyName         string
	WindowsProcessed   int
	PointsWritten      int
	StartedUnix        int64
	CompletedUnix      int64
	CompletedUntilUnix int64
	Error              string
}

var ErrDownsamplePolicyNotFound = errors.New("downsample policy not found")

func (e *Engine) RunDownsamplePolicy(
	ctx context.Context,
	name string,
	now time.Duration,
) (DownsampleRunResult, error) {
	started := time.Now().UnixNano()
	policy, err := e.downsamplePolicyByName(ctx, name)
	if err != nil {
		return DownsampleRunResult{}, err
	}
	result := DownsampleRunResult{
		PolicyName:    policy.Name,
		StartedUnix:   started,
		CompletedUnix: started,
	}
	if !policy.Enabled {
		return result, nil
	}
	watermark, _, err := e.metadata.DownsampleWatermark(ctx, policy.Name)
	if err != nil {
		return DownsampleRunResult{}, err
	}
	attempt := e.downsampleStats.begin(policy.Name)
	windows := downsampleWindowsToRun(policy, watermark, now)
	result, err = e.runDownsampleWindows(ctx, policy, watermark, windows, result)
	if err != nil {
		attempt.finishFailure(result, err)
		e.logger.Warn("downsample policy run failed",
			"policy", policy.Name,
			"windows", result.WindowsProcessed,
			"error", err,
		)
		return result, err
	}
	attempt.finishSuccess(result)
	e.logger.Info("downsample policy run completed",
		"policy", policy.Name,
		"windows", result.WindowsProcessed,
		"points_written", result.PointsWritten,
	)
	return result, nil
}

func (e *Engine) runDownsampleWindow(
	ctx context.Context,
	policy model.DownsamplePolicy,
	start int64,
	end int64,
) (DownsampleWindowResult, error) {
	if err := ctx.Err(); err != nil {
		return DownsampleWindowResult{}, err
	}
	if start >= end {
		return DownsampleWindowResult{}, fmt.Errorf("downsample window start must be before end")
	}
	query := downsampleSourceQuery(policy, start, end)
	iter, err := e.QueryColumnIterator(ctx, query)
	if err != nil {
		return DownsampleWindowResult{}, err
	}
	aggregator := newDownsampleWindowAggregator(policy, start, end)
	if err := aggregator.addIterator(iter); err != nil {
		return DownsampleWindowResult{}, err
	}
	written, err := aggregator.write(ctx, e, policy.BatchSize)
	if err != nil {
		return DownsampleWindowResult{}, err
	}
	if written == 0 {
		return DownsampleWindowResult{
			PolicyName:  policy.Name,
			WindowStart: start,
			WindowEnd:   end,
		}, nil
	}
	return DownsampleWindowResult{
		PolicyName:    policy.Name,
		WindowStart:   start,
		WindowEnd:     end,
		PointsWritten: written,
	}, nil
}

func (e *Engine) downsamplePolicyByName(
	ctx context.Context,
	name string,
) (model.DownsamplePolicy, error) {
	if strings.TrimSpace(name) == "" {
		return model.DownsamplePolicy{}, fmt.Errorf("downsample policy name is empty")
	}
	policies, err := e.metadata.ListDownsamplePolicies(ctx)
	if err != nil {
		return model.DownsamplePolicy{}, err
	}
	for _, policy := range policies {
		if policy.Name == name {
			return policy, nil
		}
	}
	return model.DownsamplePolicy{}, fmt.Errorf("%w: %q", ErrDownsamplePolicyNotFound, name)
}

func (e *Engine) runDownsampleWindows(
	ctx context.Context,
	policy model.DownsamplePolicy,
	watermark model.DownsampleWatermark,
	windows []downsampleWindow,
	result DownsampleRunResult,
) (DownsampleRunResult, error) {
	checkpoint := watermark
	completed := watermark.CompletedUntilUnix
	advanceSinceCheckpoint := 0
	for _, window := range windows {
		windowResult, err := e.runDownsampleWindow(ctx, policy, window.start, window.end)
		if err != nil {
			return e.recordDownsampleRunFailure(ctx, policy.Name, checkpoint, result, err)
		}
		result.WindowsProcessed++
		result.PointsWritten += windowResult.PointsWritten
		if window.refresh {
			continue
		}
		if window.end > completed {
			completed = window.end
			advanceSinceCheckpoint++
		}
		if advanceSinceCheckpoint >= policy.CheckpointInterval {
			var err error
			result, checkpoint, err = e.recordDownsampleRunSuccess(
				ctx,
				policy.Name,
				completed,
				result,
			)
			if err != nil {
				return e.recordDownsampleRunFailure(ctx, policy.Name, checkpoint, result, err)
			}
			advanceSinceCheckpoint = 0
		}
	}
	var err error
	result, _, err = e.recordDownsampleRunSuccess(ctx, policy.Name, completed, result)
	return result, err
}

func (e *Engine) recordDownsampleRunSuccess(
	ctx context.Context,
	policyName string,
	completed int64,
	result DownsampleRunResult,
) (DownsampleRunResult, model.DownsampleWatermark, error) {
	result.CompletedUnix = time.Now().UnixNano()
	result.CompletedUntilUnix = completed
	watermark := model.DownsampleWatermark{
		PolicyName:         policyName,
		CompletedUntilUnix: completed,
		LastRunUnix:        result.CompletedUnix,
		LastSuccessUnix:    result.CompletedUnix,
	}
	err := e.metadata.UpdateDownsampleWatermark(ctx, watermark)
	return result, watermark, err
}

func (e *Engine) recordDownsampleRunFailure(
	ctx context.Context,
	policyName string,
	watermark model.DownsampleWatermark,
	result DownsampleRunResult,
	cause error,
) (DownsampleRunResult, error) {
	result.CompletedUnix = time.Now().UnixNano()
	result.CompletedUntilUnix = watermark.CompletedUntilUnix
	result.Error = cause.Error()
	updateCtx := ctx
	if ctx.Err() != nil {
		updateCtx = context.Background()
	}
	updateErr := e.metadata.UpdateDownsampleWatermark(updateCtx, model.DownsampleWatermark{
		PolicyName:         policyName,
		CompletedUntilUnix: watermark.CompletedUntilUnix,
		LastRunUnix:        result.CompletedUnix,
		LastSuccessUnix:    watermark.LastSuccessUnix,
		LastError:          cause.Error(),
		AllowPolicyReplace: watermark.AllowPolicyReplace,
	})
	return result, errors.Join(cause, updateErr)
}

type downsampleWindow struct {
	start   int64
	end     int64
	refresh bool
}

func downsampleWindowsToRun(
	policy model.DownsamplePolicy,
	watermark model.DownsampleWatermark,
	now time.Duration,
) []downsampleWindow {
	eligibleUntil := alignDownsampleWindow(int64(now-policy.Delay), policy.Interval)
	if eligibleUntil <= 0 || policy.Interval <= 0 {
		return nil
	}
	advanceStart := downsampleAdvanceStart(policy, watermark, eligibleUntil)
	refreshStart := downsampleRefreshStart(policy, watermark, advanceStart)
	start := minInt64(refreshStart, advanceStart)
	if start >= eligibleUntil {
		return nil
	}
	interval := int64(policy.Interval)
	windows := make([]downsampleWindow, 0, int((eligibleUntil-start)/interval))
	for current := start; current < eligibleUntil; current += interval {
		windows = append(windows, downsampleWindow{
			start:   current,
			end:     current + interval,
			refresh: current < advanceStart,
		})
	}
	return windows
}

func downsampleAdvanceStart(
	policy model.DownsamplePolicy,
	watermark model.DownsampleWatermark,
	eligibleUntil int64,
) int64 {
	start := watermark.CompletedUntilUnix
	if start == 0 {
		start = downsampleInitialAdvanceStart(policy, eligibleUntil)
	}
	if start < 0 {
		start = 0
	}
	return alignDownsampleWindow(start, policy.Interval)
}

func downsampleInitialAdvanceStart(policy model.DownsamplePolicy, eligibleUntil int64) int64 {
	if policy.InitialStartTime > 0 {
		return policy.InitialStartTime
	}
	if policy.Lookback > 0 {
		return eligibleUntil - int64(policy.Lookback)
	}
	return eligibleUntil - int64(policy.Interval)
}

func downsampleRefreshStart(
	policy model.DownsamplePolicy,
	watermark model.DownsampleWatermark,
	advanceStart int64,
) int64 {
	if watermark.CompletedUntilUnix <= 0 || policy.Lookback <= 0 {
		return advanceStart
	}
	start := watermark.CompletedUntilUnix - int64(policy.Lookback)
	if start < 0 {
		start = 0
	}
	return alignDownsampleWindow(start, policy.Interval)
}

func downsampleRangeWindows(
	policy model.DownsamplePolicy,
	watermark model.DownsampleWatermark,
	start int64,
	end int64,
	advance bool,
) ([]downsampleWindow, int64, int64, error) {
	if start < 0 || end < 0 {
		return nil, 0, 0, fmt.Errorf("downsample range boundaries must be greater than or equal to zero")
	}
	if start >= end {
		return nil, 0, 0, fmt.Errorf("downsample range start must be before end")
	}
	alignedStart := alignDownsampleWindow(start, policy.Interval)
	alignedEnd := alignDownsampleWindow(end, policy.Interval)
	if alignedEnd <= alignedStart {
		return nil, alignedStart, alignedEnd, nil
	}
	interval := int64(policy.Interval)
	windows := make([]downsampleWindow, 0, int((alignedEnd-alignedStart)/interval))
	for current := alignedStart; current < alignedEnd; current += interval {
		refresh := !advance || current < watermark.CompletedUntilUnix
		windows = append(windows, downsampleWindow{
			start:   current,
			end:     current + interval,
			refresh: refresh,
		})
	}
	return windows, alignedStart, alignedEnd, nil
}

func markDownsampleWindowsRefresh(windows []downsampleWindow) []downsampleWindow {
	out := make([]downsampleWindow, len(windows))
	for index, window := range windows {
		window.refresh = true
		out[index] = window
	}
	return out
}

func minInt64(left int64, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func downsampleSourceQuery(policy model.DownsamplePolicy, start int64, end int64) model.Query {
	return model.Query{
		Database:        policy.SourceDatabase,
		RetentionPolicy: policy.SourceRetention,
		Measurement:     policy.SourceMeasurement,
		Fields:          downsampleSourceFields(policy.Functions),
		StartTime:       start,
		EndTime:         end,
		Order: model.QueryOrder{
			By:        model.QueryOrderByTime,
			Direction: model.QuerySortAsc,
		},
	}
}

func downsampleSourceFields(functions []model.DownsampleFunction) []string {
	fields := make([]string, 0, len(functions))
	seen := make(map[string]struct{}, len(functions))
	for _, function := range functions {
		if _, ok := seen[function.Field]; ok {
			continue
		}
		seen[function.Field] = struct{}{}
		fields = append(fields, function.Field)
	}
	return fields
}

func downsampleColumnsToPoints(
	policy model.DownsamplePolicy,
	start int64,
	end int64,
	columns []model.ColumnSeries,
) ([]model.Point, error) {
	fields := downsampleOutputFields(policy.Functions)
	byKey := make(map[string]*model.Point, len(columns))
	for _, column := range columns {
		fieldName, ok := fields[column.FieldName]
		if !ok {
			return nil, fmt.Errorf("unexpected downsample aggregate column %q", column.FieldName)
		}
		if len(column.Timestamps) != len(column.Values) {
			return nil, fmt.Errorf("downsample column %q has mismatched samples", column.FieldName)
		}
		for index, timestamp := range column.Timestamps {
			if timestamp < start || timestamp >= end {
				continue
			}
			point := downsamplePointForColumn(policy, byKey, column, timestamp)
			point.Fields[fieldName] = column.Values[index]
		}
	}
	return sortedDownsamplePoints(byKey), nil
}

func downsampleOutputFields(functions []model.DownsampleFunction) map[string]string {
	fields := make(map[string]string, len(functions))
	for _, function := range functions {
		fields[downsampleAggregateColumnName(function)] = function.As
	}
	return fields
}

func downsampleAggregateColumnName(function model.DownsampleFunction) string {
	return function.Function + "(" + function.Field + ")"
}

func downsamplePointForColumn(
	policy model.DownsamplePolicy,
	byKey map[string]*model.Point,
	column model.ColumnSeries,
	timestamp int64,
) *model.Point {
	key := downsamplePointKey(column.Tags, timestamp)
	point := byKey[key]
	if point != nil {
		return point
	}
	point = &model.Point{
		Database:        policy.TargetDatabase,
		RetentionPolicy: policy.TargetRetention,
		Measurement:     policy.TargetMeasurement,
		Tags:            downsampleTargetTags(column.Tags, policy),
		Timestamp:       timestamp,
		Fields:          make(map[string]model.FieldValue, len(policy.Functions)),
	}
	byKey[key] = point
	return point
}

func downsamplePointKey(tags map[string]string, timestamp int64) string {
	var builder strings.Builder
	builder.WriteString(strconv.FormatInt(timestamp, 10))
	names := sortedTagNames(tags)
	for _, name := range names {
		builder.WriteByte(0)
		builder.WriteString(name)
		builder.WriteByte('=')
		builder.WriteString(tags[name])
	}
	return builder.String()
}

func cloneDownsampleTags(tags map[string]string) map[string]string {
	return collections.CloneMapNilIfEmpty(tags)
}

func downsampleTargetTags(
	tags map[string]string,
	policy model.DownsamplePolicy,
) map[string]string {
	out := cloneDownsampleTags(tags)
	if out == nil {
		out = make(map[string]string, 1)
	}
	out[policy.PolicyTagName] = policy.Name
	return out
}

func sortedTagNames(tags map[string]string) []string {
	return collections.SortedKeys(tags)
}

func sortedDownsamplePoints(byKey map[string]*model.Point) []model.Point {
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	points := make([]model.Point, 0, len(keys))
	for _, key := range keys {
		points = append(points, *byKey[key])
	}
	return points
}
