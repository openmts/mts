package main

import (
	"net/url"
	"testing"

	mts "github.com/openmts/mts"
)

func TestFilterDownsampleStatuses(t *testing.T) {
	statuses := []mts.DownsamplePolicyStatus{
		{PolicyName: "ok", LastError: "", Active: false, LagSeconds: 0},
		{PolicyName: "err-cpu", LastError: "boom", Active: false, LagSeconds: 3},
		{PolicyName: "busy", LastError: "", Active: true, LagSeconds: 0},
		{PolicyName: "lag", LastError: "", Active: false, LagSeconds: 120},
	}
	got := filterDownsampleStatuses(statuses, url.Values{"health": []string{"error"}})
	if len(got) != 1 || got[0].PolicyName != "err-cpu" {
		t.Fatalf("error filter = %#v", got)
	}
	got = filterDownsampleStatuses(statuses, url.Values{"health": []string{"active"}})
	if len(got) != 1 || got[0].PolicyName != "busy" {
		t.Fatalf("active filter = %#v", got)
	}
	got = filterDownsampleStatuses(statuses, url.Values{"health": []string{"lagging"}, "min_lag_seconds": []string{"60"}})
	if len(got) != 1 || got[0].PolicyName != "lag" {
		t.Fatalf("lagging filter = %#v", got)
	}
	got = filterDownsampleStatuses(statuses, url.Values{"q": []string{"cpu"}})
	if len(got) != 1 || got[0].PolicyName != "err-cpu" {
		t.Fatalf("q filter = %#v", got)
	}
}
