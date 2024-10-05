package builder

import (
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func MariaDBPort(svc *corev1.Service) (*v1.ServicePort, error) {
	for _, p := range svc.Spec.Ports {
		if p.Name == MariadbPortName {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("Service port not found")
}

func ValidateServicePorts(ports []corev1.ServicePort) error {
	nameMap := make(map[string]struct{}, len(ports))
	portMap := make(map[int32]struct{}, len(ports))
	for _, p := range ports {
		if p.Name == "" {
			return fmt.Errorf("Port number %d has no name set", p.Port)
		}
		if _, exists := nameMap[p.Name]; exists {
			return fmt.Errorf("Port name %s is already taken by another port", p.Name)
		}
		if _, exists := portMap[p.Port]; exists {
			return fmt.Errorf("Port number %d is already taken by another port", p.Port)
		}
		nameMap[p.Name] = struct{}{}
		portMap[p.Port] = struct{}{}
	}
	return nil
}

type ServiceOpts struct {
	mariadbv1alpha1.ServiceTemplate
	SelectorLabels        map[string]string
	ExcludeSelectorLabels bool
	Ports                 []corev1.ServicePort
	Headless              bool
	ExtraMeta             *mariadbv1alpha1.Metadata
}

func (b *Builder) BuildService(key types.NamespacedName, owner metav1.Object, opts ServiceOpts) (*corev1.Service, error) {
	if !opts.ExcludeSelectorLabels && opts.SelectorLabels == nil {
		return nil, errors.New("SelectorLabels are mandatory when ExcludeSelectorLabels is set to false")
	}
	if len(opts.Ports) > 1 {
		if err := ValidateServicePorts(opts.Ports); err != nil {
			return nil, fmt.Errorf("error validating Ports: %v", err)
		}
	}
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(opts.ExtraMeta).
			WithMetadata(opts.Metadata).
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
		svc.Spec.Selector = opts.SelectorLabels
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
	if opts.AllocateLoadBalancerNodePorts != nil {
		svc.Spec.AllocateLoadBalancerNodePorts = opts.AllocateLoadBalancerNodePorts
	}
	if err := controllerutil.SetControllerReference(owner, svc, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Service: %v", err)
	}
	return svc, nil
}
