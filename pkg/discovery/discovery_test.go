package discovery

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Discovery", func() {
	DescribeTable("discovering resources",
		func(group, kind string, discoveryFn func(d *Discovery) (bool, error)) {
			resource := &metav1.APIResourceList{
				GroupVersion: group,
				APIResources: []metav1.APIResource{
					{
						Name: kind,
					},
				},
			}
			discovery, err := NewFakeDiscovery(resource)
			Expect(err).NotTo(HaveOccurred())

			exists, err := discoveryFn(discovery)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())

			discovery, err = NewFakeDiscovery()
			Expect(err).NotTo(HaveOccurred())

			exists, err = discoveryFn(discovery)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		},
		Entry("ServiceMonitors",
			"monitoring.coreos.com/v1",
			"servicemonitors",
			func(d *Discovery) (bool, error) {
				return d.ServiceMonitorExist()
			}),
		Entry("Certificates",
			"cert-manager.io/v1",
			"certificates",
			func(d *Discovery) (bool, error) {
				return d.CertificateExist()
			}),
		Entry("VolumeSnapshots",
			"snapshot.storage.k8s.io/v1",
			"volumesnapshots",
			func(d *Discovery) (bool, error) {
				return d.VolumeSnapshotExist()
			}),
		Entry("SecurityContextConstraints",
			"security.openshift.io/v1",
			"securitycontextconstraints",
			func(d *Discovery) (bool, error) {
				return d.SecurityContextConstraintsExist()
			}),
	)
})
