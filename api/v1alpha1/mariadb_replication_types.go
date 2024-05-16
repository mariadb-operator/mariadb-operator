package v1alpha1

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// WaitPoint defines whether the transaction should wait for ACK before committing to the storage engine.
// More info: https://mariadb.com/kb/en/semisynchronous-replication/#rpl_semi_sync_master_wait_point.
type WaitPoint string

const (
	// WaitPointAfterSync indicates that the primary waits for the replica ACK before committing the transaction to the storage engine.
	// This is the default WaitPoint. It trades off performance for consistency.
	WaitPointAfterSync WaitPoint = "AfterSync"
	// WaitPointAfterCommit indicates that the primary commits the transaction to the storage engine and waits for the replica ACK afterwards.
	// It trades off consistency for performance.
	WaitPointAfterCommit WaitPoint = "AfterCommit"
)

// Validate returns an error if the WaitPoint is not valid.
func (w WaitPoint) Validate() error {
	switch w {
	case WaitPointAfterSync, WaitPointAfterCommit:
		return nil
	default:
		return fmt.Errorf("invalid WaitPoint: %v", w)
	}
}

// MariaDBFormat formats the WaitPoint so it can be used in MariaDB config files.
func (w WaitPoint) MariaDBFormat() (string, error) {
	switch w {
	case WaitPointAfterSync:
		return "AFTER_SYNC", nil
	case WaitPointAfterCommit:
		return "AFTER_COMMIT", nil
	default:
		return "", fmt.Errorf("invalid WaitPoint: %v", w)
	}
}

// Gtid indicates which Global Transaction ID should be used when connecting a replica to the master.
// See: https://mariadb.com/kb/en/gtid/#using-current_pos-vs-slave_pos.
type Gtid string

const (
	// GtidCurrentPos indicates the union of gtid_binlog_pos and gtid_slave_pos will be used when replicating from master.
	// This is the default Gtid mode.
	GtidCurrentPos Gtid = "CurrentPos"
	// GtidSlavePos indicates that gtid_slave_pos will be used when replicating from master.
	GtidSlavePos Gtid = "SlavePos"
)

// Validate returns an error if the Gtid is not valid.
func (g Gtid) Validate() error {
	switch g {
	case GtidCurrentPos, GtidSlavePos:
		return nil
	default:
		return fmt.Errorf("invalid Gtid: %v", g)
	}
}

// MariaDBFormat formats the Gtid so it can be used in MariaDB config files.
func (g Gtid) MariaDBFormat() (string, error) {
	switch g {
	case GtidCurrentPos:
		return "current_pos", nil
	case GtidSlavePos:
		return "slave_pos", nil
	default:
		return "", fmt.Errorf("invalid Gtid: %v", g)
	}
}

// PrimaryReplication is the replication configuration for the primary node.
type PrimaryReplication struct {
	// PodIndex is the StatefulSet index of the primary node. The user may change this field to perform a manual switchover.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PodIndex *int `json:"podIndex,omitempty"`
	// AutomaticFailover indicates whether the operator should automatically update PodIndex to perform an automatic primary failover.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	AutomaticFailover *bool `json:"automaticFailover,omitempty"`
}

// FillWithDefaults fills the current PrimaryReplication object with DefaultReplicationSpec.
// This enables having minimal PrimaryReplication objects and provides sensible defaults.
func (r *PrimaryReplication) FillWithDefaults() {
	if r.PodIndex == nil {
		index := *DefaultReplicationSpec.Primary.PodIndex
		r.PodIndex = &index
	}
	if r.AutomaticFailover == nil {
		failover := *DefaultReplicationSpec.Primary.AutomaticFailover
		r.AutomaticFailover = &failover
	}
}

// ReplicaReplication is the replication configuration for the replica nodes.
type ReplicaReplication struct {
	// WaitPoint defines whether the transaction should wait for ACK before committing to the storage engine.
	// More info: https://mariadb.com/kb/en/semisynchronous-replication/#rpl_semi_sync_master_wait_point.
	// +optional
	// +kubebuilder:validation:Enum=AfterSync;AfterCommit
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	WaitPoint *WaitPoint `json:"waitPoint,omitempty"`
	// Gtid indicates which Global Transaction ID should be used when connecting a replica to the master.
	// See: https://mariadb.com/kb/en/gtid/#using-current_pos-vs-slave_pos.
	// +optional
	// +kubebuilder:validation:Enum=CurrentPos;SlavePos
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Gtid *Gtid `json:"gtid,omitempty"`
	// ReplPasswordSecretKeyRef provides a reference to the Secret to use as password for the replication user.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReplPasswordSecretKeyRef *GeneratedSecretKeyRef `json:"replPasswordSecretKeyRef,omitempty"`
	// ConnectionTimeout to be used when the replica connects to the primary.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ConnectionTimeout *metav1.Duration `json:"connectionTimeout,omitempty"`
	// ConnectionRetries to be used when the replica connects to the primary.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	ConnectionRetries *int `json:"connectionRetries,omitempty"`
	// SyncTimeout defines the timeout for a replica to be synced with the primary when performing a primary switchover.
	// If the timeout is reached, the replica GTID will be reset and the switchover will continue.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SyncTimeout *metav1.Duration `json:"syncTimeout,omitempty"`
}

