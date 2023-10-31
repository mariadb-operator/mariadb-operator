package pki

import (
	"testing"
	"time"
)

func TestCACert(t *testing.T) {
	caName := "test-mariadb-operator"
	caKeyPair, err := CreateCA(
		WithCommonName(caName),
		WithOrganization("test-org"),
		WithNotBefore(time.Now()),
		WithNotAfter(time.Now().Add(24*time.Hour)),
	)
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
}

func TestCert(t *testing.T) {
	caKeyPair, err := CreateCA()
	if err != nil {
		t.Fatalf("CA cert creation should succeed. Got error: %v", err)
	}

	fqdn := "mariadb-operator.default.svc.cluster.local"
	keyPairPEM, err := CreateCert(
		caKeyPair,
		WithCommonName(fqdn),
		WithDNSNames([]string{
			"mariadb-operator",
			"mariadb-operator.default",
			fqdn,
		}),
		WithNotBefore(time.Now()),
		WithNotAfter(time.Now().Add(24*time.Hour)),
	)
	if err != nil {
		t.Fatalf("Certificate creation should succeed. Got error: %v", err)
	}

	valid, err := ValidCert(caKeyPair, keyPairPEM, fqdn, time.Now())
	if err != nil {
		t.Fatalf("Cert validation should succeed. Got error: %v", err)
	}
	if !valid {
		t.Fatal("Expected cert to be valid")
	}

	valid, err = ValidCert(caKeyPair, keyPairPEM, fqdn, time.Now().Add(-1*time.Hour))
	if err == nil {
		t.Fatalf("Cert validation should return an error. Got nil")
	}
	if valid {
		t.Fatal("Expected cert to be invalid")
	}

	valid, err = ValidCert(caKeyPair, keyPairPEM, "foo", time.Now())
	if err == nil {
		t.Fatalf("Cert validation should return an error. Got nil")
	}
	if valid {
		t.Fatal("Expected cert to be invalid")
	}
}
