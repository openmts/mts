package engine

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/openmts/mts/internal/model"
)

const defaultDownsampleBatchSize = 1024

const defaultDownsampleCheckpointInterval = 1

const defaultDownsampleRunTimeout = 5 * time.Minute

const defaultDownsamplePolicyTagName = "mts_downsample_policy"

var supportedDownsampleFunctions = map[string]struct{}{
	"avg":        {},
	"min":        {},
	"max":        {},
	"sum":        {},
	"count":      {},
	"first":      {},
	"last":       {},
	"rate":       {},
	"irate":      {},
	"increase":   {},
	"delta":      {},
	"difference": {},
	"derivative": {},
	"spread":     {},
	"mode":       {},
	"stddev":     {},
	"stdvar":     {},
	"top":        {},
	"bottom":     {},
	"median":     {},
}

func normalizeDownsamplePolicy(
	policy model.DownsamplePolicy,
) (model.DownsamplePolicy, error) {
	rawPolicyTagName := policy.PolicyTagName
	policy.Name = strings.TrimSpace(policy.Name)
	policy.SourceDatabase = strings.TrimSpace(policy.SourceDatabase)
	policy.SourceRetention = strings.TrimSpace(policy.SourceRetention)
	policy.SourceMeasurement = strings.TrimSpace(policy.SourceMeasurement)
	policy.TargetDatabase = strings.TrimSpace(policy.TargetDatabase)
	policy.TargetRetention = strings.TrimSpace(policy.TargetRetention)
	policy.TargetMeasurement = strings.TrimSpace(policy.TargetMeasurement)
	policy.PolicyTagName = strings.TrimSpace(policy.PolicyTagName)
	if rawPolicyTagName != "" && policy.PolicyTagName == "" {
		return model.DownsamplePolicy{}, fmt.Errorf("downsample policy tag name is empty")
	}
	policy.GroupByTags = cleanDownsampleStrings(policy.GroupByTags)
	policy.Functions = normalizeDownsampleFunctions(policy.Functions)
	policy = applyDownsamplePolicyDefaults(policy)
	if err := validateDownsamplePolicy(policy); err != nil {
		return model.DownsamplePolicy{}, err
	}
	return policy, nil
}

func applyDownsamplePolicyDefaults(policy model.DownsamplePolicy) model.DownsamplePolicy {
	if policy.BatchSize == 0 {
		policy.BatchSize = defaultDownsampleBatchSize
	}
	if policy.CheckpointInterval == 0 {
		policy.CheckpointInterval = defaultDownsampleCheckpointInterval
	}
	if policy.RunTimeout == 0 {
		policy.RunTimeout = defaultDownsampleRunTimeout
	}
	if policy.PolicyTagName == "" {
		policy.PolicyTagName = defaultDownsamplePolicyTagName
	}
	return policy
}

func normalizeDownsampleFunctions(
	functions []model.DownsampleFunction,
) []model.DownsampleFunction {
	out := make([]model.DownsampleFunction, 0, len(functions))
	for _, function := range functions {
		normalized := model.DownsampleFunction{
			Function: normalizeDownsampleFunction(function.Function),
			Field:    strings.TrimSpace(function.Field),
			As:       strings.TrimSpace(function.As),
		}
		if normalized.As == "" {
			normalized.As = downsampleOutputFieldName(normalized)
		}
		out = append(out, normalized)
	}
	return out
}

func normalizeDownsampleFunction(function string) string {
	normalized := strings.ToLower(strings.TrimSpace(function))
	if normalized == "mean" {
		return "avg"
	}
	return normalized
}

