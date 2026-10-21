package azure

import (
	"crypto/x509"
	"net/http"
	"testing"

	"github.com/mariadb-operator/mariadb-operator/v26/pkg/pki"
)

func TestGetTransport(t *testing.T) {
	ca, err := pki.CreateCA(pki.WithCommonName("test-ca"))
	if err != nil {
		t.Fatalf("error creating CA: %v", err)
	}
	systemPool, err := x509.SystemCertPool()
	if err != nil {
		systemPool = x509.NewCertPool()
	}
	// A CA bundle is added to the system trust chain.
	caPool := systemPool.Clone()
	if ok := caPool.AppendCertsFromPEM(ca.CertPEM); !ok {
		t.Fatal("error adding CA to pool")
	}

	tests := []struct {
		name     string
		opts     AzBlobOpts
		wantErr  bool
		wantPool *x509.CertPool
	}{
		{
			name:     "TLS disabled",
			opts:     AzBlobOpts{TLSEnabled: false},
			wantErr:  false,
			wantPool: nil,
		},
		{
			name:     "TLS enabled without CA bundle",
			opts:     AzBlobOpts{TLSEnabled: true},
			wantErr:  false,
			wantPool: systemPool,
		},
		{
			name:     "TLS enabled with a CA bundle",
			opts:     AzBlobOpts{TLSEnabled: true, TLSCACert: ca.CertPEM},
			wantErr:  false,
			wantPool: caPool,
		},
		{
			name:     "TLS enabled with an empty CA bundle",
			opts:     AzBlobOpts{TLSEnabled: true, TLSCACert: []byte{}},
			wantErr:  true,
			wantPool: nil,
		},
		{
			name:     "TLS enabled with an invalid CA bundle",
			opts:     AzBlobOpts{TLSEnabled: true, TLSCACert: []byte("not a PEM bundle")},
			wantErr:  true,
			wantPool: nil,
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
			if tt.wantPool == nil {
				return
			}
			httpTransport, ok := transport.(*http.Transport)
			if !ok {
				t.Fatalf("expected an *http.Transport, got %T", transport)
			}
			if httpTransport.TLSClientConfig == nil || httpTransport.TLSClientConfig.RootCAs == nil {
				t.Fatal("expected the root CA pool to be set")
			}
			if !httpTransport.TLSClientConfig.RootCAs.Equal(tt.wantPool) {
				t.Error("unexpected root CA pool")
			}
		})
	}
}
