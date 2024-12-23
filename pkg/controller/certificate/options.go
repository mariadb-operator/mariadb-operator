package certificate

import (
	"crypto/x509"
	"errors"
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	"k8s.io/apimachinery/pkg/types"
)

var (
	defaultLookaheadValidity = 30 * 24 * time.Hour
)

type SecretType int

const (
	SecretTypeCA SecretType = iota
	SecretTypeTLS
)

type CertReconcilerOpts struct {
	caSecretKey  types.NamespacedName
	caSecretType SecretType
	caCommonName string
	caLifetime   time.Duration

	certSecretKey   types.NamespacedName
	certCommonName  string
	certDNSNames    []string
	certLifetime    time.Duration
	certKeyUsage    x509.KeyUsage
	certExtKeyUsage []x509.ExtKeyUsage

	lookaheadValidity time.Duration
}

type CertReconcilerOpt func(*CertReconcilerOpts)

func WithCA(secretKey types.NamespacedName, commonName string, lifetime time.Duration) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.caSecretKey = secretKey
		o.caSecretType = SecretTypeCA
		o.caCommonName = commonName
		o.caLifetime = lifetime
	}
}

func WithCASecretType(secretType SecretType) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.caSecretType = secretType
	}
}

func WithCert(secretKey types.NamespacedName, dnsNames []string, lifetime time.Duration) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.certSecretKey = secretKey
		if len(dnsNames) > 0 {
			o.certCommonName = dnsNames[0]
		}
		o.certDNSNames = dnsNames
		o.certLifetime = lifetime
	}
}

func WithCertKeyUsage(keyUsage x509.KeyUsage) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.certKeyUsage = keyUsage
	}
}

func WithCertExtKeyUsage(extKeyUsage ...x509.ExtKeyUsage) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.certExtKeyUsage = extKeyUsage
	}
}

func WithServerCertKeyUsage() CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.certKeyUsage = x509.KeyUsageKeyEncipherment
		o.certExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	}
}

func WithClientCertKeyUsage() CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.certExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
}

func WithLookaheadValidity(validity time.Duration) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.lookaheadValidity = validity
	}
}

func (o *CertReconcilerOpts) CAx509Opts() ([]pki.X509Opt, error) {
	if o.caCommonName == "" || o.caLifetime == 0 {
		return nil, errors.New("caCommonName and caValidity must be set")
	}

	return []pki.X509Opt{
		pki.WithCommonName(o.caCommonName),
		pki.WithNotBefore(time.Now().Add(-1 * time.Hour)),
		pki.WithNotAfter(time.Now().Add(o.caLifetime)),
	}, nil
}

func (o *CertReconcilerOpts) Certx509Opts() ([]pki.X509Opt, error) {
	if len(o.certDNSNames) == 0 || o.certLifetime == 0 {
		return nil, errors.New("certDNSNames and certLifetime must be set")
	}

	return []pki.X509Opt{
		pki.WithCommonName(o.certCommonName),
		pki.WithDNSNames(o.certDNSNames...),
		pki.WithNotBefore(time.Now().Add(-1 * time.Hour)),
		pki.WithNotAfter(time.Now().Add(o.certLifetime)),
		pki.WithKeyUsage(o.certKeyUsage),
		pki.WithExtKeyUsage(o.certExtKeyUsage...),
	}, nil
}

func NewDefaultCertificateOpts() *CertReconcilerOpts {
	opts := &CertReconcilerOpts{
		caLifetime:        pki.DefaultCALifetime,
		certLifetime:      pki.DefaultCertLifetime,
		lookaheadValidity: defaultLookaheadValidity,
	}
	return opts
}
