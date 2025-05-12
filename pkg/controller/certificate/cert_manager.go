package certificate

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *CertReconciler) reconcileCertManagerCert(ctx context.Context, opts *CertReconcilerOpts, logger logr.Logger) (ctrl.Result, error) {
	if r.discovery == nil || r.builder == nil {
		return ctrl.Result{}, errors.New("discovery and builder must be initialized")
	}

	certExists, err := r.discovery.CertificateExist()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error checking Certificate availability in the cluster: %w", err)
	}
	if !certExists {
		r.recorder.Event(opts.relatedObject, corev1.EventTypeWarning, mariadbv1alpha1.ReasonCRDNotFound,
			"Unable to reconcile certificate: Certificate CRD not installed in the cluster")
		logger.Error(errors.New("Certificate CRD not installed in the cluster"), "Unable to reconcile certificate")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	desiredCert, err := r.buildCertManagerCert(opts, logger)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building desired cert: %v", err)
	}
	if err := r.reconcileCertManagerDesiredCert(ctx, opts, desiredCert); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling desired cert: %v", err)
	}

	if err := r.certManagerCertReady(ctx, opts); err != nil {
		logger.V(1).Info("Certificate not ready. Requeuing...", "err", err)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *CertReconciler) buildCertManagerCert(opts *CertReconcilerOpts, logger logr.Logger) (*certmanagerv1.Certificate, error) {
	certOpts := []builder.CertOpt{
		builder.WithKey(opts.certSecretKey),
		builder.WithOwner(opts.relatedObject),
		builder.WithDNSnames(opts.certDNSNames),
		builder.WithLifetime(opts.certLifetime),
		builder.WithUsages(certManagerKeyUsages(opts, logger)...),
		builder.WithIssuerRef(*opts.certIssuerRef),
	}
	cert, err := r.builder.BuildCertificate(certOpts...)
	if err != nil {
		return nil, fmt.Errorf("error building Certificate: %v", err)
	}
	return cert, nil
}

func (r *CertReconciler) reconcileCertManagerDesiredCert(ctx context.Context, opts *CertReconcilerOpts,
	desiredCert *certmanagerv1.Certificate) error {
	var existingCert certmanagerv1.Certificate
	if err := r.Get(ctx, opts.certSecretKey, &existingCert); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting Certificate: %v", err)
		}
		if err := r.Create(ctx, desiredCert); err != nil {
			return fmt.Errorf("error creating Certificate: %v", err)
		}
		return nil
	}

	patch := client.MergeFrom(existingCert.DeepCopy())
	existingCert.Spec.Duration = desiredCert.Spec.Duration
	existingCert.Spec.DNSNames = desiredCert.Spec.DNSNames
	existingCert.Spec.CommonName = desiredCert.Spec.CommonName
	existingCert.Spec.Usages = desiredCert.Spec.Usages
	existingCert.Spec.IssuerRef = desiredCert.Spec.IssuerRef
	existingCert.Spec.SecretName = desiredCert.Spec.SecretName
	return r.Patch(ctx, &existingCert, patch)
}

func (r *CertReconciler) certManagerCertReady(ctx context.Context, opts *CertReconcilerOpts) error {
	var cert certmanagerv1.Certificate
	if err := r.Get(ctx, opts.certSecretKey, &cert); err != nil {
		return fmt.Errorf("error getting cert: %w", err)
	}
	for _, condition := range cert.Status.Conditions {
		if condition.Type != certmanagerv1.CertificateConditionReady {
			continue
		}
		if condition.Status == cmmeta.ConditionTrue {
			return nil
		} else {
			return fmt.Errorf("Certificate '%s' not ready: %s", opts.certSecretKey.Name, condition.Message)
		}
	}
	return fmt.Errorf("Certificate '%s' not ready", opts.certSecretKey.Name)
}

func certManagerKeyUsages(opts *CertReconcilerOpts, logger logr.Logger) []certmanagerv1.KeyUsage {
	var usages []certmanagerv1.KeyUsage
	if opts.certKeyUsage != 0 {
		usages = append(usages, mapX509KeyUsageToCertManager(opts.certKeyUsage)...)
	}
	usages = append(usages, mapX509ExtKeyUsageToCertManager(opts.certExtKeyUsage, logger)...)
	return usages
}

