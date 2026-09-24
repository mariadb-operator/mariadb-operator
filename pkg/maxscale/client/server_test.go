package client

import (
	"encoding/json"
	"testing"
)

func TestServerParametersProtocol(t *testing.T) {
	tests := []struct {
		name         string
		params       ServerParameters
		wantProtocol bool
	}{
		{
			name: "protocol set",
			params: ServerParameters{
				Address:  "mariadb-0",
				Port:     3306,
				Protocol: "MariaDBBackend",
			},
			wantProtocol: true,
		},
		{
			name: "protocol omitted",
			params: ServerParameters{
				Address: "mariadb-0",
				Port:    3306,
			},
			wantProtocol: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := json.Marshal(tt.params)
			if err != nil {
				t.Fatalf("unexpected error marshaling: %v", err)
			}
			var rawMap map[string]json.RawMessage
			if err := json.Unmarshal(bytes, &rawMap); err != nil {
				t.Fatalf("unexpected error unmarshalling: %v", err)
			}
			if _, ok := rawMap["protocol"]; ok != tt.wantProtocol {
				t.Errorf("expected protocol present to be %v, got %v: %s", tt.wantProtocol, ok, string(bytes))
			}
		})
	}
}
