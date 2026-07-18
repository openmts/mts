package sstable

import "testing"

func TestResolveValuePageSamplesDefaultsAndClamps(t *testing.T) {
	if got := resolveValuePageSamples(0); got != defaultValueBlockPageSamples {
		t.Fatalf("default = %d, want %d", got, defaultValueBlockPageSamples)
	}
	if got := resolveValuePageSamples(-1); got != defaultValueBlockPageSamples {
		t.Fatalf("negative = %d, want default", got)
	}
	if got := resolveValuePageSamples(1); got != minValueBlockPageSamples {
		t.Fatalf("too small = %d, want min %d", got, minValueBlockPageSamples)
	}
	if got := resolveValuePageSamples(2048); got != 2048 {
		t.Fatalf("explicit = %d, want 2048", got)
	}
	if got := resolveValuePageSamples(maxValueBlockPageSamples + 1); got != maxValueBlockPageSamples {
		t.Fatalf("too large = %d, want max %d", got, maxValueBlockPageSamples)
	}
}

func TestValuePageCountUsesConfiguredPageSamples(t *testing.T) {
	if got := valuePageCount(2500, 1024); got != 3 {
		t.Fatalf("count = %d, want 3", got)
	}
	if got := valuePageCount(0, 1024); got != 0 {
		t.Fatalf("empty count = %d, want 0", got)
	}
}
