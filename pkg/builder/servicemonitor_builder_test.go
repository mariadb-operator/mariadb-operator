package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/utils/ptr"
)

var _ = Describe("ServiceMonitorMeta", func() {
	builder := newDefaultTestBuilder()

	DescribeTable("should build ServiceMonitor with expected meta",
		func(mariadb *mariadbv1alpha1.MariaDB, wantMeta *mariadbv1alpha1.Metadata) {
			svcMonitor, err := builder.BuildServiceMonitor(mariadb)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&svcMonitor.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
		},
		Entry("no meta",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry("with meta",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
	)
})

var _ = Describe("RoleRelabelConfig", func() {
	DescribeTable("should build the role relabel config",
		func(podIndex int, primaryPodIndex *int, wantRole string) {
			relabelConfig := roleRelabelConfig(podIndex, primaryPodIndex)
			if wantRole == "" {
				Expect(relabelConfig).To(BeNil())
				return
			}
			Expect(relabelConfig).To(HaveLen(1))
			cfg := relabelConfig[0]
			Expect(cfg.Action).To(Equal("replace"))
			Expect(cfg.TargetLabel).To(Equal("role"))
			Expect(cfg.Replacement).NotTo(BeNil())
			Expect(*cfg.Replacement).To(Equal(wantRole))
		},
		Entry("primary index is nil", 0, nil, ""),
		Entry("pod is primary", 0, ptr.To(0), "primary"),
		Entry("pod is replica", 1, ptr.To(0), "replica"),
	)
})
