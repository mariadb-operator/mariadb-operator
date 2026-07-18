package certificate

import (
	"crypto/x509"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CertManagerKeyUsages", func() {
	DescribeTable("returns cert-manager key usages",
		func(opts *CertReconcilerOpts, expectedUsages []certmanagerv1.KeyUsage) {
			usages := certManagerKeyUsages(opts, logr.Discard())
			Expect(usages).To(HaveLen(len(expectedUsages)))
			for i := range usages {
				Expect(usages[i]).To(Equal(expectedUsages[i]))
			}
		},
		Entry("No key usages",
			&CertReconcilerOpts{
				certKeyUsage:    0,
				certExtKeyUsage: nil,
			},
			[]certmanagerv1.KeyUsage{},
		),
		Entry("Single key usage: DigitalSignature",
			&CertReconcilerOpts{
				certKeyUsage:    x509.KeyUsageDigitalSignature,
				certExtKeyUsage: nil,
			},
			[]certmanagerv1.KeyUsage{certmanagerv1.UsageDigitalSignature},
		),
		Entry("Multiple key usages",
			&CertReconcilerOpts{
				certKeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
				certExtKeyUsage: []x509.ExtKeyUsage{
					x509.ExtKeyUsageServerAuth,
					x509.ExtKeyUsageClientAuth,
				},
			},
			[]certmanagerv1.KeyUsage{
				certmanagerv1.UsageDigitalSignature,
				certmanagerv1.UsageKeyEncipherment,
				certmanagerv1.UsageServerAuth,
				certmanagerv1.UsageClientAuth,
			},
		),
		Entry("Unsupported ExtKeyUsage",
			&CertReconcilerOpts{
				certKeyUsage: 0,
				certExtKeyUsage: []x509.ExtKeyUsage{
					x509.ExtKeyUsageTimeStamping,
					99, // Unsupported ExtKeyUsage
				},
			},
			[]certmanagerv1.KeyUsage{certmanagerv1.UsageTimestamping},
		),
	)
})
