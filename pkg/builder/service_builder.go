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
	mariadbv1alpha1.ServiceTemplate
	Selectorlabels        map[string]string
	ExcludeSelectorLabels bool
	Ports                 []corev1.ServicePort
	Headless              bool
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
			WithLabels(opts.Labels).
			Build()
	svc := &corev1.Service{
		ObjectMeta: objMeta,
		Spec: corev1.ServiceSpec{
			Type:  opts.Type,
			Ports: opts.Ports,
		},
	}
	if opts.Headless {
		svc.Spec.ClusterIP = "None"
		svc.Spec.PublishNotReadyAddresses = true
	}
	if !opts.ExcludeSelectorLabels {
		svc.Spec.Selector = selectorLabels
	}
	if opts.LoadBalancerIP != nil {
		svc.Spec.LoadBalancerIP = *opts.LoadBalancerIP
	}
	if opts.LoadBalancerSourceRanges != nil {
		svc.Spec.LoadBalancerSourceRanges = opts.LoadBalancerSourceRanges
	}
	if opts.ExternalTrafficPolicy != nil {
		svc.Spec.ExternalTrafficPolicy = *opts.ExternalTrafficPolicy
	}
	if opts.SessionAffinity != nil {
		svc.Spec.SessionAffinity = *opts.SessionAffinity
	}
	if err := controllerutil.SetControllerReference(mariadb, svc, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Service: %v", err)
	}
	return svc, nil
}
