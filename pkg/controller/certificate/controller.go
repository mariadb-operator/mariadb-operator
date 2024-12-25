package certificate

import (
	"context"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	CAKeyPair   *pki.KeyPair
	CertKeyPair *pki.KeyPair
}

func (r *CertReconciler) Reconcile(ctx context.Context, certOpts ...CertReconcilerOpt) (*ReconcileResult, error) {
	opts := NewDefaultCertificateOpts()
	for _, setOpt := range certOpts {
		setOpt(opts)
	}
	result := &ReconcileResult{}
	logger := log.FromContext(ctx).WithName("cert")
	createCA := r.createCAFn(opts)

	var err error
	result.CAKeyPair, err = r.reconcileKeyPair(ctx, opts.caSecretKey, opts.caSecretType, false, opts, createCA)
	if err != nil {
		return nil, fmt.Errorf("Error reconciling CA KeyPair: %v", err)
	}

	caLeafCert, err := getLeafCert(result.CAKeyPair)
	if err != nil {
		return nil, fmt.Errorf("error getting CA leaf certificate: %v", err)
	}
	renewalTime, err := getRenewalTime(caLeafCert.NotBefore, caLeafCert.NotAfter, opts.renewBeforePercentage)
	if err != nil {
		return nil, fmt.Errorf("error getting CA renewal time: %v", err)
	}

	valid, err := pki.ValidateCA(result.CAKeyPair, opts.caCommonName, time.Now())
	afterRenewal := time.Now().After(*renewalTime)
	caLogger := logger.WithValues(
		"common-name", caLeafCert.Subject.CommonName,
		"issuer", caLeafCert.Issuer.CommonName,
		"valid", valid,
		"err", err,
		"renewal-time", renewalTime,
		"after-renewal", afterRenewal,
	)
	caLogger.V(1).Info("CA cert status")

	if !valid || err != nil || afterRenewal {
		caLogger.Info("Starting CA cert renewal")

		result.CAKeyPair, err = r.reconcileKeyPair(ctx, opts.caSecretKey, opts.caSecretType, true, opts, createCA)
		if err != nil {
			return nil, fmt.Errorf("Error reconciling CA KeyPair: %v", err)
		}
	}

	createCert := r.createCertFn(result.CAKeyPair, opts)
	result.CertKeyPair, err = r.reconcileKeyPair(ctx, opts.certSecretKey, SecretTypeTLS, false, opts, createCert)
	if err != nil {
		return nil, fmt.Errorf("Error reconciling certificate KeyPair: %v", err)
	}

	caCerts, err := result.CAKeyPair.Certificates()
	if err != nil {
		return nil, fmt.Errorf("error getting CA certificates: %v", err)
	}
	leafCert, err := getLeafCert(result.CertKeyPair)
	if err != nil {
		return nil, fmt.Errorf("error getting leaf certificate: %v", err)
	}
	renewalTime, err = getRenewalTime(leafCert.NotBefore, leafCert.NotAfter, opts.renewBeforePercentage)
	if err != nil {
		return nil, fmt.Errorf("error getting cert renewal time: %v", err)
	}

	valid, err = pki.ValidateCert(caCerts, result.CertKeyPair, opts.certCommonName, time.Now())
	afterRenewal = time.Now().After(*renewalTime)
	certLogger := logger.WithValues(
		"common-name", leafCert.Subject.CommonName,
		"issuer", leafCert.Issuer.CommonName,
		"valid", valid,
		"err", err,
		"renewal-time", renewalTime,
		"after-renewal", afterRenewal,
	)
	certLogger.V(1).Info("Cert status")

	if !valid || err != nil || afterRenewal {
		certLogger.Info("Starting cert renewal")

		result.CertKeyPair, err = r.reconcileKeyPair(ctx, opts.certSecretKey, SecretTypeTLS, true, opts, createCert)
		if err != nil {
			return nil, fmt.Errorf("Error reconciling certificate KeyPair: %v", err)
		}
	}
	return result, nil
}

func (r *CertReconciler) reconcileKeyPair(ctx context.Context, key types.NamespacedName, secretType SecretType,
	shouldRenew bool, opts *CertReconcilerOpts, createKeyPairFn func() (*pki.KeyPair, error)) (keyPair *pki.KeyPair, err error) {
	secret := corev1.Secret{}
	if err := r.Get(ctx, key, &secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		keyPair, err := createKeyPairFn()
		if err != nil {
			return nil, err
		}
		if err := r.createSecret(ctx, key, secretType, &secret, keyPair); err != nil {
			return nil, err
		}
		return keyPair, nil
	}

	if secret.Data == nil || shouldRenew {
		keyPair, err := createKeyPairFn()
		if err != nil {
			return nil, err
		}
		if err := r.patchSecret(ctx, secretType, &secret, keyPair); err != nil {
			return nil, err
		}
		return keyPair, nil
	}

	keyPairOpts := opts.KeyPairOpts()

	if secretType == SecretTypeCA {
		keyPair, err = pki.NewKeyPairFromCASecret(&secret, keyPairOpts...)
		if err != nil {
			return nil, err
		}
	} else {
		keyPair, err = pki.NewKeyPairFromTLSSecret(&secret, keyPairOpts...)
		if err != nil {
			return nil, err
		}
	}

	return keyPair, nil
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

func getLeafCert(keyPair *pki.KeyPair) (*x509.Certificate, error) {
	certs, err := keyPair.Certificates()
	if err != nil {
		return nil, fmt.Errorf("error getting keypair certificates: %v", err)
	}
	leafCert := certs[0] // First cert is the leaf certificate, the rest are intermediates to form a parth to root.

	return leafCert, nil
}

// See https://github.com/cert-manager/cert-manager/blob/dd8b7d233110cbd49f2f31eb709f39865f8b0300/pkg/util/pki/renewaltime.go#L35
func getRenewalTime(notBefore, notAfter time.Time, renewBeforePercentage int32) (*time.Time, error) {
	if !(renewBeforePercentage >= 10 && renewBeforePercentage <= 90) {
		return nil, fmt.Errorf("invalid renewBeforePercentage %v, it must be between [10, 90]", renewBeforePercentage)
	}
	duration := notAfter.Sub(notBefore)
	renewalDuration := duration * time.Duration(renewBeforePercentage) / 100

	renewalTime := notAfter.Add(-1 * renewalDuration).Truncate(time.Second)

	return &renewalTime, nil
}
