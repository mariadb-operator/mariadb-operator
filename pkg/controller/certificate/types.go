package certificate

import (
	"context"

	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type SecretType int

const (
	SecretTypeCA SecretType = iota
	SecretTypeTLS
)

type CertHandler interface {
	ShouldRenewCert(ctx context.Context, caKeyPair *pki.KeyPair) (shouldRenew bool, reason string, err error)
	HandleExpiredCert(ctx context.Context) error
}

type DefaultCertHandler struct{}

func (h *DefaultCertHandler) ShouldRenewCert(ctx context.Context, caKeyPair *pki.KeyPair) (shouldRenew bool, reason string, err error) {
	return true, "Certificate lifetime within renewal window", nil
}

func (h *DefaultCertHandler) HandleExpiredCert(ctx context.Context) error {
	// noop
	return nil
}

type RelatedObject interface {
	runtime.Object
	metav1.Object
}
