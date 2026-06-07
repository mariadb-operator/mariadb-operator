package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ConfigMapMeta", func() {
	builder := newDefaultTestBuilder()
	DescribeTable("ConfigMapMeta",
		func(opts ConfigMapOpts, wantMeta *mariadbv1alpha1.Metadata) {
			configMap, err := builder.BuildConfigMap(opts, &mariadbv1alpha1.MariaDB{})
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&configMap.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
		},
		Entry("no meta",
			ConfigMapOpts{
				Key: types.NamespacedName{
					Name: "configmap",
				},
				Data: map[string]string{
					"my.cnf": "test",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry("meta",
			ConfigMapOpts{
				Key: types.NamespacedName{
					Name: "configmap",
				},
				Data: map[string]string{
					"my.cnf": "test",
				},
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
