package pki

import (
	"testing"
	"time"
)

func TestCACert(t *testing.T) {
	caName := "test-mariadb-operator"
	caKeyPair, err := CreateCACert(
		time.Now(),
		time.Now().Add(24*time.Hour),
		WithCACommonName(caName),
		WithCAOrganization("test-org"),
	)
	if err != nil {
		t.Fatalf("CA cert creation should succeed. Got error: %v", err)
	}

	valid, err := ValidCert(
		caKeyPair.CertPEM,
		&caKeyPair.KeyPairPEM,
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
		&caKeyPair.KeyPairPEM,
		caName,
		time.Now().Add(-1*time.Hour),
	)
	if err == nil {
		t.Fatalf("CA cert validation should return an error. Got nil")
	}
	if valid {
		t.Fatal("Expected CA cert to be invalid")
	}

	valid, err = ValidCert(
		caKeyPair.CertPEM,
		&caKeyPair.KeyPairPEM,
		"foo",
		time.Now(),
	)
	if err == nil {
		t.Fatalf("CA cert validation should return an error. Got nil")
	}
	if valid {
		t.Fatal("Expected CA cert to be invalid")
	}
}

func TestCertPEM(t *testing.T) {
	caKeyPair, err := CreateCACert(
		time.Now(),
		time.Now().Add(24*time.Hour),
	)
	if err != nil {
		t.Fatalf("CA cert creation should succeed. Got error: %v", err)
	}

	fqdn := "mariadb-operator.default.svc.cluster.local"
	keyPairPEM, err := CreateCertPEM(
		caKeyPair,
		time.Now(),
		time.Now().Add(24*time.Hour),
		fqdn,
		[]string{
			"mariadb-operator",
			"mariadb-operator.default",
			fqdn,
		},
	)
	if err != nil {
		t.Fatalf("Certificate creation should succeed. Got error: %v", err)
	}

	valid, err := ValidCert(
		caKeyPair.CertPEM,
		keyPairPEM,
		fqdn,
		time.Now(),
	)
	if err != nil {
		t.Fatalf("Cert validation should succeed. Got error: %v", err)
	}
	if !valid {
		t.Fatal("Expected cert to be valid")
	}

	valid, err = ValidCert(
		caKeyPair.CertPEM,
		keyPairPEM,
		fqdn,
		time.Now().Add(-1*time.Hour),
	)
	if err == nil {
		t.Fatalf("Cert validation should return an error. Got nil")
	}
	if valid {
		t.Fatal("Expected cert to be invalid")
	}

	valid, err = ValidCert(
		caKeyPair.CertPEM,
		keyPairPEM,
		"foo",
		time.Now(),
	)
	if err == nil {
		t.Fatalf("Cert validation should return an error. Got nil")
	}
	if valid {
		t.Fatal("Expected cert to be invalid")
	}
}
