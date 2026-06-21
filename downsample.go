package mts

import (
	"context"
	"time"

	storageengine "github.com/openmts/mts/internal/engine"
	"github.com/openmts/mts/internal/model"
)

func (e *Engine) CreateDownsamplePolicy(ctx context.Context, policy DownsamplePolicy) error {
	return e.inner.CreateDownsamplePolicy(ctx, toModelDownsamplePolicy(policy))
}

func (e *Engine) EnableDownsamplePolicy(ctx context.Context, name string) error {
	return e.inner.EnableDownsamplePolicy(ctx, name)
}

func (e *Engine) DisableDownsamplePolicy(ctx context.Context, name string) error {
	return e.inner.DisableDownsamplePolicy(ctx, name)
}

func (e *Engine) DropDownsamplePolicy(ctx context.Context, name string) error {
	return e.inner.DropDownsamplePolicy(ctx, name)
}

func (e *Engine) DropDownsamplePolicyWithOptions(
	ctx context.Context,
	name string,
	opts DownsampleDropOptions,
) error {
	return e.inner.DropDownsamplePolicyWithOptions(ctx, name, toModelDownsampleDropOptions(opts))
}

func (e *Engine) ResetDownsamplePolicy(
	ctx context.Context,
	name string,
	reset DownsampleReset,
) error {
	return e.inner.ResetDownsamplePolicy(ctx, name, toModelDownsampleReset(reset))
}

func (e *Engine) ListDownsamplePolicies(ctx context.Context) ([]DownsamplePolicy, error) {
	policies, err := e.inner.ListDownsamplePolicies(ctx)
	if err != nil {
		return nil, err
	}
	return fromModelDownsamplePolicies(policies), nil
}

func (e *Engine) DownsamplePolicyStatuses(
	ctx context.Context,
	now time.Time,
) ([]DownsamplePolicyStatus, error) {
	statuses, err := e.inner.DownsamplePolicyStatuses(ctx, time.Duration(now.UnixNano()))
	if err != nil {
		return nil, err
	}
	return fromModelDownsamplePolicyStatuses(statuses), nil
}

func (e *Engine) RunDownsamplePolicy(
	ctx context.Context,
	name string,
	now time.Time,
) (DownsampleRunResult, error) {
	result, err := e.inner.RunDownsamplePolicy(ctx, name, time.Duration(now.UnixNano()))
	return fromStorageDownsampleRunResult(result), err
}

func (e *Engine) RunDownsamplePolicyRange(
	ctx context.Context,
	name string,
	start time.Time,
	end time.Time,
	opts DownsampleRangeOptions,
) (DownsampleRunResult, error) {
	result, err := e.inner.RunDownsamplePolicyRange(
		ctx,
		name,
		start.UnixNano(),
		end.UnixNano(),
		toModelDownsampleRangeOptions(opts),
	)
	return fromStorageDownsampleRunResult(result), err
}

func (e *Engine) RepairDownsamplePolicy(
	ctx context.Context,
	name string,
	start time.Time,
	end time.Time,
) (DownsampleRunResult, error) {
	result, err := e.inner.RepairDownsamplePolicy(ctx, name, start.UnixNano(), end.UnixNano())
	return fromStorageDownsampleRunResult(result), err
}

func (e *Engine) DryRunDownsamplePolicy(
	ctx context.Context,
	name string,
	start time.Time,
	end time.Time,
) (DownsampleDryRunResult, error) {
	result, err := e.inner.DryRunDownsamplePolicy(ctx, name, start.UnixNano(), end.UnixNano())
	return fromModelDownsampleDryRunResult(result), err
}

func toModelDownsamplePolicy(policy DownsamplePolicy) model.DownsamplePolicy {
	functions := make([]model.DownsampleFunction, len(policy.Functions))
	for index, function := range policy.Functions {
		functions[index] = model.DownsampleFunction{
			Function: function.Function,
			Field:    function.Field,
			As:       function.As,
		}
	}
	return model.DownsamplePolicy{
		Name:               policy.Name,
		SourceDatabase:     policy.SourceDatabase,
		SourceRetention:    policy.SourceRetention,
		SourceMeasurement:  policy.SourceMeasurement,
		TargetDatabase:     policy.TargetDatabase,
		TargetRetention:    policy.TargetRetention,
		TargetMeasurement:  policy.TargetMeasurement,
		Interval:           policy.Interval,
		Functions:          functions,
		GroupByTags:        append([]string(nil), policy.GroupByTags...),
		Delay:              policy.Delay,
		RefreshInterval:    policy.RefreshInterval,
		Lookback:           policy.Lookback,
		InitialStartTime:   policy.InitialStartTime,
		RunTimeout:         policy.RunTimeout,
		BatchSize:          policy.BatchSize,
		CheckpointInterval: policy.CheckpointInterval,
		PolicyTagName:      policy.PolicyTagName,
		Enabled:            policy.Enabled,
	}
}

