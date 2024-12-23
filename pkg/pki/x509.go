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
	DefaultCALifetime   = 3 * 365 * 24 * time.Hour // 3 years
	DefaultCertLifetime = 3 * 30 * 24 * time.Hour  // 3 months

	caMinLifetime = 1 * time.Hour
	caMaxLifetime = 10 * 365 * 24 * time.Hour // 10 years

	certMinLifetime = 1 * time.Hour
	certMaxLifetime = 3 * 365 * 24 * time.Hour // 3 years
)

// X509Opts represents options for creating X.509 certificates.
type X509Opts struct {
	// CommonName is the common name for the certificate.
	CommonName string
	// DNSNames is a list of DNS names for the certificate.
	DNSNames []string
	// NotBefore is the start time for the certificate's validity period.
	NotBefore time.Time
	// NotAfter is the end time for the certificate's validity period.
	NotAfter time.Time
	// KeyUsage specifies the allowed uses of the key.
	KeyUsage x509.KeyUsage
	// ExtKeyUsage specifies the extended key usages of the certificate.
	ExtKeyUsage []x509.ExtKeyUsage
	// IsCA indicates whether the certificate is a CA certificate.
	IsCA bool
}

// X509Opt is a function type used to configure X509Opts.
type X509Opt func(*X509Opts)

// WithCommonName sets the common name for the certificate.
func WithCommonName(name string) X509Opt {
	return func(x *X509Opts) {
		x.CommonName = name
	}
}

// WithDNSNames sets the DNS names for the certificate.
func WithDNSNames(dnsNames ...string) X509Opt {
	return func(x *X509Opts) {
		x.DNSNames = dnsNames
	}
}

// WithNotBefore sets the start time for the certificate's validity period.
func WithNotBefore(notBefore time.Time) X509Opt {
	return func(x *X509Opts) {
		x.NotBefore = notBefore
	}
}

// WithNotAfter sets the end time for the certificate's validity period.
func WithNotAfter(notAfter time.Time) X509Opt {
	return func(x *X509Opts) {
		x.NotAfter = notAfter
	}
}

// WithKeyUsage sets the key usage for the certificate.
func WithKeyUsage(keyUsage x509.KeyUsage) X509Opt {
	return func(x *X509Opts) {
		x.KeyUsage |= keyUsage
	}
}

// WithExtKeyUsage sets the extended key usages for the certificate.
func WithExtKeyUsage(extKeyUsage ...x509.ExtKeyUsage) X509Opt {
	return func(x *X509Opts) {
		x.ExtKeyUsage = append(x.ExtKeyUsage, extKeyUsage...)
	}
}

// WithIsCA sets whether the certificate is a CA certificate.
func WithIsCA(isCA bool) X509Opt {
	return func(x *X509Opts) {
		x.IsCA = isCA
	}
}

// CreateCA creates a new CA certificate with the given options.
func CreateCA(x509Opts ...X509Opt) (*KeyPair, error) {
	opts := X509Opts{
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(DefaultCALifetime),
	}
	for _, setOpt := range x509Opts {
		setOpt(&opts)
	}
	if opts.CommonName == "" {
		return nil, errors.New("CommonName is mandatory")
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

// CreateCert creates a new certificate signed by the given CA key pair with the given options.
func CreateCert(caKeyPair *KeyPair, x509Opts ...X509Opt) (*KeyPair, error) {
	opts := X509Opts{
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(DefaultCertLifetime),
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

// ValidateCA validates the given CA key pair at the specified time.
func ValidateCA(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
	certs, err := keyPair.Certificates()
	if err != nil {
		return false, fmt.Errorf("error getting certificates: %v", err)
	}
	return ValidateCert(certs, keyPair, dnsName, at)
}

// ValidateCertOpts represents options for validating certificates.
type ValidateCertOpts struct {
	intermediateCAs []*x509.Certificate
}

// ValidateCertOpt is a function type used to configure ValidateCertOpts.
type ValidateCertOpt func(*ValidateCertOpts)

// WithIntermediateCAs sets the intermediate CAs for certificate validation.
func WithIntermediateCAs(intermediateCAs ...*x509.Certificate) ValidateCertOpt {
	return func(vco *ValidateCertOpts) {
		vco.intermediateCAs = intermediateCAs
	}
}

// ValidateCert validates the given certificate key pair against the provided CA certificates at the specified time.
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

// ParseCertificate parses a single certificate from the given bytes.
func ParseCertificate(bytes []byte) (*x509.Certificate, error) {
	certs, err := ParseCertificates(bytes)
	if err != nil {
		return nil, err
	}
	return certs[0], nil
}

// ParseCertificates parses multiple certificates from the given bytes.
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
