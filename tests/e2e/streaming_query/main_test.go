package main

import "testing"

func TestMainSmoke(t *testing.T) {
	main()
}

func TestRunWithDirRejectsInvalidPath(t *testing.T) {
	if err := runWithDir("bad\x00path"); err == nil {
		t.Fatal("runWithDir(invalid) error = nil, want error")
	}
}

func TestStreamingPoints(t *testing.T) {
	if points := streamingPoints(0); len(points) != 0 {
		t.Fatalf("streamingPoints(0) len = %d, want 0", len(points))
	}
	points := streamingPoints(2)
	if len(points) != 2 || points[1].Timestamp != 1 || points[1].Fields["value"].Float64 != 1 {
		t.Fatalf("streamingPoints(2) = %#v, want timestamps and values", points)
	}
}