func fromModelDownsamplePolicy(policy model.DownsamplePolicy) DownsamplePolicy {
	functions := make([]DownsampleFunction, len(policy.Functions))
	for index, function := range policy.Functions {
		functions[index] = DownsampleFunction{
			Function: function.Function,
			Field:    function.Field,
			As:       function.As,
		}
	}
	return DownsamplePolicy{
		Name:               policy.Name,
		SourceDatabase:     policy.SourceDatabase,
		SourceRetention:    policy.SourceRetention,
		SourceMeasurement:  policy.SourceMeasurement,
		TargetDatabase:     policy.TargetDatabase,
		TargetRetention:    policy.TargetRetention,
		TargetMeasurement:  policy.TargetMeasurement,
		Interval:           policy.Interval,
		Functions:          functions,
		GroupByTags:        append([]string(nil), policy.GroupByTags...),
		Delay:              policy.Delay,
		RefreshInterval:    policy.RefreshInterval,
		Lookback:           policy.Lookback,
		InitialStartTime:   policy.InitialStartTime,
		RunTimeout:         policy.RunTimeout,
		BatchSize:          policy.BatchSize,
		CheckpointInterval: policy.CheckpointInterval,
		PolicyTagName:      policy.PolicyTagName,
		Enabled:            policy.Enabled,
	}
}

func fromModelDownsamplePolicies(policies []model.DownsamplePolicy) []DownsamplePolicy {
	out := make([]DownsamplePolicy, len(policies))
	for index, policy := range policies {
		out[index] = fromModelDownsamplePolicy(policy)
	}
	return out
}

func fromModelDownsamplePolicyStatuses(
	statuses []model.DownsamplePolicyStatus,
) []DownsamplePolicyStatus {
	out := make([]DownsamplePolicyStatus, len(statuses))
	for index, status := range statuses {
		out[index] = DownsamplePolicyStatus{
			PolicyName:         status.PolicyName,
			Enabled:            status.Enabled,
			Active:             status.Active,
			CompletedUntilUnix: status.CompletedUntilUnix,
			LastRunUnix:        status.LastRunUnix,
			LastSuccessUnix:    status.LastSuccessUnix,
			LastError:          status.LastError,
			NextRunUnix:        status.NextRunUnix,
			LagSeconds:         status.LagSeconds,
			LastDuration:       status.LastDuration,
			WindowsProcessed:   status.WindowsProcessed,
			PointsWritten:      status.PointsWritten,
		}
	}
	return out
}

func fromStorageDownsampleRunResult(result storageDownsampleRunResult) DownsampleRunResult {
	return DownsampleRunResult{
		PolicyName:         result.PolicyName,
		WindowsProcessed:   result.WindowsProcessed,
		PointsWritten:      result.PointsWritten,
		StartedUnix:        result.StartedUnix,
		CompletedUnix:      result.CompletedUnix,
		CompletedUntilUnix: result.CompletedUntilUnix,
		Error:              result.Error,
	}
}

type storageDownsampleRunResult = storageengine.DownsampleRunResult

func toModelDownsampleReset(reset DownsampleReset) model.DownsampleReset {
	return model.DownsampleReset{
		CompletedUntilUnix: reset.CompletedUntilUnix,
		AllowPolicyReplace: reset.AllowPolicyReplace,
		CleanupTarget:      reset.CleanupTarget,
		CleanupStartUnix:   reset.CleanupStartUnix,
		CleanupEndUnix:     reset.CleanupEndUnix,
	}
}

func toModelDownsampleDropOptions(opts DownsampleDropOptions) model.DownsampleDropOptions {
	return model.DownsampleDropOptions{
		CleanupTarget:    opts.CleanupTarget,
		CleanupStartUnix: opts.CleanupStartUnix,
		CleanupEndUnix:   opts.CleanupEndUnix,
	}
}

func toModelDownsampleRangeOptions(opts DownsampleRangeOptions) model.DownsampleRangeOptions {
	return model.DownsampleRangeOptions{
		AdvanceWatermark: opts.AdvanceWatermark,
	}
}

func fromModelDownsampleDryRunResult(result model.DownsampleDryRunResult) DownsampleDryRunResult {
	return DownsampleDryRunResult{
		PolicyName:       result.PolicyName,
		StartUnix:        result.StartUnix,
		EndUnix:          result.EndUnix,
		Windows:          result.Windows,
		RefreshWindows:   result.RefreshWindows,
		AdvanceWindows:   result.AdvanceWindows,
		PointsEstimate:   result.PointsEstimate,
		GroupsEstimate:   result.GroupsEstimate,
		SamplesEstimate:  result.SamplesEstimate,
		EstimateComplete: result.EstimateComplete,
		WouldAdvance:     result.WouldAdvance,
	}
}
