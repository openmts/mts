package main

import (
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

func TestApplyDownsamplePolicyRequestDefaults(t *testing.T) {
	p := applyDownsamplePolicyRequestDefaults(mts.DownsamplePolicy{
		Name:              "p1",
		SourceDatabase:    "default",
		SourceMeasurement: "cpu",
		Interval:          time.Minute,
		Functions:         []mts.DownsampleFunction{{Function: "mean", Field: "usage"}},
		Enabled:           true,
	})
	if p.SourceRetention != "autogen" || p.TargetRetention != "autogen" {
		t.Fatalf("retention defaults = %#v", p)
	}
	if p.TargetMeasurement != "cpu_ds" {
		t.Fatalf("target measurement = %q", p.TargetMeasurement)
	}
	if p.RefreshInterval != time.Minute || p.Lookback != time.Minute {
		t.Fatalf("refresh/lookback = %v/%v", p.RefreshInterval, p.Lookback)
	}
	if p.BatchSize != 100 {
		t.Fatalf("batch size = %d", p.BatchSize)
	}
}
