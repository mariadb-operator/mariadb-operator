package discovery

import (
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	discoverypkg "k8s.io/client-go/discovery"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeClient "k8s.io/client-go/kubernetes/fake"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Discovery struct {
	client discovery.DiscoveryInterface
}

type DiscoveryOpt func(*Discovery)

func WithClient(client discovery.DiscoveryInterface) DiscoveryOpt {
	return func(d *Discovery) {
		d.client = client
	}
}

type NewDiscoveryFn func(opts ...DiscoveryOpt) (*Discovery, error)

func NewDiscovery(opts ...DiscoveryOpt) (*Discovery, error) {
	discovery := Discovery{}
	for _, setOpt := range opts {
		setOpt(&discovery)
	}
	if discovery.client == nil {
		config, err := ctrl.GetConfig()
		if err != nil {
			return nil, err
		}
		client, err := discoverypkg.NewDiscoveryClientForConfig(config)
		if err != nil {
			return nil, err
		}
		discovery.client = client
	}
	return &discovery, nil
}

func NewFakeDiscovery(resources ...*metav1.APIResourceList) (*Discovery, error) {
	client := fakeClient.NewSimpleClientset()
	fakeDiscovery, ok := client.Discovery().(*fakeDiscovery.FakeDiscovery)
	if !ok {
		return nil, fmt.Errorf("unable to cast discovery client to FakeDiscovery")
	}
	if resources != nil {
		fakeDiscovery.Resources = resources
	}
	return NewDiscovery(
		WithClient(fakeDiscovery),
	)
}

func (c *Discovery) ServiceMonitorExist() (bool, error) {
	return c.resourceExist("monitoring.coreos.com/v1", "servicemonitors")
}

func (c *Discovery) CertificateExist() (bool, error) {
	return c.resourceExist("cert-manager.io/v1", "certificates")
}

func (c *Discovery) SecurityContextConstraintsExist() (bool, error) {
	return c.resourceExist("security.openshift.io/v1", "securitycontextconstraints")
}

func (c *Discovery) LogInfo(logger logr.Logger) error {
	logger.Info("Discovery info")
	svcMonitor, err := c.ServiceMonitorExist()
	if err != nil {
		return err
	}
	cert, err := c.CertificateExist()
	if err != nil {
		return err
	}
	scc, err := c.SecurityContextConstraintsExist()
	if err != nil {
		return err
	}
	logger.Info("Resources",
		"ServiceMonitor", svcMonitor,
		"Certificate", cert,
		"SecurityContextConstraints", scc,
	)
	return nil
}

func (c *Discovery) resourceExist(groupVersion, kind string) (bool, error) {
	apiResourceList, err := c.client.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	for _, resource := range apiResourceList.APIResources {
		if resource.Name == kind {
			return true, nil
		}
	}
	return false, nil
}
