package v1alpha1

import (
	"fmt"
)

type MultiCluster struct {
	// MultiClusterSpec is the desired MultiCluster specification.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MultiClusterSpec `json:",inline"`
	// Enabled is a flag to enable MultiCluster replication.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled,omitempty"`
}

type MultiClusterSpec struct {
	// Primary is the name of the primary MariaDB cluster.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Primary string `json:"primary,omitempty"`
	// Replicas is a string array of all the replica members.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Replicas []string `json:"replicas,omitempty"`
	// Members is an array of all the members of the multi cluster setup, including the current cluster.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Members []MultiClusterMember `json:"members,omitempty"`
}

type MultiClusterMember struct {
	// Name us the name by which the Cluster will be known.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Name string `json:"name"`
	// ExternalMariaDBRef holds a reference to an ExternalMariaDB with connection details for the remote/local cluster.
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
