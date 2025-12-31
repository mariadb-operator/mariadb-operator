package discovery

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	hashversion "github.com/hashicorp/go-version"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sversion "k8s.io/apimachinery/pkg/version"
	discoverypkg "k8s.io/client-go/discovery"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeClient "k8s.io/client-go/kubernetes/fake"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Discovery struct {
	client discoverypkg.DiscoveryInterface
}

type DiscoveryOpt func(*Discovery)

func WithClient(client discoverypkg.DiscoveryInterface) DiscoveryOpt {
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
	client := fakeClient.NewClientset()
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

func NewFakeDiscoveryWithServerVersion(serverVersion *k8sversion.Info, resources ...*metav1.APIResourceList) (*Discovery, error) {
	client := fakeClient.NewClientset()
	fakeDiscovery, ok := client.Discovery().(*fakeDiscovery.FakeDiscovery)
	if !ok {
		return nil, fmt.Errorf("unable to cast discovery client to FakeDiscovery")
	}
	fakeDiscovery.FakedServerVersion = serverVersion
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

func (c *Discovery) VolumeSnapshotExist() (bool, error) {
	return c.resourceExist("snapshot.storage.k8s.io/v1", "volumesnapshots")
}

func (c *Discovery) SecurityContextConstraintsExist() (bool, error) {
	return c.resourceExist("security.openshift.io/v1", "securitycontextconstraints")
}

func (c *Discovery) ServerVersionAtLeast(minVersion string) (bool, error) {
	serverVersionInfo, err := c.client.ServerVersion()
	if err != nil {
		return false, err
	}
	serverVersion, err := hashversion.NewSemver(strings.TrimPrefix(serverVersionInfo.GitVersion, "v"))
	if err != nil {
		return false, fmt.Errorf("error parsing server version %q: %v", serverVersionInfo.GitVersion, err)
	}
	targetVersion, err := hashversion.NewSemver(minVersion)
	if err != nil {
		return false, fmt.Errorf("error parsing target version %q: %v", minVersion, err)
	}
	return serverVersion.GreaterThanOrEqual(targetVersion), nil
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
