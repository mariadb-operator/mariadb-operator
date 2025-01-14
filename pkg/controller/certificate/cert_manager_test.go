package certificate

import (
	"crypto/x509"
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
)

func TestCertManagerKeyUsages(t *testing.T) {
	tests := []struct {
		name           string
		opts           *CertReconcilerOpts
		expectedUsages []certmanagerv1.KeyUsage
	}{
		{
			name: "No key usages",
			opts: &CertReconcilerOpts{
				certKeyUsage:    0,
				certExtKeyUsage: nil,
			},
			expectedUsages: []certmanagerv1.KeyUsage{},
		},
		{
			name: "Single key usage: DigitalSignature",
			opts: &CertReconcilerOpts{
				certKeyUsage:    x509.KeyUsageDigitalSignature,
				certExtKeyUsage: nil,
			},
			expectedUsages: []certmanagerv1.KeyUsage{certmanagerv1.UsageDigitalSignature},
		},
		{
			name: "Multiple key usages",
			opts: &CertReconcilerOpts{
				certKeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
				certExtKeyUsage: []x509.ExtKeyUsage{
					x509.ExtKeyUsageServerAuth,
					x509.ExtKeyUsageClientAuth,
				},
			},
			expectedUsages: []certmanagerv1.KeyUsage{
				certmanagerv1.UsageDigitalSignature,
				certmanagerv1.UsageKeyEncipherment,
				certmanagerv1.UsageServerAuth,
				certmanagerv1.UsageClientAuth,
			},
		},
		{
			name: "Unsupported ExtKeyUsage",
			opts: &CertReconcilerOpts{
				certKeyUsage: 0,
				certExtKeyUsage: []x509.ExtKeyUsage{
					x509.ExtKeyUsageTimeStamping,
					99, // Unsupported ExtKeyUsage
				},
			},
			expectedUsages: []certmanagerv1.KeyUsage{certmanagerv1.UsageTimestamping},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usages := certManagerKeyUsages(tt.opts, logr.Discard())

			if len(usages) != len(tt.expectedUsages) {
				t.Errorf("unexpected number of usages: got %d, want %d", len(usages), len(tt.expectedUsages))
			}

			for i, usage := range usages {
				if usage != tt.expectedUsages[i] {
					t.Errorf("unexpected usage at index %d: got %v, want %v", i, usage, tt.expectedUsages[i])
				}
			}
		})
	}
}
