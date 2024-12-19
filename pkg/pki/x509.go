package pki

import (
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"
)

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

	serialNumber, err := getSerialNumber()
	if err != nil {
		return nil, fmt.Errorf("error getting serial number: %v", err)
	}

	tpl := &x509.Certificate{
		SerialNumber: serialNumber,
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
	return NewKeyPairFromTemplate(tpl, nil)
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

	serialNumber, err := getSerialNumber()
	if err != nil {
		return nil, fmt.Errorf("error getting serial number: %v", err)
	}

	tpl := &x509.Certificate{
		SerialNumber: serialNumber,
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
	return NewKeyPairFromTemplate(tpl, caKeyPair)
}

func ParseCertificate(bytes []byte) (*x509.Certificate, error) {
	certs, err := ParseCertificates(bytes)
	if err != nil {
		return nil, err
	}
	return certs[0], nil
}

func ParseCertificates(bytes []byte) ([]*x509.Certificate, error) {
	var (
		certs []*x509.Certificate
		block *pem.Block
	)
	pemBytes := bytes

	for len(pemBytes) > 0 {
		block, pemBytes = pem.Decode(pemBytes)
		if block == nil {
			return nil, errors.New("invalid PEM block")
		}
		if block.Type != pemBlockCertificate {
			return nil, fmt.Errorf("invalid PEM certificate block, got block type: %v", block.Type)
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		return nil, errors.New("no valid certificates found")
	}

	return certs, nil
}

func ValidCert(caCerts []*x509.Certificate, certKeyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
	if err := certKeyPair.Validate(); err != nil {
		return false, fmt.Errorf("invalid keypair: %v", err)
	}

	certs, err := certKeyPair.Certificates()
	if err != nil {
		return false, fmt.Errorf("error getting certificate: %v", err)
	}
	cert := certs[0] // leaf certificates should only have a single certificate, not a bundle

	pool := x509.NewCertPool()
	for _, cert := range caCerts {
		pool.AddCert(cert)
	}
	_, err = cert.Verify(x509.VerifyOptions{
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
	certs, err := keyPair.Certificates()
	if err != nil {
		return false, fmt.Errorf("error getting certificates: %v", err)
	}
	return ValidCert(certs, keyPair, dnsName, at)
}

var serialNumberLimit = new(big.Int).Lsh(big.NewInt(1), 128)

func getSerialNumber() (*big.Int, error) {
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}
	return serialNumber, nil
}
