package ptr

import (
	"testing"

	"k8s.io/utils/ptr"
)

func TestPtr(t *testing.T) {
	tests := []struct {
		name string
		ptrs []*string
		def  string
		want string
	}{
		{
			name: "empty",
			ptrs: []*string{},
			def:  "default",
			want: "default",
		},
		{
			name: "all nil",
			ptrs: []*string{nil, nil, nil},
			def:  "default",
			want: "default",
		},
		{
			name: "some non nil",
			ptrs: []*string{nil, ptr.To("foo"), nil},
			def:  "default",
			want: "foo",
		},
		{
			name: "all non nil",
			ptrs: []*string{ptr.To("bar"), ptr.To("foo"), ptr.To("ptr")},
			def:  "default",
			want: "bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Deref(tt.ptrs, tt.def)
			if got != tt.want {
				t.Errorf("unexpected returned value, got \"%s\" want \"%s\"", got, tt.want)
			}
		})
	}
}
