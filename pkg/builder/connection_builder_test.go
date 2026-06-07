package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ConnectionMeta", func() {
	builder := newDefaultTestBuilder()
	DescribeTable(
		"should build Connection metadata",
		func(opts ConnectionOpts, wantMeta *mariadbv1alpha1.Metadata) {
			configMap, err := builder.BuildConnection(opts, &mariadbv1alpha1.MariaDB{})
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&configMap.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
		},
		Entry(
			"no meta",
			ConnectionOpts{
				Key: types.NamespacedName{
					Name: "connection",
				},
				Metadata: &mariadbv1alpha1.Metadata{},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry(
			"meta",
			ConnectionOpts{
				Key: types.NamespacedName{
					Name: "connection",
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
