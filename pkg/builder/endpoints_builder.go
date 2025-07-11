package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metadatabuilder "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (b *Builder) BuildEndpointSlice(key types.NamespacedName, mariadb *mariadbv1alpha1.MariaDB,
	addressType discoveryv1.AddressType, endpoints []discoveryv1.Endpoint, ports []discoveryv1.EndpointPort,
	serviceName string) (*discoveryv1.EndpointSlice, error) {
	objMeta :=
		metadatabuilder.NewMetadataBuilder(key).
			WithMetadata(mariadb.Spec.InheritMetadata).
			WithLabels(map[string]string{
				metadata.KubernetesEndpointSliceManagedByLabel: metadata.KubernetesEndpointSliceManagedByValue,
				metadata.KubernetesServiceLabel:                serviceName,
			}).
			Build()
	endpointSlice := &discoveryv1.EndpointSlice{
		ObjectMeta:  objMeta,
		AddressType: addressType,
		Endpoints:   endpoints,
		Ports:       ports,
	}
	if err := controllerutil.SetControllerReference(mariadb, endpointSlice, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to EndpointSlice: %v", err)
	}
	return endpointSlice, nil
}
