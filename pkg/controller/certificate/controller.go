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

type CertReconciler struct {
	client.Client
}

func NewCertReconciler(client client.Client) *CertReconciler {
	return &CertReconciler{
		Client: client,
	}
}

type ReconcileResult struct {
	CAKeyPair     *pki.KeyPair
	CertKeyPair   *pki.KeyPair
	RefreshedCA   bool
	RefreshedCert bool
}

func (r *CertReconciler) Reconcile(ctx context.Context, certOpts ...CertReconcilerOpt) (*ReconcileResult, error) {
	opts := NewDefaultCertificateOpts()
	for _, setOpt := range certOpts {
		setOpt(opts)
	}

	createCA := r.createCAFn(opts)

	result := &ReconcileResult{}
	var err error
	result.CAKeyPair, result.RefreshedCA, err = r.reconcileKeyPair(ctx, opts.caSecretKey, opts.caSecretType, false, createCA)
	if err != nil {
		return nil, fmt.Errorf("Error reconciling CA KeyPair: %v", err)
	}

	valid, err := pki.ValidateCA(result.CAKeyPair, opts.caCommonName, lookaheadTime(opts))
	if !valid || err != nil {
		result.CAKeyPair, result.RefreshedCA, err = r.reconcileKeyPair(ctx, opts.caSecretKey, opts.caSecretType, true, createCA)
		if err != nil {
			return nil, fmt.Errorf("Error reconciling CA KeyPair: %v", err)
		}
	}

	createCert := r.createCertFn(result.CAKeyPair, opts)
	result.CertKeyPair, result.RefreshedCert, err = r.reconcileKeyPair(ctx, opts.certSecretKey, SecretTypeTLS, false, createCert)
	if err != nil {
		return nil, fmt.Errorf("Error reconciling certificate KeyPair: %v", err)
	}

	caCerts, err := result.CAKeyPair.Certificates()
	if err != nil {
		return nil, fmt.Errorf("error getting CA certificates: %v", err)
	}
	valid, err = pki.ValidateCert(caCerts, result.CertKeyPair, opts.certCommonName, lookaheadTime(opts))
	if result.RefreshedCA || !valid || err != nil {
		result.CertKeyPair, result.RefreshedCert, err = r.reconcileKeyPair(ctx, opts.certSecretKey, SecretTypeTLS, true, createCert)
		if err != nil {
			return nil, fmt.Errorf("Error reconciling certificate KeyPair: %v", err)
		}
	}
	return result, nil
}

func (r *CertReconciler) reconcileKeyPair(ctx context.Context, key types.NamespacedName, secretType SecretType,
	refresh bool, createKeyPairFn func() (*pki.KeyPair, error)) (keyPair *pki.KeyPair, refreshed bool, err error) {
	secret := corev1.Secret{}
	if err := r.Get(ctx, key, &secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, err
		}
		keyPair, err := createKeyPairFn()
		if err != nil {
			return nil, false, err
		}
		if err := r.createSecret(ctx, key, secretType, &secret, keyPair); err != nil {
			return nil, false, err
		}
		return keyPair, true, nil
	}

	if secret.Data == nil || refresh {
		keyPair, err := createKeyPairFn()
		if err != nil {
			return nil, false, err
		}
		if err := r.patchSecret(ctx, secretType, &secret, keyPair); err != nil {
			return nil, false, err
		}
		return keyPair, true, nil
	}

	keyPairOpts := pki.WithSupportedPrivateKeys(
		pki.PrivateKeyTypeECDSA,
		pki.PrivateKeyTypeRSA, // backwards compatibility with webhook certs from previous versions
	)

	if secretType == SecretTypeCA {
		keyPair, err = pki.NewKeyPairFromCASecret(&secret, keyPairOpts)
		if err != nil {
			return nil, false, err
		}
	} else {
		keyPair, err = pki.NewKeyPairFromTLSSecret(&secret, keyPairOpts)
		if err != nil {
			return nil, false, err
		}
	}

	return keyPair, false, nil
}

func (r *CertReconciler) createCAFn(opts *CertReconcilerOpts) func() (*pki.KeyPair, error) {
	return func() (*pki.KeyPair, error) {
		x509Opts, err := opts.CAx509Opts()
		if err != nil {
			return nil, fmt.Errorf("error getting CA x509 opts: %v", err)
		}
		return pki.CreateCA(x509Opts...)
	}
}

func (r *CertReconciler) createCertFn(caKeyPair *pki.KeyPair, opts *CertReconcilerOpts) func() (*pki.KeyPair, error) {
	return func() (*pki.KeyPair, error) {
		x509Opts, err := opts.Certx509Opts()
		if err != nil {
			return nil, fmt.Errorf("errors getting certificate x509 opts: %v", err)
		}
		return pki.CreateCert(caKeyPair, x509Opts...)
	}
}

func (r *CertReconciler) createSecret(ctx context.Context, key types.NamespacedName, secretType SecretType,
	secret *corev1.Secret, keyPair *pki.KeyPair) error {
	secret.ObjectMeta = metav1.ObjectMeta{
		Name:      key.Name,
		Namespace: key.Namespace,
	}

	if secretType == SecretTypeCA {
		keyPair.UpdateCASecret(secret)
	} else {
		secret.Type = corev1.SecretTypeTLS
		keyPair.UpdateTLSSecret(secret)
	}

	if err := r.Create(ctx, secret); err != nil {
		return fmt.Errorf("Error creating TLS Secret: %v", err)
	}
	return nil
}

func (r *CertReconciler) patchSecret(ctx context.Context, secretType SecretType, secret *corev1.Secret, keyPair *pki.KeyPair) error {
	patch := client.MergeFrom(secret.DeepCopy())

	if secretType == SecretTypeCA {
		keyPair.UpdateCASecret(secret)
	} else {
		secret.Type = corev1.SecretTypeTLS
		keyPair.UpdateTLSSecret(secret)
	}

	if err := r.Patch(ctx, secret, patch); err != nil {
		return fmt.Errorf("Error patching TLS Secret: %v", err)
	}
	return nil
}

func lookaheadTime(opts *CertReconcilerOpts) time.Time {
	return time.Now().Add(opts.lookaheadValidity)
}
