package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/openmts/mts/internal/model"
)

func (e *Engine) CreateDownsamplePolicy(
	ctx context.Context,
	policy model.DownsamplePolicy,
) error {
	normalized, err := normalizeDownsamplePolicy(policy)
	if err != nil {
		return err
	}
	if err := e.validateDownsamplePolicyReplace(ctx, normalized); err != nil {
		return err
	}
	if err := e.metadata.UpsertDownsamplePolicy(ctx, normalized); err != nil {
		return err
	}
	return e.clearDownsamplePolicyReplaceAllowance(ctx, normalized.Name)
}

func (e *Engine) EnableDownsamplePolicy(ctx context.Context, name string) error {
	return e.setDownsamplePolicyEnabled(ctx, name, true)
}

func (e *Engine) DisableDownsamplePolicy(ctx context.Context, name string) error {
	return e.setDownsamplePolicyEnabled(ctx, name, false)
}

func (e *Engine) DropDownsamplePolicy(ctx context.Context, name string) error {
	return e.DropDownsamplePolicyWithOptions(ctx, name, model.DownsampleDropOptions{})
}

func (e *Engine) DropDownsamplePolicyWithOptions(
	ctx context.Context,
	name string,
	opts model.DownsampleDropOptions,
) error {
	if name == "" {
		return fmt.Errorf("downsample policy name is empty")
	}
	if opts.CleanupTarget {
		policy, err := e.downsamplePolicyByName(ctx, name)
		if err != nil {
			return err
		}
		if err := e.cleanupDownsampleTarget(ctx, policy, opts.CleanupStartUnix, opts.CleanupEndUnix); err != nil {
			return err
		}
	}
	return e.metadata.DropDownsamplePolicy(ctx, name)
}

func (e *Engine) ResetDownsamplePolicy(
	ctx context.Context,
	name string,
	reset model.DownsampleReset,
) error {
	if name == "" {
		return fmt.Errorf("downsample policy name is empty")
	}
	if reset.CompletedUntilUnix < 0 {
		return fmt.Errorf("downsample reset completed watermark must be greater than or equal to zero")
	}
	watermark, _, err := e.metadata.DownsampleWatermark(ctx, name)
	if err != nil {
		return err
	}
	watermark.PolicyName = name
	watermark.CompletedUntilUnix = reset.CompletedUntilUnix
	watermark.LastError = ""
	watermark.AllowPolicyReplace = reset.AllowPolicyReplace
	if reset.CleanupTarget {
		policy, err := e.downsamplePolicyByName(ctx, name)
		if err != nil {
			return err
		}
		if err := e.cleanupDownsampleTarget(ctx, policy, reset.CleanupStartUnix, reset.CleanupEndUnix); err != nil {
			return err
		}
	}
	return e.metadata.UpdateDownsampleWatermark(ctx, watermark)
}

func (e *Engine) ListDownsamplePolicies(
	ctx context.Context,
) ([]model.DownsamplePolicy, error) {
	return e.metadata.ListDownsamplePolicies(ctx)
}

func (e *Engine) GetDownsamplePolicy(
	ctx context.Context,
	name string,
) (model.DownsamplePolicy, error) {
	return e.downsamplePolicyByName(ctx, name)
}

func (e *Engine) DownsamplePolicyStatuses(
	ctx context.Context,
	now time.Duration,
) ([]model.DownsamplePolicyStatus, error) {
	policies, err := e.metadata.ListDownsamplePolicies(ctx)
	if err != nil {
		return nil, err
	}
	statuses := make([]model.DownsamplePolicyStatus, 0, len(policies))
	for _, policy := range policies {
		watermark, _, err := e.metadata.DownsampleWatermark(ctx, policy.Name)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, e.downsamplePolicyStatus(policy, watermark, now))
	}
	return statuses, nil
}

func (e *Engine) downsamplePolicyStatus(
	policy model.DownsamplePolicy,
	watermark model.DownsampleWatermark,
	now time.Duration,
) model.DownsamplePolicyStatus {
	runtime := e.downsampleStats.policySnapshot(policy.Name)
	lastRun := watermark.LastRunUnix
	if lastRun == 0 {
		lastRun = runtime.LastRunUnix
	}
	lastSuccess := watermark.LastSuccessUnix
	if lastSuccess == 0 {
		lastSuccess = runtime.LastSuccessUnix
	}
	lastError := watermark.LastError
	if lastError == "" {
		lastError = runtime.LastError
	}
	completed := watermark.CompletedUntilUnix
	if completed == 0 {
		completed = runtime.LastWatermarkUnix
	}
	nextRun := int64(0)
	if lastRun > 0 && policy.RefreshInterval > 0 {
		nextRun = lastRun + int64(policy.RefreshInterval)
	}
	lag := int64(0)
	if completed < int64(now) {
		lag = int64(now-time.Duration(completed)) / int64(time.Second)
	}
	return model.DownsamplePolicyStatus{
		PolicyName:         policy.Name,
		Enabled:            policy.Enabled,
		Active:             runtime.Active > 0,
		CompletedUntilUnix: completed,
		LastRunUnix:        lastRun,
		LastSuccessUnix:    lastSuccess,
		LastError:          lastError,
		NextRunUnix:        nextRun,
		LagSeconds:         lag,
		LastDuration:       runtime.LastDuration,
		WindowsProcessed:   runtime.WindowsProcessed,
		PointsWritten:      runtime.PointsWritten,
	}
}

