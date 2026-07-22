package main

import (
	"testing"

	mts "github.com/openmts/mts"
)

func TestSummarizeDownsampleStatuses(t *testing.T) {
	sum := summarizeDownsampleStatuses([]mts.DownsamplePolicyStatus{
		{PolicyName: "a", Enabled: true, Active: true, LagSeconds: 0},
		{PolicyName: "b", Enabled: true, LastError: "x", LagSeconds: 5},
		{PolicyName: "c", Enabled: false, LagSeconds: 30},
	})
	if sum.Total != 3 || sum.Enabled != 2 || sum.Active != 1 || sum.Error != 1 || sum.Lagging != 2 || sum.MaxLagSeconds != 30 {
		t.Fatalf("summary = %#v", sum)
	}
}
