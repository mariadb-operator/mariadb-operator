package pki

import (
	"testing"
	"time"
)

func TestCACert(t *testing.T) {
	caName := "test-mariadb-operator"
	x509Opts := []X509Opt{
		WithCommonName(caName),
		WithOrganization("test-org"),
		WithNotBefore(time.Now()),
		WithNotAfter(time.Now().Add(24 * time.Hour)),
	}
	caKeyPair, err := CreateCA(x509Opts...)
	if err != nil {
		t.Fatalf("CA cert creation should succeed. Got error: %v", err)
	}

	valid, err := ValidCACert(caKeyPair, caName, time.Now())
	if err != nil {
		t.Fatalf("CA cert validation should succeed. Got error: %v", err)
	}
	if !valid {
		t.Fatal("Expected CA cert to be valid")
	}

	valid, err = ValidCACert(caKeyPair, caName, time.Now().Add(-1*time.Hour))
	if err == nil {
		t.Fatalf("CA cert validation should return an error. Got nil")
	}
	if valid {
		t.Fatal("Expected CA cert to be invalid")
	}

	valid, err = ValidCACert(caKeyPair, "foo", time.Now())
	if err == nil {
		t.Fatalf("CA cert validation should return an error. Got nil")
	}
	if valid {
		t.Fatal("Expected CA cert to be invalid")
	}

	caKeyPair, err = CreateCA(x509Opts...)
	if err != nil {
		t.Fatalf("CA cert renewal should succeed. Got error: %v", err)
	}

	valid, err = ValidCACert(caKeyPair, caName, time.Now())
	if err != nil {
		t.Fatalf("CA cert validation should succeed after renewal. Got error: %v", err)
	}
	if !valid {
		t.Fatal("Expected CA cert to be valid after renewal")
	}
}

func TestCert(t *testing.T) {
	caKeyPair, err := CreateCA()
	if err != nil {
		t.Fatalf("CA cert creation should succeed. Got error: %v", err)
	}
	caCerts, err := caKeyPair.Certificates()
	if err != nil {
		t.Fatalf("Unable to get CA certificates: %v", err)
	}

	commonName := "mariadb-operator.default.svc"
	x509Opts := []X509Opt{
		WithCommonName(commonName),
		WithDNSNames([]string{
			"mariadb-operator",
			"mariadb-operator.default",
			commonName,
		}),
		WithNotBefore(time.Now()),
		WithNotAfter(time.Now().Add(24 * time.Hour)),
	}
	keyPairPEM, err := CreateCert(caKeyPair, x509Opts...)
	if err != nil {
		t.Fatalf("Certificate creation should succeed. Got error: %v", err)
	}

	valid, err := ValidCert(caCerts, keyPairPEM, commonName, time.Now())
	if err != nil {
		t.Fatalf("Cert validation should succeed. Got error: %v", err)
	}
	if !valid {
		t.Fatal("Expected cert to be valid")
	}

	valid, err = ValidCert(caCerts, keyPairPEM, commonName, time.Now().Add(-1*time.Hour))
	if err == nil {
		t.Fatalf("Cert validation should return an error. Got nil")
	}
	if valid {
		t.Fatal("Expected cert to be invalid")
	}

	valid, err = ValidCert(caCerts, keyPairPEM, "foo", time.Now())
	if err == nil {
		t.Fatalf("Cert validation should return an error. Got nil")
	}
	if valid {
		t.Fatal("Expected cert to be invalid")
	}

	keyPairPEM, err = CreateCert(caKeyPair, x509Opts...)
	if err != nil {
		t.Fatalf("Certificate renewal should succeed. Got error: %v", err)
	}

	valid, err = ValidCert(caCerts, keyPairPEM, commonName, time.Now())
	if err != nil {
		t.Fatalf("Cert validation should succeed after renewal. Got error: %v", err)
	}
	if !valid {
		t.Fatal("Expected cert to be valid")
	}
}

func TestParseCertificate(t *testing.T) {
	tests := []struct {
		name      string
		certBytes []byte
		wantErr   bool
	}{
		{
			name:      "Valid cert",
			certBytes: []byte(testTLSCert),
			wantErr:   false,
		},
		{
			name:      "Valid cert bundle",
			certBytes: []byte(testTLSCertBundle),
			wantErr:   false,
		},
		{
			name:      "No block cert",
			certBytes: []byte(testTLSCertNoBlock),
			wantErr:   true,
		},
		{
			name:      "Invalid cert",
			certBytes: []byte("foo"),
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCertificate(tt.certBytes)
			if tt.wantErr && err == nil {
				t.Fatalf("Expecting error to be non nil when parsing '%s'", tt.name)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Expecting error to be nil when parsing '%s'. Got: %v", tt.name, err)
			}
		})
	}
}