// FillWithDefaults fills the current ReplicaReplication object with DefaultReplicationSpec.
// This enables having minimal ReplicaReplication objects and provides sensible defaults.
func (r *ReplicaReplication) FillWithDefaults() {
	if r.WaitPoint == nil {
		waitPoint := *DefaultReplicationSpec.Replica.WaitPoint
		r.WaitPoint = &waitPoint
	}
	if r.Gtid == nil {
		gtid := *DefaultReplicationSpec.Replica.Gtid
		r.Gtid = &gtid
	}
	if r.ConnectionTimeout == nil {
		timeout := *DefaultReplicationSpec.Replica.ConnectionTimeout
		r.ConnectionTimeout = &timeout
	}
	if r.ConnectionRetries == nil {
		retries := *DefaultReplicationSpec.Replica.ConnectionRetries
		r.ConnectionRetries = &retries
	}
	if r.SyncTimeout == nil {
		timeout := *DefaultReplicationSpec.Replica.SyncTimeout
		r.SyncTimeout = &timeout
	}
}

// Validate returns an error if the ReplicaReplication is not valid.
func (r *ReplicaReplication) Validate() error {
	if r.WaitPoint != nil {
		if err := r.WaitPoint.Validate(); err != nil {
			return fmt.Errorf("invalid WaitPoint: %v", err)
		}
	}
	if r.Gtid != nil {
		if err := r.Gtid.Validate(); err != nil {
			return fmt.Errorf("invalid GTID: %v", err)
		}
	}
	return nil
}

// Replication allows you to enable single-master HA via semi-synchronours replication in your MariaDB cluster.
type Replication struct {
	// ReplicationSpec is the Replication desired state specification.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReplicationSpec `json:",inline"`
	// Enabled is a flag to enable Replication.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled,omitempty"`
}

// ReplicationSpec is the Replication desired state specification.
type ReplicationSpec struct {
	// Primary is the replication configuration for the primary node.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Primary *PrimaryReplication `json:"primary,omitempty"`
	// ReplicaReplication is the replication configuration for the replica nodes.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Replica *ReplicaReplication `json:"replica,omitempty"`
	// SyncBinlog indicates whether the binary log should be synchronized to the disk after every event.
	// It trades off performance for consistency.
	// See: https://mariadb.com/kb/en/replication-and-binary-log-system-variables/#sync_binlog.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	SyncBinlog *bool `json:"syncBinlog,omitempty"`
	// ProbesEnabled indicates to use replication specific liveness and readiness probes.
	// This probes check that the primary can receive queries and that the replica has the replication thread running.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ProbesEnabled *bool `json:"probesEnabled,omitempty"`
}

// FillWithDefaults fills the current ReplicationSpec object with DefaultReplicationSpec.
// This enables having minimal ReplicationSpec objects and provides sensible defaults.
func (r *ReplicationSpec) FillWithDefaults() {
	if r.Primary == nil {
		primary := *DefaultReplicationSpec.Primary
		r.Primary = &primary
	} else {
		r.Primary.FillWithDefaults()
	}
	if r.Replica == nil {
		replica := *DefaultReplicationSpec.Replica
		r.Replica = &replica
	} else {
		r.Replica.FillWithDefaults()
	}
	if r.SyncBinlog == nil {
		syncBinlog := *DefaultReplicationSpec.SyncBinlog
		r.SyncBinlog = &syncBinlog
	}
	if r.ProbesEnabled == nil {
		probesEnabled := *DefaultReplicationSpec.ProbesEnabled
		r.ProbesEnabled = &probesEnabled
	}
}

var (
	tenSeconds = metav1.Duration{Duration: 10 * time.Second}

	// DefaultReplicationSpec provides sensible defaults for the ReplicationSpec.
	DefaultReplicationSpec = ReplicationSpec{
		Primary: &PrimaryReplication{
			PodIndex:          ptr.To(0),
			AutomaticFailover: ptr.To(true),
		},
		Replica: &ReplicaReplication{
			WaitPoint:         ptr.To(WaitPointAfterSync),
			Gtid:              ptr.To(GtidCurrentPos),
			ConnectionTimeout: ptr.To(tenSeconds),
			ConnectionRetries: ptr.To(10),
			SyncTimeout:       ptr.To(tenSeconds),
		},
		SyncBinlog:    ptr.To(true),
		ProbesEnabled: ptr.To(false),
	}
)

// IsReplicationConfigured indicates whether replication has been configured.
func (m *MariaDB) IsReplicationConfigured() bool {
	return m.Status.ReplicationStatus.IsReplicationConfigured()
}

// IsSwitchingPrimary indicates whether the primary is being switched.
func (m *MariaDB) IsSwitchingPrimary() bool {
	return meta.IsStatusConditionFalse(m.Status.Conditions, ConditionTypePrimarySwitched)
}

type ReplicationState string

const (
	ReplicationStateMaster        ReplicationState = "Master"
	ReplicationStateSlave         ReplicationState = "Slave"
	ReplicationStateNotConfigured ReplicationState = "NotConfigured"
)

type ReplicationStatus map[string]ReplicationState

func (r ReplicationStatus) IsReplicationConfigured() bool {
	for _, state := range r {
		if state == ReplicationStateNotConfigured {
			return false
		}
	}
	return true
}
