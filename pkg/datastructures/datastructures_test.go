package datastructures

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestDataStructures(t *testing.T) {
	idx := newIndex("a", "b", "b", "c", "d", "e")
	if !reflect.DeepEqual(idx, newIndex("a", "b", "c", "d", "e")) {
		t.Error("expeting index to remove duplicates")
	}

	item, err := Get(idx, "b")
	expectedItem := "b"
	if err != nil {
		t.Errorf("expecting error to not have occurred: %v", err)
	}
	if item != expectedItem {
		t.Errorf("expeting item to be %s, got %s", "a", item)
	}

	keys := Keys(idx)
	expectedKeys := []string{"a", "b", "c", "d", "e"}
	sort.Strings(keys)
	sort.Strings(expectedKeys)
	if !reflect.DeepEqual(keys, expectedKeys) {
		t.Errorf("expecting keys to be %v, got: %v", expectedKeys, keys)
	}

	exists := AllExists(idx, "a", "b", "c")
	expectedExists := true
	if exists != expectedExists {
		t.Errorf("expecting exists to be %v, got: %v", expectedExists, exists)
	}

	exists = AllExists(idx, "a", "b", "c", "z")
	expectedExists = false
	if exists != expectedExists {
		t.Errorf("expecting exists to be %v, got: %v", expectedExists, exists)
	}

	filteredIdx := Filter(idx, "a", "b", "c")
	expectedFilteredIdx := newIndex("a", "b", "c")
	if !reflect.DeepEqual(filteredIdx, expectedFilteredIdx) {
		t.Errorf("expecting filtered index to be %v, got: %v", expectedFilteredIdx, filteredIdx)
	}
}

