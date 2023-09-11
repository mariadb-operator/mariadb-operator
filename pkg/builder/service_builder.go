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
	ExcludeSelectorLabels    bool
	Annotations              map[string]string
	Type                     corev1.ServiceType
	Ports                    []corev1.ServicePort
	ClusterIP                *string
	PublishNotReadyAddresses *bool
	ExternalTrafficPolicy    string
	LoadBalancerSourceRanges []string
	LoadBalancerIp           string
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
			Type:  opts.Type,
			Ports: opts.Ports,
		},
	}
	if opts.ClusterIP != nil {
		svc.Spec.ClusterIP = *opts.ClusterIP
	}
	if opts.PublishNotReadyAddresses != nil {
		svc.Spec.PublishNotReadyAddresses = *opts.PublishNotReadyAddresses
	}
	if opts.ExternalTrafficPolicy != nil {
		svc.Spec.ExternalTrafficPolicy = *opts.ExternalTrafficPolicy
	}
	if opts.LoadBalancerIp != nil {
		svc.Spec.LoadBalancerIp = *opts.LoadBalancerIp
	}
	if opts.LoadBalancerSourceRanges != nil {
		svc.Spec.LoadBalancerSourceRanges = *opts.LoadBalancerSourceRanges
	}
	if !opts.ExcludeSelectorLabels {
		svc.Spec.Selector = selectorLabels
	} 
	if err := controllerutil.SetControllerReference(mariadb, svc, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Service: %v", err)
	}
	return svc, nil
}
