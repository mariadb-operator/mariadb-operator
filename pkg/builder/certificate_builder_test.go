package builder

import (
	"testing"
	"time"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

func TestBuildCertificate(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name:      "test-cert",
		Namespace: "test",
	}
	owner := &mariadbv1alpha1.MariaDB{}
	tests := []struct {
		name     string
		certOpts []CertOpt
		wantErr  bool
	}{
		{
			name: "missing key",
			certOpts: []CertOpt{
				WithOwner(owner),
				WithDNSnames([]string{"example.com"}),
				WithLifetime(24 * time.Hour),
				WithRenewBeforePercentage(50),
				WithIssuerRef(cmmeta.ObjectReference{Name: "test-issuer"}),
			},
			wantErr: true,
		},
		{
			name: "missing owner",
			certOpts: []CertOpt{
				WithKey(key),
				WithDNSnames([]string{"example.com"}),
				WithLifetime(24 * time.Hour),
				WithRenewBeforePercentage(50),
				WithIssuerRef(cmmeta.ObjectReference{Name: "test-issuer"}),
			},
			wantErr: true,
		},
		{
			name: "missing DNS names",
			certOpts: []CertOpt{
				WithKey(key),
				WithOwner(owner),
				WithLifetime(24 * time.Hour),
				WithRenewBeforePercentage(50),
				WithIssuerRef(cmmeta.ObjectReference{Name: "test-issuer"}),
			},
			wantErr: true,
		},
		{
			name: "missing lifetime",
			certOpts: []CertOpt{
				WithKey(key),
				WithOwner(owner),
				WithDNSnames([]string{"example.com"}),
				WithRenewBeforePercentage(50),
				WithIssuerRef(cmmeta.ObjectReference{Name: "test-issuer"}),
			},
			wantErr: false,
		},
		{
			name: "missing renew before percentage",
			certOpts: []CertOpt{
				WithKey(key),
				WithOwner(owner),
				WithDNSnames([]string{"example.com"}),
				WithLifetime(24 * time.Hour),
				WithIssuerRef(cmmeta.ObjectReference{Name: "test-issuer"}),
			},
			wantErr: false,
		},
		{
			name: "missing issuer ref",
			certOpts: []CertOpt{
				WithKey(key),
				WithOwner(owner),
				WithDNSnames([]string{"example.com"}),
				WithLifetime(24 * time.Hour),
				WithRenewBeforePercentage(50),
			},
			wantErr: true,
		},
		{
			name: "valid options",
			certOpts: []CertOpt{
				WithKey(key),
				WithOwner(owner),
				WithDNSnames([]string{"example.com"}),
				WithLifetime(24 * time.Hour),
				WithRenewBeforePercentage(50),
				WithIssuerRef(cmmeta.ObjectReference{Name: "test-issuer"}),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := builder.BuildCertificate(tt.certOpts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildCertificate() error = %v, expectError %v", err, tt.wantErr)
			}
		})
	}
}
