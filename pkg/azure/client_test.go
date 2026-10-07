package azure

import (
	"net/http"
	"testing"
)

func TestGetTransport(t *testing.T) {
	tests := []struct {
		name          string
		opts          AzBlobOpts
		wantErr       bool
		wantTLSConfig bool
	}{
		{
			name:          "TLS disabled",
			opts:          AzBlobOpts{TLSEnabled: false},
			wantErr:       false,
			wantTLSConfig: false,
		},
		{
			name:          "TLS enabled without CA bundle",
			opts:          AzBlobOpts{TLSEnabled: true},
			wantErr:       false,
			wantTLSConfig: true,
		},
		{
			name:          "TLS enabled with an invalid CA bundle",
			opts:          AzBlobOpts{TLSEnabled: true, TLSCACert: []byte("not a PEM bundle")},
			wantErr:       true,
			wantTLSConfig: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport, err := getTransport(&tt.opts)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if !tt.wantTLSConfig {
				return
			}
			httpTransport, ok := transport.(*http.Transport)
			if !ok {
				t.Fatalf("expected an *http.Transport, got %T", transport)
			}
			// Without a CA bundle the system trust chain must still be in place.
			if httpTransport.TLSClientConfig == nil || httpTransport.TLSClientConfig.RootCAs == nil {
				t.Error("expected the root CA pool to be populated from the system trust chain")
			}
		})
	}
}
