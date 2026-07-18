package builder

import (
	"time"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("BuildCertificate", func() {
	builder := newDefaultTestBuilder()
	key := types.NamespacedName{
		Name:      "test-cert",
		Namespace: "test",
	}
	owner := &mariadbv1alpha1.MariaDB{}

	DescribeTable("building a certificate",
		func(certOpts []CertOpt, wantErr bool) {
			_, err := builder.BuildCertificate(certOpts...)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("missing key",
			[]CertOpt{
				WithOwner(owner),
				WithDNSnames([]string{"example.com"}),
				WithLifetime(24 * time.Hour),
				WithRenewBeforePercentage(50),
				WithIssuerRef(cmmeta.IssuerReference{Name: "test-issuer"}),
			},
			true,
		),
		Entry("missing owner",
			[]CertOpt{
				WithKey(key),
				WithDNSnames([]string{"example.com"}),
				WithLifetime(24 * time.Hour),
				WithRenewBeforePercentage(50),
				WithIssuerRef(cmmeta.IssuerReference{Name: "test-issuer"}),
			},
			true,
		),
		Entry("missing DNS names",
			[]CertOpt{
				WithKey(key),
				WithOwner(owner),
				WithLifetime(24 * time.Hour),
				WithRenewBeforePercentage(50),
				WithIssuerRef(cmmeta.IssuerReference{Name: "test-issuer"}),
			},
			true,
		),
		Entry("missing lifetime",
			[]CertOpt{
				WithKey(key),
				WithOwner(owner),
				WithDNSnames([]string{"example.com"}),
				WithRenewBeforePercentage(50),
				WithIssuerRef(cmmeta.IssuerReference{Name: "test-issuer"}),
			},
			false,
		),
		Entry("missing renew before percentage",
			[]CertOpt{
				WithKey(key),
				WithOwner(owner),
				WithDNSnames([]string{"example.com"}),
				WithLifetime(24 * time.Hour),
				WithIssuerRef(cmmeta.IssuerReference{Name: "test-issuer"}),
			},
			false,
		),
		Entry("missing issuer ref",
			[]CertOpt{
				WithKey(key),
				WithOwner(owner),
				WithDNSnames([]string{"example.com"}),
				WithLifetime(24 * time.Hour),
				WithRenewBeforePercentage(50),
			},
			true,
		),
		Entry("valid options",
			[]CertOpt{
				WithKey(key),
				WithOwner(owner),
				WithDNSnames([]string{"example.com"}),
				WithLifetime(24 * time.Hour),
				WithRenewBeforePercentage(50),
				WithIssuerRef(cmmeta.IssuerReference{Name: "test-issuer"}),
			},
			false,
		),
	)
})
