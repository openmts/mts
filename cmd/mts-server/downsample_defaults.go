package main

import (
	mts "github.com/openmts/mts"
)

// applyDownsamplePolicyRequestDefaults 为 Dashboard/API 创建请求补齐可商用默认值（POC 无兼容负担）。
func applyDownsamplePolicyRequestDefaults(policy mts.DownsamplePolicy) mts.DownsamplePolicy {
	if policy.SourceRetention == "" {
		policy.SourceRetention = "autogen"
	}
	if policy.TargetDatabase == "" {
		policy.TargetDatabase = policy.SourceDatabase
	}
	if policy.TargetRetention == "" {
		policy.TargetRetention = policy.SourceRetention
	}
	if policy.TargetMeasurement == "" && policy.SourceMeasurement != "" {
		policy.TargetMeasurement = policy.SourceMeasurement + "_ds"
	}
	if policy.Interval > 0 {
		if policy.RefreshInterval <= 0 {
			policy.RefreshInterval = policy.Interval
		}
		if policy.Lookback < 0 {
			policy.Lookback = 0
		}
		if policy.Lookback == 0 {
			policy.Lookback = policy.Interval
		}
	}
	if policy.BatchSize <= 0 {
		policy.BatchSize = 100
	}
	return policy
}
