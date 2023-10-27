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
	"math/big"
	"time"
)

type KeyPairPEM struct {
	CertPEM []byte
	KeyPEM  []byte
}

type KeyPair struct {
	KeyPairPEM
	Cert *x509.Certificate
	Key  *rsa.PrivateKey
}

type CAOpts struct {
	CommonName   string
	Organization string
}

type CAOpt func(*CAOpts)

func WithCACommonName(name string) CAOpt {
	return func(c *CAOpts) {
		c.CommonName = name
	}
}

func WithCAOrganization(org string) CAOpt {
	return func(c *CAOpts) {
		c.Organization = org
	}
}

func CreateCACert(begin, end time.Time, opts ...CAOpt) (*KeyPair, error) {
	caOpts := CAOpts{
		CommonName:   "mariadb-operator",
		Organization: "mariadb-operator",
	}
	for _, setOpt := range opts {
		setOpt(&caOpts)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(0),
		Subject: pkix.Name{
			CommonName:   caOpts.CommonName,
			Organization: []string{caOpts.Organization},
		},
		DNSNames: []string{
			caOpts.CommonName,
		},
		NotBefore:             begin,
		NotAfter:              end,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, key.Public(), key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	keyPairPEM, err := pemEncodeKeyPair(der, key)
	if err != nil {
		return nil, err
	}

	return &KeyPair{
		Cert:       cert,
		Key:        key,
		KeyPairPEM: *keyPairPEM,
	}, nil
}

func CreateCertPEM(ca *KeyPair, begin, end time.Time, commonName string, dnsNames []string) (*KeyPairPEM, error) {
	templ := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		DNSNames:              dnsNames,
		NotBefore:             begin,
		NotAfter:              end,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	der, err := x509.CreateCertificate(rand.Reader, templ, ca.Cert, key.Public(), ca.Key)
	if err != nil {
		return nil, err
	}
	keyPairPEM, err := pemEncodeKeyPair(der, key)
	if err != nil {
		return nil, err
	}
	return keyPairPEM, nil
}

func ValidCert(caCert []byte, keyPairPEM *KeyPairPEM, dnsName string, at time.Time) (bool, error) {
	if len(caCert) == 0 || len(keyPairPEM.CertPEM) == 0 || len(keyPairPEM.KeyPEM) == 0 {
		return false, errors.New("CA certificate, certificate and private key must be provided")
	}

	pool := x509.NewCertPool()
	caDer, _ := pem.Decode(caCert)
	if caDer == nil {
		return false, errors.New("Invalid CA certificate")
	}
	ca, err := x509.ParseCertificate(caDer.Bytes)
	if err != nil {
		return false, err
	}
	pool.AddCert(ca)

	_, err = tls.X509KeyPair(keyPairPEM.CertPEM, keyPairPEM.KeyPEM)
	if err != nil {
		return false, err
	}

	certBytes, _ := pem.Decode(keyPairPEM.CertPEM)
	if certBytes == nil {
		return false, err
	}

	parsedCert, err := x509.ParseCertificate(certBytes.Bytes)
	if err != nil {
		return false, err
	}
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

func ValidCACert(keyPairPEM *KeyPairPEM, dnsName string, at time.Time) (bool, error) {
	return ValidCert(keyPairPEM.CertPEM, keyPairPEM, dnsName, at)
}

func pemEncodeKeyPair(certificateDER []byte, key *rsa.PrivateKey) (*KeyPairPEM, error) {
	certBuf := &bytes.Buffer{}
	if err := pem.Encode(certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}); err != nil {
		return nil, err
	}
	keyBuf := &bytes.Buffer{}
	if err := pem.Encode(keyBuf, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		return nil, err
	}
	return &KeyPairPEM{
		CertPEM: certBuf.Bytes(),
		KeyPEM:  keyBuf.Bytes(),
	}, nil
}
