package engine

import (
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestValidateDownsamplePolicyRejectsInvalidInput(t *testing.T) {
	policy := model.DownsamplePolicy{}
	if err := validateDownsamplePolicy(policy); err == nil {
		t.Fatal("validateDownsamplePolicy(empty) error = nil, want error")
	}

	policy = model.DownsamplePolicy{
		Name:              "raw-loop",
		SourceDatabase:    "metrics",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "metrics",
		TargetRetention:   "autogen",
		TargetMeasurement: "cpu",
		Interval:          time.Minute,
		Functions: []model.DownsampleFunction{
			{Function: "avg", Field: "usage"},
		},
		Delay:           time.Minute,
		RefreshInterval: time.Minute,
		Lookback:        time.Minute,
	}
	if err := validateDownsamplePolicy(policy); err == nil {
		t.Fatal("validateDownsamplePolicy(source=target) error = nil, want error")
	}
}

func TestNormalizeDownsamplePolicyFunctionsAndOutputNames(t *testing.T) {
	policy := model.DownsamplePolicy{
		Name:              "cpu_1m",
		SourceDatabase:    "metrics",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "metrics",
		TargetRetention:   "rp_1m",
		TargetMeasurement: "cpu",
		Interval:          time.Minute,
		Functions: []model.DownsampleFunction{
			{Function: "mean", Field: "usage"},
			{Function: "max", Field: "usage", As: "usage_peak"},
		},
		Delay:           time.Minute,
		RefreshInterval: time.Minute,
		Lookback:        3 * time.Minute,
		Enabled:         true,
	}
	got, err := normalizeDownsamplePolicy(policy)
	if err != nil {
		t.Fatalf("normalizeDownsamplePolicy() error = %v", err)
	}
	if got.Functions[0].Function != "avg" || got.Functions[0].As != "avg_usage" {
		t.Fatalf("first function = %#v, want avg_usage", got.Functions[0])
	}
	if got.Functions[1].As != "usage_peak" {
		t.Fatalf("second As = %q, want usage_peak", got.Functions[1].As)
	}
	if got.GroupByTags == nil {
		t.Fatal("GroupByTags = nil, want non-nil stable slice")
	}
	if got.BatchSize != defaultDownsampleBatchSize {
		t.Fatalf("BatchSize = %d, want default %d", got.BatchSize, defaultDownsampleBatchSize)
	}
	if got.CheckpointInterval != defaultDownsampleCheckpointInterval {
		t.Fatalf(
			"CheckpointInterval = %d, want default %d",
			got.CheckpointInterval,
			defaultDownsampleCheckpointInterval,
		)
	}
	if got.RunTimeout != defaultDownsampleRunTimeout {
		t.Fatalf("RunTimeout = %s, want %s", got.RunTimeout, defaultDownsampleRunTimeout)
	}
	if got.PolicyTagName != defaultDownsamplePolicyTagName {
		t.Fatalf("PolicyTagName = %q, want %q", got.PolicyTagName, defaultDownsamplePolicyTagName)
	}
}

func TestNormalizeDownsamplePolicyRejectsInvalidCommercialControls(t *testing.T) {
	valid := model.DownsamplePolicy{
		Name:              "cpu_1m",
		SourceDatabase:    "metrics",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "metrics",
		TargetRetention:   "rp_1m",
		TargetMeasurement: "cpu",
		Interval:          time.Minute,
		Functions: []model.DownsampleFunction{{
			Function: "avg",
			Field:    "usage",
		}},
		RefreshInterval: time.Minute,
		BatchSize:       128,
		RunTimeout:      time.Minute,
		PolicyTagName:   "mts_policy",
	}
	cases := map[string]model.DownsamplePolicy{
		"negative_initial_start": func() model.DownsamplePolicy {
			policy := valid
			policy.InitialStartTime = -1
			return policy
		}(),
		"negative_run_timeout": func() model.DownsamplePolicy {
			policy := valid
			policy.RunTimeout = -time.Second
			return policy
		}(),
		"negative_batch": func() model.DownsamplePolicy {
			policy := valid
			policy.BatchSize = -1
			return policy
		}(),
		"negative_checkpoint": func() model.DownsamplePolicy {
			policy := valid
			policy.CheckpointInterval = -1
			return policy
		}(),
		"empty_policy_tag": func() model.DownsamplePolicy {
			policy := valid
			policy.PolicyTagName = " "
			return policy
		}(),
	}
	for name, policy := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := normalizeDownsamplePolicy(policy); err == nil {
				t.Fatal("normalizeDownsamplePolicy() error = nil, want error")
			}
		})
	}
}

func TestDownsamplePolicyCompatibilityRejectsStructuralChanges(t *testing.T) {
	current := mustNormalizeDownsamplePolicyForTest(t, model.DownsamplePolicy{
		Name:              "cpu_1m",
		SourceDatabase:    "metrics",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "metrics",
		TargetRetention:   "rp_1m",
		TargetMeasurement: "cpu",
		Interval:          time.Minute,
		Functions: []model.DownsampleFunction{{
			Function: "avg",
			Field:    "usage",
		}},
		GroupByTags:      []string{"host"},
		InitialStartTime: int64(time.Minute),
		RefreshInterval:  time.Minute,
		PolicyTagName:    "policy",
		Enabled:          true,
	})
	compatible := current
	compatible.Enabled = false
	compatible.RefreshInterval = 2 * time.Minute
	if err := validateDownsamplePolicyUpdate(current, compatible, false); err != nil {
		t.Fatalf("validateDownsamplePolicyUpdate(compatible) error = %v", err)
	}
	changed := current
	changed.Functions = []model.DownsampleFunction{{
		Function: "max",
		Field:    "usage",
		As:       "max_usage",
	}}
	if err := validateDownsamplePolicyUpdate(current, changed, false); err == nil {
		t.Fatal("validateDownsamplePolicyUpdate(changed functions) error = nil, want error")
	}
	if err := validateDownsamplePolicyUpdate(current, changed, true); err != nil {
		t.Fatalf("validateDownsamplePolicyUpdate(allow reset) error = %v", err)
	}
}
