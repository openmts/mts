package mts

import (
	"context"
	"time"

	storageengine "github.com/openmts/mts/internal/engine"
	"github.com/openmts/mts/internal/model"
)

// CreateDownsamplePolicy 创建或更新本地降采样策略。
func (e *Engine) CreateDownsamplePolicy(ctx context.Context, policy DownsamplePolicy) error {
	return publicError(e.runtime.Storage().CreateDownsamplePolicy(ctx, toModelDownsamplePolicy(policy)))
}

// EnableDownsamplePolicy 启用降采样策略。
func (e *Engine) EnableDownsamplePolicy(ctx context.Context, name string) error {
	return publicError(e.runtime.Storage().EnableDownsamplePolicy(ctx, name))
}

// DisableDownsamplePolicy 禁用降采样策略。
func (e *Engine) DisableDownsamplePolicy(ctx context.Context, name string) error {
	return publicError(e.runtime.Storage().DisableDownsamplePolicy(ctx, name))
}

// DropDownsamplePolicy 删除降采样策略但不清理目标 rollup 数据。
func (e *Engine) DropDownsamplePolicy(ctx context.Context, name string) error {
	return publicError(e.runtime.Storage().DropDownsamplePolicy(ctx, name))
}

// DropDownsamplePolicyWithOptions 删除降采样策略，并按选项清理目标 rollup 数据。
func (e *Engine) DropDownsamplePolicyWithOptions(
	ctx context.Context,
	name string,
	opts DownsampleDropOptions,
) error {
	return publicError(e.runtime.Storage().DropDownsamplePolicyWithOptions(ctx, name, toModelDownsampleDropOptions(opts)))
}

// ResetDownsamplePolicy 重置降采样策略 watermark 和替换许可。
func (e *Engine) ResetDownsamplePolicy(
	ctx context.Context,
	name string,
	reset DownsampleReset,
) error {
	return publicError(e.runtime.Storage().ResetDownsamplePolicy(ctx, name, toModelDownsampleReset(reset)))
}

// ListDownsamplePolicies 列出本地降采样策略。
func (e *Engine) ListDownsamplePolicies(ctx context.Context) ([]DownsamplePolicy, error) {
	policies, err := e.runtime.Storage().ListDownsamplePolicies(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	return fromModelDownsamplePolicies(policies), nil
}

// GetDownsamplePolicy 按名称读取单条本地降采样策略。
func (e *Engine) GetDownsamplePolicy(ctx context.Context, name string) (DownsamplePolicy, error) {
	policy, err := e.runtime.Storage().GetDownsamplePolicy(ctx, name)
	if err != nil {
		return DownsamplePolicy{}, publicError(err)
	}
	return fromModelDownsamplePolicy(policy), nil
}

// DownsamplePolicyStatuses 返回降采样策略运行状态。
func (e *Engine) DownsamplePolicyStatuses(
	ctx context.Context,
	now time.Time,
) ([]DownsamplePolicyStatus, error) {
	statuses, err := e.runtime.Storage().DownsamplePolicyStatuses(ctx, time.Duration(now.UnixNano()))
	if err != nil {
		return nil, publicError(err)
	}
	return fromModelDownsamplePolicyStatuses(statuses), nil
}

// RunDownsamplePolicy 按当前时间触发一次策略运行。
func (e *Engine) RunDownsamplePolicy(
	ctx context.Context,
	name string,
	now time.Time,
) (DownsampleRunResult, error) {
	result, err := e.runtime.Storage().RunDownsamplePolicy(ctx, name, time.Duration(now.UnixNano()))
	return fromStorageDownsampleRunResult(result), publicError(err)
}

// RunDownsamplePolicyRange 对指定时间范围手动运行降采样。
func (e *Engine) RunDownsamplePolicyRange(
	ctx context.Context,
	name string,
	start time.Time,
	end time.Time,
	opts DownsampleRangeOptions,
) (DownsampleRunResult, error) {
	result, err := e.runtime.Storage().RunDownsamplePolicyRange(
		ctx,
		name,
		start.UnixNano(),
		end.UnixNano(),
		toModelDownsampleRangeOptions(opts),
	)
	return fromStorageDownsampleRunResult(result), publicError(err)
}

// RepairDownsamplePolicy 重算指定时间范围内的降采样结果。
func (e *Engine) RepairDownsamplePolicy(
	ctx context.Context,
	name string,
	start time.Time,
	end time.Time,
) (DownsampleRunResult, error) {
	result, err := e.runtime.Storage().RepairDownsamplePolicy(ctx, name, start.UnixNano(), end.UnixNano())
	return fromStorageDownsampleRunResult(result), publicError(err)
}

// DryRunDownsamplePolicy 估算指定时间范围内降采样成本，不写入目标数据。
func (e *Engine) DryRunDownsamplePolicy(
	ctx context.Context,
	name string,
	start time.Time,
	end time.Time,
) (DownsampleDryRunResult, error) {
	result, err := e.runtime.Storage().DryRunDownsamplePolicy(ctx, name, start.UnixNano(), end.UnixNano())
	return fromModelDownsampleDryRunResult(result), publicError(err)
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
