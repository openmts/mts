package sstable

import "testing"

func TestConstStepWindow(t *testing.T) {
	// base=0 step=1e9 count=100, query [5e9, 14e9] => index 5..14 inclusive => [5,15)
	start, end := constStepWindow(0, 1_000_000_000, 100, Query{Start: 5_000_000_000, End: 14_000_000_000})
	if start != 5 || end != 15 {
		t.Fatalf("window = [%d,%d), want [5,15)", start, end)
	}
	start, end = constStepWindow(0, 1_000_000_000, 10, Query{Start: 100, End: 50})
	if start != 0 || end != 0 {
		t.Fatalf("invalid range window = [%d,%d)", start, end)
	}
}

func TestSortedTimestampWindow(t *testing.T) {
	ts := []int64{10, 20, 30, 40, 50}
	lo, hi := sortedTimestampWindow(ts, Query{Start: 20, End: 40})
	if lo != 1 || hi != 4 {
		t.Fatalf("window=[%d,%d) want [1,4)", lo, hi)
	}
}

func TestDecodeConstStepTimestampsWindow(t *testing.T) {
	payload := appendConstStepTimestamps(nil, 100, 10)
	got, start, err := decodeConstStepTimestampsWindow(payload, 20, Query{Start: 150, End: 180})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	// base 100 step 10: index0=100 ... index5=150 ... index8=180
	if start != 5 {
		t.Fatalf("start=%d want 5", start)
	}
	if len(got) != 4 || got[0] != 150 || got[3] != 180 {
		t.Fatalf("got=%v", got)
	}
}
