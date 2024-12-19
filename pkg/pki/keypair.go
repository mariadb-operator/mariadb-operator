package pki

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
)

var (
	CACertKey  = "ca.crt"
	TLSCertKey = "tls.crt"
	TLSKeyKey  = "tls.key"

	defaultCAValidityDuration   = 4 * 365 * 24 * time.Hour
	defaultCertValidityDuration = 365 * 24 * time.Hour
)

type KeyPair struct {
	Key     *rsa.PrivateKey
	CertPEM []byte
	KeyPEM  []byte
}

func (k *KeyPair) Validate() error {
	if len(k.CertPEM) == 0 {
		return errors.New("certificate PEM is empty")
	}
	if len(k.KeyPEM) == 0 {
		return errors.New("private key PEM is empty")
	}

	if _, err := k.Certificates(); err != nil {
		return fmt.Errorf("error parsing certificates: %v", err)
	}
	if _, err := tls.X509KeyPair(k.CertPEM, k.KeyPEM); err != nil {
		return fmt.Errorf("invalid keypair: %v", err)
	}
	return nil
}

func (k *KeyPair) Certificates() ([]*x509.Certificate, error) {
	return ParseCertificates(k.CertPEM)
}

func (k *KeyPair) UpdateTLSSecret(secret *corev1.Secret) {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[TLSCertKey] = k.CertPEM
	secret.Data[TLSKeyKey] = k.KeyPEM
}

func KeyPairFromTLSSecret(secret *corev1.Secret) (*KeyPair, error) {
	if secret.Data == nil {
		return nil, errors.New("TLS Secret is empty")
	}
	if secret.Type != corev1.SecretTypeTLS {
		return nil, fmt.Errorf("invalid secret type, got: %v, want: %v", secret.Type, corev1.SecretTypeTLS)
	}

	certPEM := secret.Data[TLSCertKey]
	keyPEM := secret.Data[TLSKeyKey]
	return KeyPairFromPEM(certPEM, keyPEM)
}

func KeyPairFromPEM(certPEM, keyPEM []byte) (*KeyPair, error) {
	pemBlockKey, _ := pem.Decode(keyPEM)
	if pemBlockKey == nil {
		return nil, fmt.Errorf("Bad private key")
	}
	key, err := x509.ParsePKCS1PrivateKey(pemBlockKey.Bytes)
	if err != nil {
		return nil, fmt.Errorf("Error parsing PKCS1 private key: %v", err)
	}

	keyPair := &KeyPair{
		Key:     key,
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
	}
	if err := keyPair.Validate(); err != nil {
		return nil, fmt.Errorf("invalid keypair: %v", err)
	}

	return keyPair, nil
}

func createKeyPair(tpl *x509.Certificate, caKeyPair *KeyPair) (*KeyPair, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	parentCert := tpl
	parentKey := privateKey
	if caKeyPair != nil {
		caCerts, err := caKeyPair.Certificates()
		if err != nil {
			return nil, fmt.Errorf("error getting certificate: %v", err)
		}

		parentCert = caCerts[0] // assume first certificate in the CA bundle
		parentKey = caKeyPair.Key
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, tpl, parentCert, privateKey.Public(), parentKey)
	if err != nil {
		return nil, err
	}
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)

	return &KeyPair{
		Key:     privateKey,
		CertPEM: pemEncodeCertificate(certBytes),
		KeyPEM:  pemEncodePrivateKey(privateKeyBytes),
	}, nil
}
