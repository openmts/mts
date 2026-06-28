package main

import (
	"runtime"
	"testing"
)

func TestMainSmoke(t *testing.T) {
	main()
}

func TestRunRejectsInvalidTempDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 不支持 /dev/null 作为无效临时目录路径")
	}
	t.Setenv("TMPDIR", "/dev/null")
	if err := run(); err == nil {
		t.Fatal("run(invalid temp dir) error = nil, want error")
	}
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
