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

var (
	defaultCACommonName         = "mariadb-operator"
	defaultCALifetimeDuration   = 3 * 365 * 24 * time.Hour // 3 years
	defaultCertLifetimeDuration = 3 * 30 * 24 * time.Hour  // 3 months
)

type X509Opts struct {
	CommonName  string
	DNSNames    []string
	NotBefore   time.Time
	NotAfter    time.Time
	KeyUsage    x509.KeyUsage
	ExtKeyUsage []x509.ExtKeyUsage
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

func WithKeyUsage(keyUsage x509.KeyUsage) X509Opt {
	return func(x *X509Opts) {
		x.KeyUsage |= keyUsage
	}
}

func WithExtKeyUsage(extKeyUsage ...x509.ExtKeyUsage) X509Opt {
	return func(x *X509Opts) {
		x.ExtKeyUsage = append(x.ExtKeyUsage, extKeyUsage...)
	}
}

func CreateCA(x509Opts ...X509Opt) (*KeyPair, error) {
	opts := X509Opts{
		CommonName: defaultCACommonName,
		NotBefore:  time.Now().Add(-1 * time.Hour),
		NotAfter:   time.Now().Add(defaultCALifetimeDuration),
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
			CommonName: opts.CommonName,
		},
		DNSNames: []string{
			opts.CommonName,
		},
		NotBefore:             opts.NotBefore,
		NotAfter:              opts.NotAfter,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	return NewKeyPairFromTemplate(tpl, nil)
}

func CreateCert(caKeyPair *KeyPair, x509Opts ...X509Opt) (*KeyPair, error) {
	opts := X509Opts{
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(defaultCertLifetimeDuration),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
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
		KeyUsage:              opts.KeyUsage,
		ExtKeyUsage:           opts.ExtKeyUsage,
		BasicConstraintsValid: true,
		IsCA:                  false,
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

func ValidateCert(caCerts []*x509.Certificate, certKeyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
	if len(caCerts) == 0 {
		return false, errors.New("CA certicates should be provided to establish trust")
	}
	if err := certKeyPair.Validate(); err != nil {
		return false, fmt.Errorf("invalid keypair: %v", err)
	}
	certs, err := certKeyPair.Certificates()
	if err != nil {
		return false, fmt.Errorf("error getting certificate: %v", err)
	}

	leafCert := certs[0] // leaf certificate should be the first in the chain to establish trust
	var intermediateCAs []*x509.Certificate
	if len(certs) > 1 {
		intermediateCAs = certs[1:] // intermediate certificates, if present, form the chain leading to a trusted root CA
	}

	rootCAsPool := x509.NewCertPool()
	for _, cert := range caCerts {
		rootCAsPool.AddCert(cert)
	}
	intermediateCAsPool := x509.NewCertPool()
	for _, cert := range intermediateCAs {
		intermediateCAsPool.AddCert(cert)
	}

	_, err = leafCert.Verify(x509.VerifyOptions{
		Roots:         rootCAsPool,
		Intermediates: intermediateCAsPool,
		DNSName:       dnsName,
		CurrentTime:   at,
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func ValidateCACert(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
	certs, err := keyPair.Certificates()
	if err != nil {
		return false, fmt.Errorf("error getting certificates: %v", err)
	}
	return ValidateCert(certs, keyPair, dnsName, at)
}

var serialNumberLimit = new(big.Int).Lsh(big.NewInt(1), 128)

func getSerialNumber() (*big.Int, error) {
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}
	return serialNumber, nil
}
