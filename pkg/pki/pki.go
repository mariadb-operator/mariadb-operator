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

type KeyPair struct {
	Cert    *x509.Certificate
	Key     *rsa.PrivateKey
	CertPEM []byte
	KeyPEM  []byte
}

type CAOpts struct {
	Name string
	Org  string
}

type CAOpt func(*CAOpts)

func WithCAName(name string) CAOpt {
	return func(c *CAOpts) {
		c.Name = name
	}
}

func WithCAOrg(org string) CAOpt {
	return func(c *CAOpts) {
		c.Org = org
	}
}

func CreateCACert(begin, end time.Time, opts ...CAOpt) (*KeyPair, error) {
	caOpts := CAOpts{
		Name: "mariadb-operator",
		Org:  "mariadb-operator",
	}
	for _, setOpt := range opts {
		setOpt(&caOpts)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(0),
		Subject: pkix.Name{
			CommonName:   caOpts.Name,
			Organization: []string{caOpts.Org},
		},
		DNSNames: []string{
			caOpts.Name,
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
	certPEM, keyPEM, err := pemEncodeKeyPair(der, key)
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

func ValidCert(caCert, cert, key []byte, dnsName string, at time.Time) (bool, error) {
	if len(caCert) == 0 || len(cert) == 0 || len(key) == 0 {
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

	_, err = tls.X509KeyPair(cert, key)
	if err != nil {
		return false, err
	}

	certBytes, _ := pem.Decode(cert)
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

func pemEncodeKeyPair(certificateDER []byte, key *rsa.PrivateKey) (certBytes []byte, keyBytes []byte, err error) {
	certBuf := &bytes.Buffer{}
	if err := pem.Encode(certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}); err != nil {
		return nil, nil, err
	}
	keyBuf := &bytes.Buffer{}
	if err := pem.Encode(keyBuf, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		return nil, nil, err
	}
	certBytes = certBuf.Bytes()
	keyBytes = keyBuf.Bytes()
	err = nil
	return
}
