package discovery

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
)

type DiscoveryClient struct {
	discovery.DiscoveryClient
}

func NewDiscoveryClient() (*DiscoveryClient, error) {
	config, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	return &DiscoveryClient{
		DiscoveryClient: *discoveryClient,
	}, nil
}

func (c *DiscoveryClient) ServiceMonitorExist() (bool, error) {
	return c.resourceExist("monitoring.coreos.com/v1", "servicemonitors")
}

func (c *DiscoveryClient) SecurityContextConstrainstsExist() (bool, error) {
	return c.resourceExist("security.openshift.io/v1", "securitycontextconstraints")
}

func (c *DiscoveryClient) resourceExist(groupVersion, kind string) (bool, error) {
	apiResourceList, err := c.DiscoveryClient.ServerResourcesForGroupVersion(groupVersion)
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
