package builder

import (
	"strings"
	"testing"
	"time"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				WithIssuerRef(cmmeta.IssuerReference{Name: "test-issuer"}),
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
				WithIssuerRef(cmmeta.IssuerReference{Name: "test-issuer"}),
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
				WithIssuerRef(cmmeta.IssuerReference{Name: "test-issuer"}),
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
				WithIssuerRef(cmmeta.IssuerReference{Name: "test-issuer"}),
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
				WithIssuerRef(cmmeta.IssuerReference{Name: "test-issuer"}),
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
				WithIssuerRef(cmmeta.IssuerReference{Name: "test-issuer"}),
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

// cert-manager rejects a Certificate whose commonName is longer than 64 bytes, which the full
// Service name reaches with a long MariaDB name and namespace.
func TestBuildCertificateCommonName(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	longName := "dcp-mdb-mymaria-staging1-1211"
	tests := []struct {
		name           string
		dnsNames       []string
		wantCommonName string
	}{
		{
			name: "first DNS name fits",
			dnsNames: (&mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{Name: "mariadb", Namespace: "default"},
			}).TLSServerDNSNames(),
			wantCommonName: "mariadb.default.svc.cluster.local",
		},
		{
			name: "first DNS name too long",
			dnsNames: (&mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{Name: longName, Namespace: longName},
			}).TLSServerDNSNames(),
			wantCommonName: longName + "." + longName + ".svc",
		},
		{
			name:           "no DNS name fits",
			dnsNames:       []string{strings.Repeat("a", 65)},
			wantCommonName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, err := builder.BuildCertificate(
				WithKey(types.NamespacedName{Name: "test-cert", Namespace: "test"}),
				WithOwner(&mariadbv1alpha1.MariaDB{}),
				WithDNSnames(tt.dnsNames),
				WithIssuerRef(cmmeta.IssuerReference{Name: "test-issuer"}),
			)
			if err != nil {
				t.Fatalf("BuildCertificate() unexpected error: %v", err)
			}
			if cert.Spec.CommonName != tt.wantCommonName {
				t.Errorf("BuildCertificate() commonName = %q, want %q", cert.Spec.CommonName, tt.wantCommonName)
			}
		})
	}
}