func TestDataStructuresDiff(t *testing.T) {
	tests := []struct {
		name     string
		current  Index[string]
		previous Index[string]
		wantDiff DiffResult
	}{
		{
			name:     "no diff",
			current:  newIndex("a", "b"),
			previous: newIndex("a", "b"),
			wantDiff: DiffResult{
				Rest: []string{"a", "b"},
			},
		},
		{
			name:     "added",
			current:  newIndex("a", "b", "c", "d"),
			previous: newIndex("a", "b"),
			wantDiff: DiffResult{
				Added: []string{"c", "d"},
				Rest:  []string{"a", "b"},
			},
		},
		{
			name:     "deleted",
			current:  newIndex(),
			previous: newIndex("a", "b"),
			wantDiff: DiffResult{
				Deleted: []string{"a", "b"},
			},
		},
		{
			name:     "added and deleted",
			current:  newIndex("b", "d", "e", "f"),
			previous: newIndex("a", "b", "c"),
			wantDiff: DiffResult{
				Added:   []string{"d", "e", "f"},
				Deleted: []string{"a", "c"},
				Rest:    []string{"b"},
			},
		},
		{
			name:     "no intersection",
			current:  newIndex("d", "e", "f"),
			previous: newIndex("a", "b", "c"),
			wantDiff: DiffResult{
				Added:   []string{"d", "e", "f"},
				Deleted: []string{"a", "b", "c"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff := Diff(tt.current, tt.previous)
			if !reflect.DeepEqual(tt.wantDiff, diff) {
				t.Errorf("expecting config to be:\n%v\ngot:\n%v\n", tt.wantDiff, diff)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name      string
		slices    [][]string
		wantSlice []string
	}{
		{
			name: "empty",
			slices: [][]string{
				{},
				nil,
			},
			wantSlice: nil,
		},
		{
			name: "half empty",
			slices: [][]string{
				{"a", "b", "c"},
				{},
			},
			wantSlice: []string{"a", "b", "c"},
		},
		{
			name: "full",
			slices: [][]string{
				{"a", "b", "c"},
				{"d", "e"},
			},
			wantSlice: []string{"a", "b", "c", "d", "e"},
		},
		{
			name: "multiple",
			slices: [][]string{
				{"a", "b", "c"},
				{"d", "e"},
				{"f", "g", "h"},
				{"i"},
			},
			wantSlice: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slices := Merge(tt.slices...)
			if !reflect.DeepEqual(slices, tt.wantSlice) {
				t.Errorf("expecting merged slices to be:\n%v\ngot:\n%v\n", tt.wantSlice, slices)
			}
		})
	}
}

func TestUnique(t *testing.T) {
	tests := []struct {
		name         string
		elements     []string
		wantElements []string
	}{
		{
			name:         "empty",
			elements:     nil,
			wantElements: nil,
		},
		{
			name:         "some repeated",
			elements:     []string{"a", "b", "b", "c"},
			wantElements: []string{"a", "b", "c"},
		},
		{
			name:         "multiple repeated",
			elements:     []string{"a", "b", "b", "c", "d", "d", "d", "e"},
			wantElements: []string{"a", "b", "c", "d", "e"},
		},
		{
			name:         "all different",
			elements:     []string{"a", "b", "c", "d", "e"},
			wantElements: []string{"a", "b", "c", "d", "e"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elements := Unique(tt.wantElements...)
			if !reflect.DeepEqual(elements, tt.wantElements) {
				t.Errorf("expecting unique elements to be:\n%v\ngot:\n%v\n", tt.wantElements, elements)
			}
		})
	}
}

func TestAny(t *testing.T) {
	tests := []struct {
		name     string
		elements []string
		fn       func(string) bool
		wantBool bool
	}{
		{
			name:     "empty",
			elements: nil,
			fn:       nil,
			wantBool: false,
		},
		{
			name: "no match",
			elements: []string{
				"--single-transaction",
				"--events",
				"--routines",
			},
			fn:       func(s string) bool { return strings.HasPrefix(s, "--databases") },
			wantBool: false,
		},
		{
			name: "single match",
			elements: []string{
				"--single-transaction",
				"--events",
				"--routines",
				"--databases foo",
			},
			fn:       func(s string) bool { return strings.HasPrefix(s, "--databases") },
			wantBool: true,
		},
		{
			name: "multiple match",
			elements: []string{
				"--single-transaction",
				"--databases foo",
				"--events",
				"--routines",
				"--databases foo",
			},
			fn:       func(s string) bool { return strings.HasPrefix(s, "--databases") },
			wantBool: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBool := Any(tt.elements, tt.fn)
			if !reflect.DeepEqual(gotBool, tt.wantBool) {
				t.Errorf("expecting Any returned value to be:\n%v\ngot:\n%v\n", tt.wantBool, gotBool)
			}
		})
	}
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name         string
		elements     []string
		fn           func(string) bool
		wantElements []string
	}{
		{
			name:         "empty",
			elements:     nil,
			fn:           nil,
			wantElements: nil,
		},
		{
			name: "no match",
			elements: []string{
				"--single-transaction",
				"--events",
				"--routines",
			},
			fn: func(s string) bool { return strings.HasPrefix(s, "--databases") },
			wantElements: []string{
				"--single-transaction",
				"--events",
				"--routines",
			},
		},
		{
			name: "remove first",
			elements: []string{
				"--databases foo",
				"--single-transaction",
				"--events",
				"--routines",
			},
			fn: func(s string) bool { return strings.HasPrefix(s, "--databases") },
			wantElements: []string{
				"--single-transaction",
				"--events",
				"--routines",
			},
		},
		{
			name: "remove middle",
			elements: []string{
				"--single-transaction",
				"--databases foo",
				"--events",
				"--routines",
			},
			fn: func(s string) bool { return strings.HasPrefix(s, "--databases") },
			wantElements: []string{
				"--single-transaction",
				"--events",
				"--routines",
			},
		},
		{
			name: "remove last",
			elements: []string{
				"--single-transaction",
				"--events",
				"--routines",
				"--databases foo",
			},
			fn: func(s string) bool { return strings.HasPrefix(s, "--databases") },
			wantElements: []string{
				"--single-transaction",
				"--events",
				"--routines",
			},
		},
		{
			name: "multiple match",
			elements: []string{
				"--databases foo",
				"--single-transaction",
				"--databases foo",
				"--events",
				"--routines",
				"--databases foo",
			},
			fn: func(s string) bool { return strings.HasPrefix(s, "--databases") },
			wantElements: []string{
				"--single-transaction",
				"--events",
				"--routines",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elements := Remove(tt.elements, tt.fn)
			if !reflect.DeepEqual(elements, tt.wantElements) {
				t.Errorf("expecting Remove returned value to be:\n%v\ngot:\n%v\n", tt.wantElements, elements)
			}
		})
	}
}

func newIndex(items ...string) Index[string] {
	return NewIndex[string](items, func(s string) string {
		return s
	})
}
