package certificate

import (
	"crypto/x509"
	"errors"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	"k8s.io/apimachinery/pkg/types"
)

const DefaultRenewBeforePercentage = 33

type SecretType int

const (
	SecretTypeCA SecretType = iota
	SecretTypeTLS
)

type CertReconcilerOpts struct {
	caBundleSecretKey *mariadbv1alpha1.SecretKeySelector
	caBundleNamespace *string

	shouldIssueCA bool
	caSecretKey   types.NamespacedName
	caSecretType  SecretType
	caCommonName  string
	caLifetime    time.Duration

	shouldIssueCert bool
	certSecretKey   types.NamespacedName
	certCommonName  string
	certDNSNames    []string
	certLifetime    time.Duration
	certKeyUsage    x509.KeyUsage
	certExtKeyUsage []x509.ExtKeyUsage

	supportedPrivateKeys []pki.PrivateKey

	renewBeforePercentage int32
}

type CertReconcilerOpt func(*CertReconcilerOpts)

func WithCABundle(secretKey mariadbv1alpha1.SecretKeySelector, namespace string) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.caBundleSecretKey = &secretKey
		o.caBundleNamespace = &namespace
	}
}

func WithCA(shouldIssue bool, secretKey types.NamespacedName, commonName string, lifetime time.Duration) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.shouldIssueCA = shouldIssue
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

func WithCert(shouldIssue bool, secretKey types.NamespacedName, dnsNames []string, lifetime time.Duration) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.shouldIssueCert = shouldIssue
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

func WithSupportedPrivateKeys(privateKeys ...pki.PrivateKey) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.supportedPrivateKeys = privateKeys
	}
}

func WithRenewBeforePercentage(percentage int32) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.renewBeforePercentage = percentage
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
		pki.WithKeyPairOpts(o.KeyPairOpts()...),
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
		pki.WithKeyPairOpts(o.KeyPairOpts()...),
	}, nil
}

func (o *CertReconcilerOpts) KeyPairOpts() []pki.KeyPairOpt {
	return []pki.KeyPairOpt{
		pki.WithSupportedPrivateKeys(o.supportedPrivateKeys...),
	}
}

func NewDefaultCertificateOpts() *CertReconcilerOpts {
	opts := &CertReconcilerOpts{
		shouldIssueCA:   true,
		caLifetime:      pki.DefaultCALifetime,
		shouldIssueCert: true,
		certLifetime:    pki.DefaultCertLifetime,
		supportedPrivateKeys: []pki.PrivateKey{
			pki.PrivateKeyTypeECDSA,
		},
		renewBeforePercentage: DefaultRenewBeforePercentage,
	}
	return opts
}