func validateDownsamplePolicy(policy model.DownsamplePolicy) error {
	if policy.Name == "" {
		return fmt.Errorf("downsample policy name is empty")
	}
	if policy.SourceDatabase == "" || policy.SourceRetention == "" ||
		policy.SourceMeasurement == "" {
		return fmt.Errorf("downsample source is incomplete")
	}
	if policy.TargetDatabase == "" || policy.TargetRetention == "" ||
		policy.TargetMeasurement == "" {
		return fmt.Errorf("downsample target is incomplete")
	}
	if sameDownsampleSourceAndTarget(policy) {
		return fmt.Errorf("downsample source and target must differ")
	}
	if policy.Interval <= 0 {
		return fmt.Errorf("downsample interval must be greater than zero")
	}
	if policy.Delay < 0 {
		return fmt.Errorf("downsample delay must be greater than or equal to zero")
	}
	if policy.RefreshInterval <= 0 {
		return fmt.Errorf("downsample refresh interval must be greater than zero")
	}
	if policy.Lookback < 0 {
		return fmt.Errorf("downsample lookback must be greater than or equal to zero")
	}
	if policy.InitialStartTime < 0 {
		return fmt.Errorf("downsample initial start time must be greater than or equal to zero")
	}
	if policy.RunTimeout < 0 {
		return fmt.Errorf("downsample run timeout must be greater than or equal to zero")
	}
	if policy.BatchSize <= 0 {
		return fmt.Errorf("downsample batch size must be greater than zero")
	}
	if policy.CheckpointInterval <= 0 {
		return fmt.Errorf("downsample checkpoint interval must be greater than zero")
	}
	if policy.PolicyTagName == "" {
		return fmt.Errorf("downsample policy tag name is empty")
	}
	if len(policy.Functions) == 0 {
		return fmt.Errorf("downsample functions are empty")
	}
	return validateDownsampleFunctions(policy.Functions)
}

func validateDownsamplePolicyUpdate(
	current model.DownsamplePolicy,
	next model.DownsamplePolicy,
	allowReplace bool,
) error {
	if allowReplace || downsamplePoliciesCompatible(current, next) {
		return nil
	}
	return fmt.Errorf("downsample policy %q has incompatible changes; reset policy before replace", current.Name)
}

func downsamplePoliciesCompatible(current model.DownsamplePolicy, next model.DownsamplePolicy) bool {
	return current.Name == next.Name &&
		current.SourceDatabase == next.SourceDatabase &&
		current.SourceRetention == next.SourceRetention &&
		current.SourceMeasurement == next.SourceMeasurement &&
		current.TargetDatabase == next.TargetDatabase &&
		current.TargetRetention == next.TargetRetention &&
		current.TargetMeasurement == next.TargetMeasurement &&
		current.Interval == next.Interval &&
		current.InitialStartTime == next.InitialStartTime &&
		current.PolicyTagName == next.PolicyTagName &&
		reflect.DeepEqual(current.Functions, next.Functions) &&
		reflect.DeepEqual(current.GroupByTags, next.GroupByTags)
}

func validateDownsampleFunctions(functions []model.DownsampleFunction) error {
	seenOutput := make(map[string]struct{}, len(functions))
	for _, function := range functions {
		if function.Field == "" {
			return fmt.Errorf("downsample function field is empty")
		}
		if _, ok := supportedDownsampleFunctions[function.Function]; !ok {
			return fmt.Errorf("downsample function %q is not supported", function.Function)
		}
		if function.As == "" {
			return fmt.Errorf("downsample output field is empty")
		}
		if _, ok := seenOutput[function.As]; ok {
			return fmt.Errorf("downsample output field %q is duplicated", function.As)
		}
		seenOutput[function.As] = struct{}{}
	}
	return nil
}

func sameDownsampleSourceAndTarget(policy model.DownsamplePolicy) bool {
	return policy.SourceDatabase == policy.TargetDatabase &&
		policy.SourceRetention == policy.TargetRetention &&
		policy.SourceMeasurement == policy.TargetMeasurement
}

func downsampleOutputFieldName(function model.DownsampleFunction) string {
	if function.Function == "" || function.Field == "" {
		return ""
	}
	return function.Function + "_" + function.Field
}

func alignDownsampleWindow(timestamp int64, interval time.Duration) int64 {
	if interval <= 0 {
		return timestamp
	}
	size := int64(interval)
	if timestamp >= 0 {
		return timestamp / size * size
	}
	return ((timestamp+1)/size - 1) * size
}

func cleanDownsampleStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		clean := strings.TrimSpace(value)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out
}