func (e *Engine) DryRunDownsamplePolicy(
	ctx context.Context,
	name string,
	start int64,
	end int64,
) (model.DownsampleDryRunResult, error) {
	policy, err := e.downsamplePolicyByName(ctx, name)
	if err != nil {
		return model.DownsampleDryRunResult{}, err
	}
	watermark, _, err := e.metadata.DownsampleWatermark(ctx, policy.Name)
	if err != nil {
		return model.DownsampleDryRunResult{}, err
	}
	windows, alignedStart, alignedEnd, err := downsampleRangeWindows(policy, watermark, start, end, false)
	if err != nil {
		return model.DownsampleDryRunResult{}, err
	}
	result := model.DownsampleDryRunResult{
		PolicyName:       policy.Name,
		StartUnix:        alignedStart,
		EndUnix:          alignedEnd,
		Windows:          len(windows),
		EstimateComplete: true,
	}
	for _, window := range windows {
		if window.refresh {
			result.RefreshWindows++
		} else {
			result.AdvanceWindows++
		}
	}
	estimate, err := e.estimateDownsampleCost(ctx, policy, alignedStart, alignedEnd)
	if err != nil {
		return model.DownsampleDryRunResult{}, err
	}
	result.GroupsEstimate = estimate.groups
	result.SamplesEstimate = estimate.samples
	result.PointsEstimate = estimate.groups * len(windows)
	return result, nil
}

func (e *Engine) RepairDownsamplePolicy(
	ctx context.Context,
	name string,
	start int64,
	end int64,
) (DownsampleRunResult, error) {
	return e.RunDownsamplePolicyRange(ctx, name, start, end, model.DownsampleRangeOptions{})
}

func (e *Engine) RunDownsamplePolicyRange(
	ctx context.Context,
	name string,
	start int64,
	end int64,
	opts model.DownsampleRangeOptions,
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
	watermark, _, err := e.metadata.DownsampleWatermark(ctx, policy.Name)
	if err != nil {
		return DownsampleRunResult{}, err
	}
	windows, alignedStart, _, err := downsampleRangeWindows(policy, watermark, start, end, opts.AdvanceWatermark)
	if err != nil {
		return DownsampleRunResult{}, err
	}
	if opts.AdvanceWatermark && watermark.CompletedUntilUnix > 0 &&
		alignedStart > watermark.CompletedUntilUnix {
		return DownsampleRunResult{}, fmt.Errorf("downsample range starts after watermark")
	}
	if !opts.AdvanceWatermark {
		windows = markDownsampleWindowsRefresh(windows)
	}
	attempt := e.downsampleStats.begin(policy.Name)
	result, err = e.runDownsampleWindows(ctx, policy, watermark, windows, result)
	if err != nil {
		attempt.finishFailure(result, err)
		return result, err
	}
	attempt.finishSuccess(result)
	return result, nil
}

func (e *Engine) setDownsamplePolicyEnabled(
	ctx context.Context,
	name string,
	enabled bool,
) error {
	if name == "" {
		return fmt.Errorf("downsample policy name is empty")
	}
	policies, err := e.metadata.ListDownsamplePolicies(ctx)
	if err != nil {
		return err
	}
	for _, policy := range policies {
		if policy.Name != name {
			continue
		}
		policy.Enabled = enabled
		return e.metadata.UpsertDownsamplePolicy(ctx, policy)
	}
	return fmt.Errorf("%w: %q", ErrDownsamplePolicyNotFound, name)
}

func (e *Engine) validateDownsamplePolicyReplace(
	ctx context.Context,
	next model.DownsamplePolicy,
) error {
	policies, err := e.metadata.ListDownsamplePolicies(ctx)
	if err != nil {
		return err
	}
	for _, current := range policies {
		if current.Name != next.Name {
			continue
		}
		watermark, _, err := e.metadata.DownsampleWatermark(ctx, next.Name)
		if err != nil {
			return err
		}
		return validateDownsamplePolicyUpdate(current, next, watermark.AllowPolicyReplace)
	}
	return nil
}

func (e *Engine) clearDownsamplePolicyReplaceAllowance(ctx context.Context, name string) error {
	watermark, ok, err := e.metadata.DownsampleWatermark(ctx, name)
	if err != nil || !ok || !watermark.AllowPolicyReplace {
		return err
	}
	watermark.AllowPolicyReplace = false
	return e.metadata.UpdateDownsampleWatermark(ctx, watermark)
}
