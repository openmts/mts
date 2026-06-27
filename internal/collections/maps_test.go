package collections

import (
	"reflect"
	"testing"
)

func TestCloneMapPreservesEmptyMap(t *testing.T) {
	input := map[string]string{}
	cloned := CloneMap(input)
	if cloned == nil {
		t.Fatal("CloneMap(empty) = nil, want empty map")
	}
	cloned["k"] = "v"
	if _, ok := input["k"]; ok {
		t.Fatal("CloneMap result aliases input")
	}
}

func TestCloneMapNilIfEmptyReturnsNilForEmptyMap(t *testing.T) {
	if cloned := CloneMapNilIfEmpty(map[string]string{}); cloned != nil {
		t.Fatalf("CloneMapNilIfEmpty(empty) = %#v, want nil", cloned)
	}
	input := map[string]int{"b": 2}
	cloned := CloneMapNilIfEmpty(input)
	if !reflect.DeepEqual(cloned, input) {
		t.Fatalf("CloneMapNilIfEmpty() = %#v, want %#v", cloned, input)
	}
	cloned["b"] = 3
	if input["b"] != 2 {
		t.Fatal("CloneMapNilIfEmpty result aliases input")
	}
}

func TestCloneSliceCopiesInput(t *testing.T) {
	input := []int{1}
	cloned := CloneSlice(input)
	if !reflect.DeepEqual(cloned, input) {
		t.Fatalf("CloneSlice() = %#v, want %#v", cloned, input)
	}
	cloned[0] = 2
	if input[0] != 1 {
		t.Fatal("CloneSlice result aliases input")
	}
}

func TestCloneSlicePreservesEmptySlice(t *testing.T) {
	cloned := CloneSlice([]int{})
	if cloned == nil {
		t.Fatal("CloneSlice(empty) = nil, want empty slice")
	}
}

func TestCloneSliceNilIfEmptyReturnsNilForEmptySlice(t *testing.T) {
	if cloned := CloneSliceNilIfEmpty([]int{}); cloned != nil {
		t.Fatalf("CloneSliceNilIfEmpty(empty) = %#v, want nil", cloned)
	}
	input := []int{1, 2}
	cloned := CloneSliceNilIfEmpty(input)
	if !reflect.DeepEqual(cloned, input) {
		t.Fatalf("CloneSliceNilIfEmpty() = %#v, want %#v", cloned, input)
	}
	cloned[0] = 9
	if input[0] != 1 {
		t.Fatal("CloneSliceNilIfEmpty result aliases input")
	}
}

func TestSortedKeysReturnsSortedStringKeys(t *testing.T) {
	keys := SortedKeys(map[string]int{"b": 2, "a": 1, "c": 3})
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("SortedKeys() = %#v, want %#v", keys, want)
	}
}
