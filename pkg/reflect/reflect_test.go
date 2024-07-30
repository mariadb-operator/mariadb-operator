package reflect

import (
	"testing"
	"unsafe"
)

func TestIsNil(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  bool
	}{
		{"nil interface", nil, true},
		{"nil pointer", (*int)(nil), true},
		{"nil slice", ([]int)(nil), true},
		{"nil map", (map[string]int)(nil), true},
		{"nil channel", (chan int)(nil), true},
		{"nil func", (func())(nil), true},
		{"nil unsafe pointer", (unsafe.Pointer)(nil), true},
		{"non-nil pointer", new(int), false},
		{"non-nil slice", []int{}, false},
		{"non-nil map", map[string]int{}, false},
		{"non-nil channel", make(chan int), false},
		{"non-nil func", func() {}, false},
		{"non-nil unsafe pointer", unsafe.Pointer(new(int)), false},
		{"non-nil interface", interface{}(0), false},
		{"non-nil int", 123, false},
		{"non-nil string", "hello", false},
		{"nil interface containing nil pointer", interface{}((*int)(nil)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNil(tt.input); got != tt.want {
				t.Errorf("IsNil(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
