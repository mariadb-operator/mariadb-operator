package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	discoveryv1 "k8s.io/api/discovery/v1"

	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("EndpointsMeta", func() {
	builder := newDefaultTestBuilder()
	key := types.NamespacedName{
		Name: "endpoints",
	}
	addressType := discoveryv1.AddressTypeIPv4
	endpoints := []discoveryv1.Endpoint{}
	ports := []discoveryv1.EndpointPort{}
	serviceName := "test"

	DescribeTable(
		"should build the expected Endpoints meta",
		func(mariadb *mariadbv1alpha1.MariaDB, wantMeta *mariadbv1alpha1.Metadata) {
			endpoints, err := builder.BuildEndpointSlice(key, mariadb, addressType, endpoints, ports, serviceName)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&endpoints.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
		},
		Entry(
			"no meta",
			&mariadbv1alpha1.MariaDB{},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					metadata.KubernetesEndpointSliceManagedByLabel: metadata.KubernetesEndpointSliceManagedByValue,
					metadata.KubernetesServiceLabel:                serviceName,
				},
				Annotations: map[string]string{},
			},
		),
		Entry(
			"meta",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
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
					metadata.KubernetesEndpointSliceManagedByLabel: metadata.KubernetesEndpointSliceManagedByValue,
					metadata.KubernetesServiceLabel:                serviceName,
					"database.myorg.io":                            "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
	)
})
