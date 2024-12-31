package certificate

import (
	"context"
	"crypto/x509"
	"errors"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type SecretType int

const (
	SecretTypeCA SecretType = iota
	SecretTypeTLS
)

type ShouldRenewCertFn func(ctx context.Context, caKeyPair *pki.KeyPair) (shouldRenew bool, reason string, err error)

type RelatedObject interface {
	runtime.Object
	metav1.Object
}

const DefaultRenewBeforePercentage = 33

var (
	DefaultRenewCertReason   = "Certificate lifetime within renewal window"
	DefaultShouldRenewCertFn = func(context.Context, *pki.KeyPair) (bool, string, error) {
		return true, DefaultRenewCertReason, nil
	}
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
	shouldRenewCert ShouldRenewCertFn
	certSecretKey   types.NamespacedName
	certCommonName  string
	certDNSNames    []string
	certLifetime    time.Duration
	certKeyUsage    x509.KeyUsage
	certExtKeyUsage []x509.ExtKeyUsage

	supportedPrivateKeys []pki.PrivateKey

	renewBeforePercentage int32

	relatedObject RelatedObject
}

type CertReconcilerOpt func(*CertReconcilerOpts)

func WithCABundle(secretKey mariadbv1alpha1.SecretKeySelector, namespace string) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.caBundleSecretKey = &secretKey
		o.caBundleNamespace = &namespace
	}
}

func WithCA(shouldIssue bool, secretKey types.NamespacedName) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.shouldIssueCA = shouldIssue
		o.caSecretKey = secretKey
		o.caCommonName = secretKey.Name
	}
}

func WithCACommonName(commonName string) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.caCommonName = commonName
	}
}

func WithCALifetime(lifetime time.Duration) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.caLifetime = lifetime
	}
}

func WithCASecretType(secretType SecretType) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.caSecretType = secretType
	}
}

func WithCert(shouldIssue bool, secretKey types.NamespacedName, dnsNames []string) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.shouldIssueCert = shouldIssue
		o.certSecretKey = secretKey
		if len(dnsNames) > 0 {
			o.certCommonName = dnsNames[0]
		}
		o.certDNSNames = dnsNames
	}
}

func WithShouldRenewCertFn(shouldRenewFn ShouldRenewCertFn) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.shouldRenewCert = shouldRenewFn
	}
}

func WithCertLifetime(lifetime time.Duration) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
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

func WithRelatedObject(obj RelatedObject) CertReconcilerOpt {
	return func(o *CertReconcilerOpts) {
		o.relatedObject = obj
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
		caSecretType:    SecretTypeCA,
		caLifetime:      pki.DefaultCALifetime,
		shouldIssueCert: true,
		shouldRenewCert: DefaultShouldRenewCertFn,
		certLifetime:    pki.DefaultCertLifetime,
		supportedPrivateKeys: []pki.PrivateKey{
			pki.PrivateKeyTypeECDSA,
		},
		renewBeforePercentage: DefaultRenewBeforePercentage,
	}
	return opts
}
