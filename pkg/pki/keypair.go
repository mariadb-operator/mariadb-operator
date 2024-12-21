package pki

import (
	"crypto"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

var (
	// CACertKey is the key used to store the CA certificate in a secret.
	CACertKey = "ca.crt"
	// TLSCertKey is the key used to store the TLS certificate in a secret.
	TLSCertKey = "tls.crt"
	// TLSKeyKey is the key used to store the TLS private key in a secret.
	TLSKeyKey = "tls.key"
)

// KeyPairOpt is a function type used to configure a KeyPair.
type KeyPairOpt func(*KeyPair)

// WithSupportedPrivateKeys returns a KeyPairOpt that sets the supported private keys for a KeyPair.
func WithSupportedPrivateKeys(pks ...PrivateKey) KeyPairOpt {
	return func(k *KeyPair) {
		k.SupportedPrivateKeys = pks
	}
}

// KeyPair represents a TLS key pair with its certificate and private key.
type KeyPair struct {
	// CertPEM is the PEM-encoded certificate.
	CertPEM []byte
	// KeyPEM is the PEM-encoded private key.
	KeyPEM []byte
	// SupportedPrivateKeys is a list of supported private key types.
	SupportedPrivateKeys []PrivateKey
}

// NewKeyPair creates a new KeyPair with the given certificate and private key PEM data.
// Additional options can be provided to configure the KeyPair.
func NewKeyPair(certPEM, keyPEM []byte, opts ...KeyPairOpt) (*KeyPair, error) {
	k := KeyPair{
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
		SupportedPrivateKeys: []PrivateKey{
			PrivateKeyTypeECDSA,
		},
	}
	for _, setOpt := range opts {
		setOpt(&k)
	}
	if err := k.Validate(); err != nil {
		return nil, fmt.Errorf("invalid keypair: %v", err)
	}
	return &k, nil
}

// Validate checks if the KeyPair is valid by ensuring the certificate and private key are not empty
// and can be parsed correctly.
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
	if _, err := k.PrivateKey(); err != nil {
		return fmt.Errorf("error parsing private key: %v", err)
	}
	if _, err := tls.X509KeyPair(k.CertPEM, k.KeyPEM); err != nil {
		return fmt.Errorf("invalid keypair: %v", err)
	}
	return nil
}

// Certificates parses and returns the certificates from the CertPEM field.
func (k *KeyPair) Certificates() ([]*x509.Certificate, error) {
	return ParseCertificates(k.CertPEM)
}

// PrivateKey parses and returns the private key from the KeyPEM field.
func (k *KeyPair) PrivateKey() (crypto.Signer, error) {
	return ParsePrivateKey(k.KeyPEM, k.SupportedPrivateKeys)
}

// UpdateTLSSecret updates the given Kubernetes secret with the certificate and private key from the KeyPair.
func (k *KeyPair) UpdateTLSSecret(secret *corev1.Secret) {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[TLSCertKey] = k.CertPEM
	secret.Data[TLSKeyKey] = k.KeyPEM
}

// NewKeyPairFromTLSSecret creates a new KeyPair from the given Kubernetes TLS secret.
// Additional options can be provided to configure the KeyPair.
func NewKeyPairFromTLSSecret(secret *corev1.Secret, opts ...KeyPairOpt) (*KeyPair, error) {
	if secret.Data == nil {
		return nil, errors.New("TLS Secret is empty")
	}
	if secret.Type != corev1.SecretTypeTLS {
		return nil, fmt.Errorf("invalid secret type, got: %v, want: %v", secret.Type, corev1.SecretTypeTLS)
	}

	certPEM := secret.Data[TLSCertKey]
	keyPEM := secret.Data[TLSKeyKey]
	return NewKeyPair(certPEM, keyPEM, opts...)
}

// NewKeyPairFromTemplate creates a new KeyPair from the given certificate template and CA KeyPair.
// Additional options can be provided to configure the KeyPair.
func NewKeyPairFromTemplate(tpl *x509.Certificate, caKeyPair *KeyPair, opts ...KeyPairOpt) (*KeyPair, error) {
	privateKey, err := GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("error generating private key: %v", err)
	}

	parentCert := tpl
	parentKey := privateKey
	if caKeyPair != nil {
		caCerts, err := caKeyPair.Certificates()
		if err != nil {
			return nil, fmt.Errorf("error getting CA certificate: %v", err)
		}
		caPrivateKey, err := caKeyPair.PrivateKey()
		if err != nil {
			return nil, fmt.Errorf("error getting CA private key: %v", err)
		}

		parentCert = caCerts[0] // assume first certificate in the CA bundle
		parentKey = caPrivateKey
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, tpl, parentCert, privateKey.Public(), parentKey)
	if err != nil {
		return nil, fmt.Errorf("error creating certificate: %v", err)
	}
	privateKeyBytes, err := MarshalPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("error creating private key: %v", err)
	}

	certPEMBytes := pemEncodeCertificate(certBytes)
	privateKeyPEMBytes, err := pemEncodePrivateKey(privateKeyBytes, parentKey)
	if err != nil {
		return nil, fmt.Errorf("error encoding private key PEM: %v", err)
	}

	return NewKeyPair(certPEMBytes, privateKeyPEMBytes, opts...)
}
