package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
)

var _ = Describe("PodDisruptionBudgetMeta", func() {
	var builder *Builder

	BeforeEach(func() {
		builder = newDefaultTestBuilder()
	})

	DescribeTable("BuildPodDisruptionBudget",
		func(opts PodDisruptionBudgetOpts, wantMeta *mariadbv1alpha1.Metadata) {
			configMap, err := builder.BuildPodDisruptionBudget(opts, &mariadbv1alpha1.MariaDB{})
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&configMap.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
		},
		Entry("no meta",
			PodDisruptionBudgetOpts{},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry("meta",
			PodDisruptionBudgetOpts{
				Metadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"database.myorg.io": "mariadb",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mariadb",
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
