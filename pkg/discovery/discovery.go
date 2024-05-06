package discovery

import (
	"errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Discovery struct {
	client discovery.DiscoveryInterface
}

func NewDiscovery() (*Discovery, error) {
	config, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}
	client, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	return &Discovery{
		client: client,
	}, nil
}

func NewDiscoveryWithClient(client discovery.DiscoveryInterface) (*Discovery, error) {
	if client == nil {
		return nil, errors.New("client should not be nil")
	}
	return &Discovery{
		client: client,
	}, nil
}

func (c *Discovery) ServiceMonitorExist() (bool, error) {
	return c.resourceExist("monitoring.coreos.com/v1", "servicemonitors")
}

func (c *Discovery) SecurityContextConstrainstsExist() (bool, error) {
	return c.resourceExist("security.openshift.io/v1", "securitycontextconstraints")
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
