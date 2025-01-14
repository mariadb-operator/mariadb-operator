package builder

import (
	"errors"
	"fmt"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type CertOpts struct {
	Key                   *types.NamespacedName
	Owner                 metav1.Object
	DNSNames              []string
	Lifetime              *time.Duration
	RenewBeforePercentage *int32
	Usages                []certmanagerv1.KeyUsage
	IssuerRef             *cmmeta.ObjectReference
}

type CertOpt func(*CertOpts)

func WithKey(key types.NamespacedName) CertOpt {
	return func(o *CertOpts) {
		o.Key = ptr.To(key)
	}
}

func WithOwner(owner metav1.Object) CertOpt {
	return func(o *CertOpts) {
		o.Owner = owner
	}
}

func WithDNSnames(names []string) CertOpt {
	return func(o *CertOpts) {
		o.DNSNames = names
	}
}

func WithLifetime(lifetime time.Duration) CertOpt {
	return func(o *CertOpts) {
		o.Lifetime = ptr.To(lifetime)
	}
}

func WithRenewBeforePercentage(percentage int32) CertOpt {
	return func(o *CertOpts) {
		o.RenewBeforePercentage = ptr.To(percentage)
	}
}

func WithUsages(usages ...certmanagerv1.KeyUsage) CertOpt {
	return func(o *CertOpts) {
		o.Usages = append(o.Usages, usages...)
	}
}

func WithIssuerRef(issuerRef cmmeta.ObjectReference) CertOpt {
	return func(o *CertOpts) {
		o.IssuerRef = ptr.To(issuerRef)
	}
}

func (b *Builder) BuildCertificate(certOpts ...CertOpt) (*certmanagerv1.Certificate, error) {
	opts := CertOpts{
		Lifetime:              ptr.To(pki.DefaultCertLifetime),
		RenewBeforePercentage: ptr.To(pki.DefaultRenewBeforePercentage),
		Usages: []certmanagerv1.KeyUsage{
			certmanagerv1.UsageDigitalSignature,
			certmanagerv1.UsageKeyAgreement,
		},
	}
	for _, setOpt := range certOpts {
		setOpt(&opts)
	}
	if opts.Key == nil || opts.Owner == nil || len(opts.DNSNames) == 0 ||
		opts.Lifetime == nil || opts.RenewBeforePercentage == nil || opts.IssuerRef == nil {
		return nil, errors.New("Key, Owner, DNSNames, Lifetime, RenewBeforePercentage and IssuerRef must be set")
	}

	renewBefore, err := pki.RenewalDuration(*opts.Lifetime, *opts.RenewBeforePercentage)
	if err != nil {
		return nil, fmt.Errorf("error getting renewal duration: %v", err)
	}

	cert := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Key.Name,
			Namespace: opts.Key.Namespace,
		},
		Spec: certmanagerv1.CertificateSpec{
			Duration:    &metav1.Duration{Duration: *opts.Lifetime},
			RenewBefore: &metav1.Duration{Duration: *renewBefore},
			DNSNames:    opts.DNSNames,
			CommonName:  opts.DNSNames[0],
			Usages:      opts.Usages,
			IssuerRef:   *opts.IssuerRef,
			IsCA:        false,
			PrivateKey: &certmanagerv1.CertificatePrivateKey{
				Encoding:  certmanagerv1.PKCS1,
				Algorithm: certmanagerv1.ECDSAKeyAlgorithm,
				Size:      256,
			},
			SecretTemplate: &certmanagerv1.CertificateSecretTemplate{
				Labels: map[string]string{
					metadata.WatchLabel: "",
				},
			},
			SecretName:           opts.Key.Name,
			RevisionHistoryLimit: ptr.To(int32(10)),
		},
	}
	if err := controllerutil.SetControllerReference(opts.Owner, cert, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Certificate: %v", err)
	}
	return cert, nil
}
