package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/utils/ptr"
)

type MultiCluster struct {
	// MultiClusterSpec is the desired multi-cluster topology specification.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MultiClusterSpec `json:",inline"`
	// Enabled is a flag to enable the multi-cluster topology.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled,omitempty"`
}

type MultiClusterSpec struct {
	// Primary is the name of the primary cluster. It refers to a member in the 'members' field, containing its full specification.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Primary string `json:"primary,omitempty"`
	// Replicas is the name of all replica clusters. They refer to a member in the 'members' field, containing its full specification.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Replicas []string `json:"replicas,omitempty"`
	// Members is the specification of each member of the multi-cluster topology.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Members []MultiClusterMember `json:"members,omitempty"`
}

type MultiClusterMember struct {
	// Name is the identifier of the member.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Name string `json:"name"`
	// ExternalMariaDBRef holds a reference to an ExternalMariaDB with connection details to form the multi-cluster topology.
	// These connection details are utilized to setup remote replicas.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	ExternalMariaDBRef ObjectReference `json:"externalMariaDbRef,omitempty"`
}

// GetExternalMariaDBRefForMember allows for easy access to the ExternalMariaDBRef defined in the members list
func (c *MultiCluster) GetExternalMariaDBRefForMember(memberName string) (*ObjectReference, error) {
	members := c.Members
	for _, member := range members {
		if member.Name == memberName {
			return &member.ExternalMariaDBRef, nil
		}
	}
	return nil, fmt.Errorf("no externalMariaDBRef found for member %s", memberName)
}

// IsMultiClusterEnabled indicates whether the multi-cluster topology is enabled.
func (m *MariaDB) IsMultiClusterEnabled() bool {
	return ptr.Deref(m.Spec.MultiCluster, MultiCluster{}).Enabled
}

// IsMultiClusterPrimary indicates whether the current cluster is a primary cluster part of a multi-cluster topology.
func (m *MariaDB) IsMultiClusterPrimary() bool {
	return m.IsMultiClusterEnabled() && ptr.Deref(m.Spec.MultiCluster, MultiCluster{}).Primary == m.Name
}

// GetMultiClusterPrimary obtains the primary cluster member name.
func (m *MariaDB) GetMultiClusterPrimary() *string {
	if !m.IsMultiClusterEnabled() {
		return nil
	}
	return ptr.To(ptr.Deref(m.Spec.MultiCluster, MultiCluster{}).Primary)
}

// IsMultiClusterReplica indicates whether the current cluster is a replica cluster part of a multi-cluster topology.
func (m *MariaDB) IsMultiClusterReplica() bool {
	return m.IsMultiClusterEnabled() && ptr.Deref(m.Spec.MultiCluster, MultiCluster{}).Primary != m.Name
}

// IsMultiClusterPrimaryReplica determines whether a given Pod index is a primary Pod in a replica cluster.
func (m *MariaDB) IsMultiClusterPrimaryReplica(podIndex int) bool {
	return m.IsMultiClusterReplica() && m.Status.CurrentPrimaryPodIndex != nil && *m.Status.CurrentPrimaryPodIndex == podIndex
}

// HasConfiguredMultiCluster checks if ConditionTypeMultiClusterConfigured condition is true.
// If so, it indicates that multi-cluster replication has been configured. Only used for Galera.
func (m *MariaDB) HasConfiguredMultiCluster() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeMultiClusterConfigured)
}
