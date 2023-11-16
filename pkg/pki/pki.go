package pki

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"

	corev1 "k8s.io/api/core/v1"
)

var (
	tlsCert                     = "tls.crt"
	tlsKey                      = "tls.key"
	defaultCAValidityDuration   = 4 * 365 * 24 * time.Hour
	defaultCertValidityDuration = 365 * 24 * time.Hour
)

type KeyPair struct {
	Cert    *x509.Certificate
	Key     *rsa.PrivateKey
	CertPEM []byte
	KeyPEM  []byte
}

func (k *KeyPair) IsValid() bool {
	return k.Cert != nil && k.Key != nil && len(k.CertPEM) > 0 && len(k.KeyPEM) > 0
}

func (k *KeyPair) FillTLSSecret(secret *corev1.Secret) {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[tlsCert] = k.CertPEM
	secret.Data[tlsKey] = k.KeyPEM
}

func KeyPairFromTLSSecret(secret *corev1.Secret) (*KeyPair, error) {
	if secret.Data == nil {
		return nil, errors.New("TLS Secret is empty")
	}
	certPEM := secret.Data[tlsCert]
	keyPEM := secret.Data[tlsKey]
	return KeyPairFromPEM(certPEM, keyPEM)
}

func KeyPairFromPEM(certPEM, keyPEM []byte) (*KeyPair, error) {
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		return nil, errors.New("TLS Secret is empty")
	}
	pemBlockCert, _ := pem.Decode(certPEM)
	if pemBlockCert == nil {
		return nil, errors.New("Bad certificate")
	}
	cert, err := x509.ParseCertificate(pemBlockCert.Bytes)
	if err != nil {
		return nil, fmt.Errorf("Error parsing x509 certificate: %v", err)
	}

	pemBlockKey, _ := pem.Decode(keyPEM)
	if pemBlockKey == nil {
		return nil, fmt.Errorf("Bad private key")
	}
	key, err := x509.ParsePKCS1PrivateKey(pemBlockKey.Bytes)
	if err != nil {
		return nil, fmt.Errorf("Error parsing PKCS1 private key: %v", err)
	}

	return &KeyPair{
		Cert:    cert,
		Key:     key,
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
	}, nil
}

type X509Opts struct {
	CommonName   string
	DNSNames     []string
	Organization string
	NotBefore    time.Time
	NotAfter     time.Time
}

type X509Opt func(*X509Opts)

func WithCommonName(name string) X509Opt {
	return func(x *X509Opts) {
		x.CommonName = name
	}
}

func WithDNSNames(dnsNames []string) X509Opt {
	return func(x *X509Opts) {
		x.DNSNames = dnsNames
	}
}

func WithOrganization(org string) X509Opt {
	return func(x *X509Opts) {
		x.Organization = org
	}
}

func WithNotBefore(notBefore time.Time) X509Opt {
	return func(x *X509Opts) {
		x.NotBefore = notBefore
	}
}

func WithNotAfter(notAfter time.Time) X509Opt {
	return func(x *X509Opts) {
		x.NotAfter = notAfter
	}
}

func CreateCA(x509Opts ...X509Opt) (*KeyPair, error) {
	opts := X509Opts{
		CommonName:   "mariadb-operator",
		Organization: "mariadb-operator",
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(defaultCAValidityDuration),
	}
	for _, setOpt := range x509Opts {
		setOpt(&opts)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(0),
		Subject: pkix.Name{
			CommonName:   opts.CommonName,
			Organization: []string{opts.Organization},
		},
		DNSNames: []string{
			opts.CommonName,
		},
		NotBefore:             opts.NotBefore,
		NotAfter:              opts.NotAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	return createKeyPair(tpl, nil)
}

func CreateCert(caKeyPair *KeyPair, x509Opts ...X509Opt) (*KeyPair, error) {
	opts := X509Opts{
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(defaultCertValidityDuration),
	}
	for _, setOpt := range x509Opts {
		setOpt(&opts)
	}
	if opts.CommonName == "" || opts.DNSNames == nil {
		return nil, errors.New("CommonName and DNSNames are mandatory")
	}

	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: opts.CommonName,
		},
		DNSNames:              opts.DNSNames,
		NotBefore:             opts.NotBefore,
		NotAfter:              opts.NotAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	return createKeyPair(tpl, caKeyPair)
}

func ParseCert(bytes []byte) (*x509.Certificate, error) {
	pemBlockCert, _ := pem.Decode(bytes)
	if pemBlockCert == nil {
		return nil, errors.New("Error parsing PEM block")
	}
	parsedCert, err := x509.ParseCertificate(pemBlockCert.Bytes)
	if err != nil {
		return nil, err
	}
	return parsedCert, nil
}

func ValidCert(caCert *x509.Certificate, certKeyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
	if !certKeyPair.IsValid() {
		return false, errors.New("Invalid certificate KeyPair")
	}
	_, err := tls.X509KeyPair(certKeyPair.CertPEM, certKeyPair.KeyPEM)
	if err != nil {
		return false, err
	}
	parsedCert, err := ParseCert(certKeyPair.CertPEM)
	if err != nil {
		return false, err
	}

	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	_, err = parsedCert.Verify(x509.VerifyOptions{
		DNSName:     dnsName,
		Roots:       pool,
		CurrentTime: at,
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func ValidCACert(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
	return ValidCert(keyPair.Cert, keyPair, dnsName, at)
}

func createKeyPair(tpl *x509.Certificate, caKeyPair *KeyPair) (*KeyPair, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	parent := tpl
	privateKey := key
	if caKeyPair != nil {
		parent = caKeyPair.Cert
		privateKey = caKeyPair.Key
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, tpl, parent, key.Public(), privateKey)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, err
	}
	certPEM, keyPEM, err := pemEncodeKeyPair(certBytes, key)
	if err != nil {
		return nil, err
	}

	return &KeyPair{
		Cert:    cert,
		Key:     key,
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
	}, nil
}

func pemEncodeKeyPair(certificateDER []byte, key *rsa.PrivateKey) (certPEM []byte, keyPEM []byte, err error) {
	certBuf := &bytes.Buffer{}
	if err := pem.Encode(certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}); err != nil {
		return nil, nil, err
	}
	keyBuf := &bytes.Buffer{}
	if err := pem.Encode(keyBuf, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		return nil, nil, err
	}
	return certBuf.Bytes(), keyBuf.Bytes(), nil
}
