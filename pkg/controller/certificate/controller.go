package certificate

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	ErrSkipCertRenewal = errors.New("skipping certificate renewal")
)

type CertReconciler struct {
	client.Client
	scheme      *runtime.Scheme
	recorder    record.EventRecorder
	refResolver *refresolver.RefResolver
}

func NewCertReconciler(client client.Client, scheme *runtime.Scheme, recorder record.EventRecorder) *CertReconciler {
	return &CertReconciler{
		Client:      client,
		scheme:      scheme,
		recorder:    recorder,
		refResolver: refresolver.New(client),
	}
}

type ReconcileResult struct {
	ctrl.Result
	CAKeyPair   *pki.KeyPair
	CertKeyPair *pki.KeyPair
}

func (r *ReconcileResult) IsZero() bool {
	if r == nil {
		return true
	}
	return r.Result.IsZero()
}

func (r *CertReconciler) Reconcile(ctx context.Context, certOpts ...CertReconcilerOpt) (*ReconcileResult, error) {
	opts := NewDefaultCertificateOpts()
	for _, setOpt := range certOpts {
		setOpt(opts)
	}
	logger := log.FromContext(ctx).WithName("cert")
	result := &ReconcileResult{}
	var err error

	result.CAKeyPair, err = r.reconcileCA(ctx, opts, logger)
	if err != nil {
		return nil, fmt.Errorf("error reconciling CA: %v", err)
	}
	result.Result, result.CertKeyPair, err = r.reconcileCert(ctx, result.CAKeyPair, opts, logger)
	if err != nil {
		return nil, fmt.Errorf("error reconciling certificate: %v", err)
	}
	return result, nil
}

func (r *CertReconciler) reconcileCA(ctx context.Context, opts *CertReconcilerOpts, logger logr.Logger) (*pki.KeyPair, error) {
	if !opts.shouldIssueCA && !opts.shouldIssueCert {
		return nil, nil
	}
	if !opts.shouldIssueCA && opts.shouldIssueCert {
		caKeyPair, err := r.getCAKeyPair(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("error getting CA keypair: %v", err)
		}
		return caKeyPair, nil
	}

	createCA := r.createCAFn(opts)
	caKeyPair, err := r.reconcileKeyPair(ctx, opts.caSecretKey, opts.caSecretType, false, opts, createCA)
	if err != nil {
		return nil, fmt.Errorf("Error reconciling CA keypair: %v", err)
	}

	caLeafCert, err := caKeyPair.LeafCertificate()
	if err != nil {
		return nil, fmt.Errorf("error getting CA leaf certificate: %v", err)
	}
	renewalTime, err := getRenewalTime(caLeafCert.NotBefore, caLeafCert.NotAfter, opts.renewBeforePercentage)
	if err != nil {
		return nil, fmt.Errorf("error getting CA renewal time: %v", err)
	}

	valid, err := pki.ValidateCA(caKeyPair, opts.caCommonName, time.Now())
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
		caLogger.Info("starting CA cert renewal")

		caKeyPair, err = r.reconcileKeyPair(ctx, opts.caSecretKey, opts.caSecretType, true, opts, createCA)
		if err != nil {
			return nil, fmt.Errorf("Error reconciling CA keypair: %v", err)
		}
	}
	return caKeyPair, nil
}

