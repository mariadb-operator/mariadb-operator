package pki

import (
	"testing"
	"time"
)

func TestValidCACert(t *testing.T) {
	caName := "test-mariadb-operator"
	caKeyPair, err := CreateCACert(
		time.Now(),
		time.Now().Add(24*time.Hour),
		WithCAName(caName),
	)
	if err != nil {
		t.Fatalf("CA cert creation should succeed. Got error: %v", err)
	}

	valid, err := ValidCert(
		caKeyPair.CertPEM,
		caKeyPair.CertPEM,
		caKeyPair.KeyPEM,
		caName,
		time.Now(),
	)
	if err != nil {
		t.Fatalf("CA cert validation should succeed. Got error: %v", err)
	}
	if !valid {
		t.Fatal("Expected CA cert to be valid")
	}

	valid, err = ValidCert(
		caKeyPair.CertPEM,
		caKeyPair.CertPEM,
		caKeyPair.KeyPEM,
		caName,
		time.Now().Add(-1*time.Hour),
	)
	if err == nil {
		t.Fatalf("CA cert validation should return an error. Got nil")
	}
	if valid {
		t.Fatal("Expected CA cert to be invalid")
	}
}
