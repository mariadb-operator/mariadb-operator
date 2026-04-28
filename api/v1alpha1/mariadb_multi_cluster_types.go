package v1alpha1

import (
	"fmt"
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