func (r *CertReconciler) reconcileCert(ctx context.Context, caKeyPair *pki.KeyPair, opts *CertReconcilerOpts,
	logger logr.Logger) (ctrl.Result, *pki.KeyPair, error) {
	if !opts.shouldIssueCert {
		return ctrl.Result{}, nil, nil
	}
	if caKeyPair == nil {
		return ctrl.Result{}, nil, errors.New("unable to issue cert: CA keypair is nil")
	}

	createCert := r.createCertFn(caKeyPair, opts)
	certKeyPair, err := r.reconcileKeyPair(ctx, opts.certSecretKey, SecretTypeTLS, false, opts, createCert)
	if err != nil {
		return ctrl.Result{}, nil, fmt.Errorf("Error reconciling certificate keypair: %v", err)
	}

	caCerts, err := r.getCABundle(ctx, caKeyPair, opts, logger)
	if err != nil {
		return ctrl.Result{}, nil, fmt.Errorf("Error getting CA bundle: %v", err)
	}
	leafCert, err := certKeyPair.LeafCertificate()
	if err != nil {
		return ctrl.Result{}, nil, fmt.Errorf("error getting leaf certificate: %v", err)
	}
	renewalTime, err := getRenewalTime(leafCert.NotBefore, leafCert.NotAfter, opts.renewBeforePercentage)
	if err != nil {
		return ctrl.Result{}, nil, fmt.Errorf("error getting cert renewal time: %v", err)
	}

	valid, err := pki.ValidateCert(caCerts, certKeyPair, opts.certCommonName, time.Now())
	afterRenewal := time.Now().After(*renewalTime)
	certLogger := logger.WithValues(
		"common-name", leafCert.Subject.CommonName,
		"issuer", leafCert.Issuer.CommonName,
		"valid", valid,
		"err", err,
		"renewal-time", renewalTime,
		"after-renewal", afterRenewal,
	)
	certLogger.V(1).Info("cert status")

	if !valid || err != nil {
		certLogger.Info("starting cert renewal", "reason", "Invalid cert")

		certKeyPair, err = r.reconcileKeyPair(ctx, opts.certSecretKey, SecretTypeTLS, true, opts, createCert)
		if err != nil {
			return ctrl.Result{}, nil, fmt.Errorf("error reconciling certificate KeyPair: %v", err)
		}
		if err := opts.certHandler.HandleExpiredCert(ctx); err != nil {
			return ctrl.Result{}, certKeyPair, fmt.Errorf("error handling expired certificate: %v", err)
		}
		return ctrl.Result{}, certKeyPair, nil
	}

	if !afterRenewal {
		return ctrl.Result{}, certKeyPair, nil
	}
	shouldRenew, reason, err := opts.certHandler.ShouldRenewCert(ctx, caKeyPair)
	if err != nil {
		if errors.Is(err, ErrSkipCertRenewal) {
			certLogger.V(1).Info("skipping cert renewal", "reason", reason)

			return ctrl.Result{}, certKeyPair, nil
		}
		return ctrl.Result{}, nil, fmt.Errorf("error checking whether certificate should be renewed: %v", err)
	}
	if !shouldRenew {
		certLogger.Info("waiting for cert renewal", "reason", reason)

		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil, nil
	}
	if shouldRenew {
		certLogger.Info("starting cert renewal", "reason", reason)

		certKeyPair, err = r.reconcileKeyPair(ctx, opts.certSecretKey, SecretTypeTLS, true, opts, createCert)
		if err != nil {
			return ctrl.Result{}, nil, fmt.Errorf("error reconciling certificate KeyPair: %v", err)
		}
		return ctrl.Result{}, certKeyPair, nil
	}

	return ctrl.Result{}, certKeyPair, nil
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
		if err := r.createSecret(ctx, key, secretType, &secret, keyPair, opts.relatedObject); err != nil {
			return nil, err
		}
		return keyPair, nil
	}

	if secret.Data == nil || shouldRenew {
		keyPair, err := createKeyPairFn()
		if err != nil {
			return nil, err
		}
		if err := r.patchSecret(ctx, secretType, &secret, keyPair, opts.relatedObject); err != nil {
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

func (r *CertReconciler) getCAKeyPair(ctx context.Context, opts *CertReconcilerOpts) (*pki.KeyPair, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, opts.caSecretKey, &secret); err != nil {
		return nil, fmt.Errorf("error getting CA keypair Secret: %w", err)
	}
	keyPairOpts := opts.KeyPairOpts()

	if opts.caSecretType == SecretTypeCA {
		keyPair, err := pki.NewKeyPairFromCASecret(&secret, keyPairOpts...)
		return r.handleCAKeyPairResult(keyPair, err, opts.caSecretKey.Name, opts)
	}

	keyPair, err := pki.NewKeyPairFromTLSSecret(&secret, keyPairOpts...)
	return r.handleCAKeyPairResult(keyPair, err, opts.caSecretKey.Name, opts)
}

func (r *CertReconciler) handleCAKeyPairResult(keyPair *pki.KeyPair, err error, secretName string,
	opts *CertReconcilerOpts) (*pki.KeyPair, error) {
	if err != nil {
		if errors.Is(err, pki.ErrSecretKeyNotFound) {
			msg := fmt.Sprintf("key not found in CA Secret \"%s\": %v", secretName, err)

			if relatedObj := opts.relatedObject; relatedObj != nil {
				r.recorder.Event(opts.relatedObject, corev1.EventTypeWarning, mariadbv1alpha1.SecretKeyNotFound, msg)
			}
			return nil, errors.New(msg)
		}
		return nil, fmt.Errorf("error getting CA Secret \"%s\": %v", secretName, err)
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

func (r *CertReconciler) createSecret(ctx context.Context, key types.NamespacedName, secretType SecretType, secret *corev1.Secret,
	keyPair *pki.KeyPair, owner metav1.Object) error {
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
	if err := r.updateSecretMetadata(secret, owner); err != nil {
		return fmt.Errorf("error updating Secret metadata: %v", err)
	}

	if err := r.Create(ctx, secret); err != nil {
		return fmt.Errorf("Error creating TLS Secret: %v", err)
	}
	return nil
}

func (r *CertReconciler) patchSecret(ctx context.Context, secretType SecretType, secret *corev1.Secret,
	keyPair *pki.KeyPair, owner metav1.Object) error {
	patch := client.MergeFrom(secret.DeepCopy())

	if secretType == SecretTypeCA {
		keyPair.UpdateCASecret(secret)
	} else {
		secret.Type = corev1.SecretTypeTLS
		keyPair.UpdateTLSSecret(secret)
	}
	if err := r.updateSecretMetadata(secret, owner); err != nil {
		return fmt.Errorf("error updating Secret metadata: %v", err)
	}

	if err := r.Patch(ctx, secret, patch); err != nil {
		return fmt.Errorf("Error patching TLS Secret: %v", err)
	}
	return nil
}

func (r *CertReconciler) updateSecretMetadata(secret *corev1.Secret, owner metav1.Object) error {
	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}
	secret.Labels[metadata.WatchLabel] = ""

	if owner != nil {
		if err := controllerutil.SetControllerReference(owner, secret, r.scheme); err != nil {
			return fmt.Errorf("error setting controller reference to Secret: %v", err)
		}
	}
	return nil
}

func (r *CertReconciler) getCABundle(ctx context.Context, caKeyPair *pki.KeyPair, opts *CertReconcilerOpts,
	logger logr.Logger) ([]*x509.Certificate, error) {
	if opts.caBundleSecretKey != nil && opts.caBundleNamespace != nil {
		bundle, err := r.refResolver.SecretKeyRef(ctx, *opts.caBundleSecretKey, *opts.caBundleNamespace)
		if err == nil {
			certs, err := pki.ParseCertificates([]byte(bundle))
			if err != nil {
				return nil, fmt.Errorf("error parsing bundle certificates: %v", err)
			}
			return certs, nil
		} else {
			logger.V(1).Info("error getting CA bundle", "err", err)
		}
	}

	if caKeyPair != nil {
		caCerts, err := caKeyPair.Certificates()
		if err != nil {
			return nil, fmt.Errorf("error getting CA certificates: %v", err)
		}
		return caCerts, nil
	}

	return nil, errors.New("unable to get CA bundle")
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
