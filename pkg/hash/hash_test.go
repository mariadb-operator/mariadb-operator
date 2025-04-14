package hash

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestStruct struct {
	Field1 string `json:"field1"`
	Field2 int    `json:"field2"`
}

func TestHashJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    string
		wantErr bool
	}{
		{
			name:    "Struct",
			input:   TestStruct{Field1: "value1", Field2: 42},
			want:    Hash(`{"field1":"value1","field2":42}`),
			wantErr: false,
		},
		{
			name:    "Slice of structs",
			input:   []TestStruct{{Field1: "value1", Field2: 42}, {Field1: "value2", Field2: 84}},
			want:    Hash(`[{"field1":"value1","field2":42},{"field1":"value2","field2":84}]`),
			wantErr: false,
		},
		{
			name:    "Empty struct",
			input:   TestStruct{},
			want:    Hash(`{"field1":"","field2":0}`),
			wantErr: false,
		},
		{
			name: "Map",
			input: map[string]string{
				"a": "1",
				"d": "4",
				"b": "2",
				"e": "5",
				"c": "3",
			},
			want:    Hash(`{"a":"1","b":"2","c":"3","d":"4","e":"5"}`),
			wantErr: false,
		},
		{
			name:    "Nil",
			input:   nil,
			want:    Hash("null"),
			wantErr: false,
		},
		{
			name:    "Invalid input type",
			input:   make(chan int),
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HashJSON(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