func mapX509KeyUsageToCertManager(usage x509.KeyUsage) []certmanagerv1.KeyUsage {
	var cmUsages []certmanagerv1.KeyUsage

	if usage&x509.KeyUsageDigitalSignature != 0 {
		cmUsages = append(cmUsages, certmanagerv1.UsageDigitalSignature)
	}
	if usage&x509.KeyUsageContentCommitment != 0 {
		cmUsages = append(cmUsages, certmanagerv1.UsageContentCommitment)
	}
	if usage&x509.KeyUsageKeyEncipherment != 0 {
		cmUsages = append(cmUsages, certmanagerv1.UsageKeyEncipherment)
	}
	if usage&x509.KeyUsageDataEncipherment != 0 {
		cmUsages = append(cmUsages, certmanagerv1.UsageDataEncipherment)
	}
	if usage&x509.KeyUsageKeyAgreement != 0 {
		cmUsages = append(cmUsages, certmanagerv1.UsageKeyAgreement)
	}
	if usage&x509.KeyUsageCertSign != 0 {
		cmUsages = append(cmUsages, certmanagerv1.UsageCertSign)
	}
	if usage&x509.KeyUsageCRLSign != 0 {
		cmUsages = append(cmUsages, certmanagerv1.UsageCRLSign)
	}
	if usage&x509.KeyUsageEncipherOnly != 0 {
		cmUsages = append(cmUsages, certmanagerv1.UsageEncipherOnly)
	}
	if usage&x509.KeyUsageDecipherOnly != 0 {
		cmUsages = append(cmUsages, certmanagerv1.UsageDecipherOnly)
	}

	return cmUsages
}

func mapX509ExtKeyUsageToCertManager(extUsages []x509.ExtKeyUsage, logger logr.Logger) []certmanagerv1.KeyUsage {
	var cmUsages []certmanagerv1.KeyUsage

	for _, usage := range extUsages {
		switch usage {
		case x509.ExtKeyUsageAny:
			cmUsages = append(cmUsages, certmanagerv1.UsageAny)
		case x509.ExtKeyUsageServerAuth:
			cmUsages = append(cmUsages, certmanagerv1.UsageServerAuth)
		case x509.ExtKeyUsageClientAuth:
			cmUsages = append(cmUsages, certmanagerv1.UsageClientAuth)
		case x509.ExtKeyUsageCodeSigning:
			cmUsages = append(cmUsages, certmanagerv1.UsageCodeSigning)
		case x509.ExtKeyUsageEmailProtection:
			cmUsages = append(cmUsages, certmanagerv1.UsageEmailProtection)
		case x509.ExtKeyUsageIPSECEndSystem:
			cmUsages = append(cmUsages, certmanagerv1.UsageIPsecEndSystem)
		case x509.ExtKeyUsageIPSECTunnel:
			cmUsages = append(cmUsages, certmanagerv1.UsageIPsecTunnel)
		case x509.ExtKeyUsageIPSECUser:
			cmUsages = append(cmUsages, certmanagerv1.UsageIPsecUser)
		case x509.ExtKeyUsageTimeStamping:
			cmUsages = append(cmUsages, certmanagerv1.UsageTimestamping)
		case x509.ExtKeyUsageOCSPSigning:
			cmUsages = append(cmUsages, certmanagerv1.UsageOCSPSigning)
		case x509.ExtKeyUsageMicrosoftServerGatedCrypto:
			cmUsages = append(cmUsages, certmanagerv1.UsageMicrosoftSGC)
		case x509.ExtKeyUsageNetscapeServerGatedCrypto:
			cmUsages = append(cmUsages, certmanagerv1.UsageNetscapeSGC)
		default:
			logger.Error(errors.New("unsupported x509 key usage"), "Unsupported ExtKeyUsage encountered", "key-usage", usage)
			continue
		}
	}

	return cmUsages
}
