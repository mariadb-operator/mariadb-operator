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
	defaultCACommonName = "mariadb-operator"
	defaultCALifetime   = 3 * 365 * 24 * time.Hour // 3 years
	defaultCertLifetime = 3 * 30 * 24 * time.Hour  // 3 months

	caMinLifetime = 1 * time.Hour
	caMaxLifetime = 10 * 365 * 24 * time.Hour // 10 years

	certMinLifetime = 1 * time.Hour
	certMaxLifetime = 3 * 365 * 24 * time.Hour // 3 years
)

type X509Opts struct {
	CommonName  string
	DNSNames    []string
	NotBefore   time.Time
	NotAfter    time.Time
	KeyUsage    x509.KeyUsage
	ExtKeyUsage []x509.ExtKeyUsage
	IsCA        bool
}

type X509Opt func(*X509Opts)

func WithCommonName(name string) X509Opt {
	return func(x *X509Opts) {
		x.CommonName = name
	}
}

func WithDNSNames(dnsNames ...string) X509Opt {
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

func WithIsCA(isCA bool) X509Opt {
	return func(x *X509Opts) {
		x.IsCA = isCA
	}
}

func CreateCA(x509Opts ...X509Opt) (*KeyPair, error) {
	opts := X509Opts{
		CommonName: defaultCACommonName,
		NotBefore:  time.Now().Add(-1 * time.Hour),
		NotAfter:   time.Now().Add(defaultCALifetime),
	}
	for _, setOpt := range x509Opts {
		setOpt(&opts)
	}
	if err := validateLifetime(opts.NotBefore, opts.NotAfter, caMinLifetime, caMaxLifetime); err != nil {
		return nil, fmt.Errorf("invalid CA lifetime: %v", err)
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
		NotAfter:  time.Now().Add(defaultCertLifetime),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
		IsCA:      false,
	}
	for _, setOpt := range x509Opts {
		setOpt(&opts)
	}
	if opts.CommonName == "" || opts.DNSNames == nil {
		return nil, errors.New("CommonName and DNSNames are mandatory")
	}
	if err := validateLifetime(opts.NotBefore, opts.NotAfter, certMinLifetime, certMaxLifetime); err != nil {
		return nil, fmt.Errorf("invalid certificate lifetime: %v", err)
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
		IsCA:                  opts.IsCA,
	}
	return NewKeyPairFromTemplate(tpl, caKeyPair)
}

func ValidateCA(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
	certs, err := keyPair.Certificates()
	if err != nil {
		return false, fmt.Errorf("error getting certificates: %v", err)
	}
	return ValidateCert(certs, keyPair, dnsName, at)
}

type ValidateCertOpts struct {
	intermediateCAs []*x509.Certificate
}

type ValidateCertOpt func(*ValidateCertOpts)

func WithIntermediateCAs(intermediateCAs ...*x509.Certificate) ValidateCertOpt {
	return func(vco *ValidateCertOpts) {
		vco.intermediateCAs = intermediateCAs
	}
}

func ValidateCert(
	caCerts []*x509.Certificate,
	certKeyPair *KeyPair,
	dnsName string,
	at time.Time,
	validateCertOpts ...ValidateCertOpt,
) (bool, error) {
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

	opts := ValidateCertOpts{}
	for _, setOpt := range validateCertOpts {
		setOpt(&opts)
	}

	rootCAsPool := x509.NewCertPool()
	for _, cert := range caCerts {
		rootCAsPool.AddCert(cert)
	}
	intermediateCAsPool := x509.NewCertPool()
	for _, cert := range intermediateCAs {
		intermediateCAsPool.AddCert(cert)
	}
	// explicitly provided intermediate CAs to form the chain leading to a trusted root CA
	for _, cert := range opts.intermediateCAs {
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

func validateLifetime(notBefore, notAfter time.Time, minDuration, maxDuration time.Duration) error {
	if notBefore.After(notAfter) {
		return fmt.Errorf("NotBefore (%v) cannot be after NotAfter (%v)", notBefore, notAfter)
	}

	duration := notAfter.Sub(notBefore)
	if duration < minDuration {
		return fmt.Errorf("lifetime duration (%v) is less than the minimum allowed duration (%v)", duration, minDuration)
	}

	if duration > maxDuration {
		return fmt.Errorf("lifetime duration (%v) exceeds the maximum allowed duration (%v)", duration, maxDuration)
	}

	return nil
}

var serialNumberLimit = new(big.Int).Lsh(big.NewInt(1), 128)

func getSerialNumber() (*big.Int, error) {
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}
	return serialNumber, nil
}
