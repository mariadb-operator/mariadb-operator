package certificate

import (
	"context"
	"fmt"
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	defaultCAValidityDuration   = 4 * 365 * 24 * time.Hour
	defaultCertValidityDuration = 365 * 24 * time.Hour
	defaultLookaheadValidity    = 90 * 24 * time.Hour
)

type CertReconcilerOpts struct {
	caSecretKey  types.NamespacedName
	caCommonName string
	caValidity   time.Duration

	certSecretKey  types.NamespacedName
	certCommonName string
	certDNSNames   []string
	certValidity   time.Duration

	lookaheadValidity time.Duration
}

type CertReconcilerOpt func(opts *CertReconcilerOpts)

func WithCAValidity(validity time.Duration) CertReconcilerOpt {
	return func(opts *CertReconcilerOpts) {
		opts.caValidity = validity
	}
}

func WithCertValidity(validity time.Duration) CertReconcilerOpt {
	return func(opts *CertReconcilerOpts) {
		opts.certValidity = validity
	}
}

func WithLookaheadValidity(validity time.Duration) CertReconcilerOpt {
	return func(opts *CertReconcilerOpts) {
		opts.lookaheadValidity = validity
	}
}

type CertReconciler struct {
	client.Client
	CertReconcilerOpts
}

func NewCertReconciler(client client.Client, caSecretKey types.NamespacedName, caCommonName string,
	certSecretKey types.NamespacedName, certCommonName string, certDNSNames []string,
	reconcilerOpts ...CertReconcilerOpt) *CertReconciler {
	opts := CertReconcilerOpts{
		caSecretKey:  caSecretKey,
		caCommonName: caCommonName,
		caValidity:   defaultCAValidityDuration,

		certSecretKey:  certSecretKey,
		certCommonName: certCommonName,
		certValidity:   defaultCertValidityDuration,
		certDNSNames:   certDNSNames,

		lookaheadValidity: defaultLookaheadValidity,
	}
	for _, setOpt := range reconcilerOpts {
		setOpt(&opts)
	}

	return &CertReconciler{
		Client:             client,
		CertReconcilerOpts: opts,
	}
}

type ReconcileResult struct {
	CAKeyPair     *pki.KeyPair
	CertKeyPair   *pki.KeyPair
	RefreshedCA   bool
	RefreshedCert bool
}

func (r *CertReconciler) Reconcile(ctx context.Context) (*ReconcileResult, error) {
	result := &ReconcileResult{}
	var err error
	result.CAKeyPair, result.RefreshedCA, err = r.reconcileKeyPair(ctx, r.caSecretKey, false, r.createCA)
	if err != nil {
		return nil, fmt.Errorf("Error reconciling CA KeyPair: %v", err)
	}

	valid, err := pki.ValidCACert(result.CAKeyPair, r.caCommonName, r.lookaheadTime())
	if !valid || err != nil {
		result.CAKeyPair, result.RefreshedCA, err = r.reconcileKeyPair(ctx, r.caSecretKey, true, r.createCA)
		if err != nil {
			return nil, fmt.Errorf("Error reconciling CA KeyPair: %v", err)
		}
	}

	createCert := r.createCertFn(result.CAKeyPair)
	result.CertKeyPair, result.RefreshedCert, err = r.reconcileKeyPair(ctx, r.certSecretKey, false, createCert)
	if err != nil {
		return nil, fmt.Errorf("Error reconciling certificate KeyPair: %v", err)
	}

	valid, err = pki.ValidCert(result.CAKeyPair.Cert, result.CertKeyPair, r.certCommonName, r.lookaheadTime())
	if result.RefreshedCA || !valid || err != nil {
		result.CertKeyPair, result.RefreshedCert, err = r.reconcileKeyPair(ctx, r.certSecretKey, true, createCert)
		if err != nil {
			return nil, fmt.Errorf("Error reconciling certificate KeyPair: %v", err)
		}
	}
	return result, nil
}

func (r *CertReconciler) reconcileKeyPair(ctx context.Context, key types.NamespacedName, refresh bool,
	createKeyPairFn func() (*pki.KeyPair, error)) (keyPair *pki.KeyPair, refreshed bool, err error) {
	secret := corev1.Secret{}
	if err := r.Get(ctx, key, &secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, err
		}
		keyPair, err := createKeyPairFn()
		if err != nil {
			return nil, false, err
		}
		if err := r.createSecret(ctx, key, &secret, keyPair); err != nil {
			return nil, false, err
		}
		return keyPair, true, nil
	}

	if secret.Data == nil || refresh {
		keyPair, err := createKeyPairFn()
		if err != nil {
			return nil, false, err
		}
		if err := r.patchSecret(ctx, &secret, keyPair); err != nil {
			return nil, false, err
		}
		return keyPair, true, nil
	}

	keyPair, err = pki.KeyPairFromTLSSecret(&secret)
	if err != nil {
		return nil, false, err
	}
	return keyPair, false, nil
}

func (r *CertReconciler) createCA() (*pki.KeyPair, error) {
	return pki.CreateCA(
		pki.WithCommonName(r.caCommonName),
		pki.WithNotBefore(time.Now().Add(-1*time.Hour)),
		pki.WithNotAfter(time.Now().Add(r.caValidity)),
	)
}

func (r *CertReconciler) createCertFn(caKeyPair *pki.KeyPair) func() (*pki.KeyPair, error) {
	return func() (*pki.KeyPair, error) {
		return pki.CreateCert(
			caKeyPair,
			pki.WithCommonName(r.certCommonName),
			pki.WithDNSNames(r.certDNSNames),
			pki.WithNotBefore(time.Now().Add(-1*time.Hour)),
			pki.WithNotAfter(time.Now().Add(r.certValidity)),
		)
	}
}

func (r *CertReconciler) createSecret(ctx context.Context, key types.NamespacedName, secret *corev1.Secret, keyPair *pki.KeyPair) error {
	secret.ObjectMeta = metav1.ObjectMeta{
		Name:      key.Name,
		Namespace: key.Namespace,
	}
	secret.Type = corev1.SecretTypeTLS
	keyPair.FillTLSSecret(secret)
	if err := r.Create(ctx, secret); err != nil {
		return fmt.Errorf("Error creating TLS Secret: %v", err)
	}
	return nil
}

func (r *CertReconciler) patchSecret(ctx context.Context, secret *corev1.Secret, keyPair *pki.KeyPair) error {
	patch := client.MergeFrom(secret.DeepCopy())
	keyPair.FillTLSSecret(secret)
	if err := r.Patch(ctx, secret, patch); err != nil {
		return fmt.Errorf("Error patching TLS Secret: %v", err)
	}
	return nil
}

func (r *CertReconciler) lookaheadTime() time.Time {
	return time.Now().Add(r.lookaheadValidity)
}
