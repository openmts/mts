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

func TestReadAmpPoint(t *testing.T) {
	point := readAmpPoint(7)
	if point.Measurement != "read_amp" || point.Timestamp != 7 || point.Fields["value"].Int64 != 7 {
		t.Fatalf("readAmpPoint() = %#v, want timestamp value", point)
	}
}
