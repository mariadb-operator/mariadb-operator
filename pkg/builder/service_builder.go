package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func MariaDBPort(svc *corev1.Service) (*v1.ServicePort, error) {
	for _, p := range svc.Spec.Ports {
		if p.Name == MariaDbPortName {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("Service port not found")
}

type ServiceOpts struct {
	Selectorlabels           map[string]string
	Annotations              map[string]string
	Type                     corev1.ServiceType
	Ports                    []corev1.ServicePort
	ClusterIP                *string
	PublishNotReadyAddresses *bool
}

func (b *Builder) BuildService(mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName,
	opts ServiceOpts) (*corev1.Service, error) {
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMariaDBSelectorLabels(mariadb).
			WithLabels(opts.Selectorlabels).
			Build()
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			WithAnnotations(opts.Annotations).
			WithLabels(selectorLabels).
			Build()
	svc := &corev1.Service{
		ObjectMeta: objMeta,
		Spec: corev1.ServiceSpec{
			Type:     opts.Type,
			Ports:    opts.Ports,
			Selector: selectorLabels,
		},
	}
	if opts.ClusterIP != nil {
		svc.Spec.ClusterIP = *opts.ClusterIP
	}
	if opts.PublishNotReadyAddresses != nil {
		svc.Spec.PublishNotReadyAddresses = *opts.PublishNotReadyAddresses
	}
	if err := controllerutil.SetControllerReference(mariadb, svc, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Service: %v", err)
	}
	return svc, nil
}
