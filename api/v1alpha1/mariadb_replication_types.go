package v1alpha1

import (
	"fmt"
	"reflect"
	"time"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/docker"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// WaitPoint defines whether the transaction should wait for ACK before committing to the storage engine.
// More info: https://mariadb.com/kb/en/semisynchronous-replication/#rpl_semi_sync_master_wait_point.
type WaitPoint string

const (
	// WaitPointAfterSync indicates that the primary waits for the replica ACK before committing the transaction to the storage engine.
	// It trades off performance for consistency.
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
	// AutomaticFailoverDelay indicates the duration before performing an automatic primary failover. By default, no extra delay is added.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	AutomaticFailoverDelay *metav1.Duration `json:"automaticFailoverDelay,omitempty"`
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
	if r.AutomaticFailoverDelay == nil {
		failoverDelay := *DefaultReplicationSpec.Primary.AutomaticFailoverDelay
		r.AutomaticFailoverDelay = &failoverDelay
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
	// During a switchover, all replicas must be synced with the primary before promoting the new primary.
	// During a failover, the primary will be down, therefore this sync step will be skipped.
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
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReplicationSpec `json:",inline"`
	// Enabled is a flag to enable Replication.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled,omitempty"`
}

// SetDefaults sets reasonable defaults.
func (r *Replication) SetDefaults(mdb *MariaDB, env *environment.OperatorEnv) error {
	if r.GtidStrictMode == nil {
		r.GtidStrictMode = ptr.To(true)
	}
	if reflect.ValueOf(r.InitContainer).IsZero() {
		r.InitContainer = InitContainer{
			Image: env.MariadbOperatorImage,
		}
	}
	if err := r.Agent.SetDefaults(mdb, env); err != nil {
		return fmt.Errorf("error setting agent defaults: %v", err)
	}

	autoUpdateDataPlane := ptr.Deref(mdb.Spec.UpdateStrategy.AutoUpdateDataPlane, false)
	if autoUpdateDataPlane {
		initBumped, err := docker.SetTagOrDigest(env.MariadbOperatorImage, r.InitContainer.Image)
		if err != nil {
			return fmt.Errorf("error bumping replication init image: %v", err)
		}
		r.InitContainer.Image = initBumped

		agentBumped, err := docker.SetTagOrDigest(env.MariadbOperatorImage, r.Agent.Image)
		if err != nil {
			return fmt.Errorf("error bumping replication agent image: %v", err)
		}
		r.Agent.Image = agentBumped
	}

	return nil
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
	// GtidStrictMode determines whether the GTID strict mode is enabled. See: https://mariadb.com/docs/server/ha-and-performance/standard-replication/gtid#gtid_strict_mode.
	// It is enabled by default.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	GtidStrictMode *bool `json:"gtidStrictMode,omitempty"`
	// SyncBinlog indicates after how many events the binary log is synchronized to the disk.
	// The default is 1, flushing the binary log to disk after every write, which trades off performance for consistency. See: https://mariadb.com/docs/server/ha-and-performance/standard-replication/replication-and-binary-log-system-variables#sync_binlog
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	SyncBinlog *int `json:"syncBinlog,omitempty"`
	// InitContainer is an init container that runs in the MariaDB Pod and co-operates with mariadb-operator.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	InitContainer InitContainer `json:"initContainer,omitempty"`
	// Agent is a sidecar agent that co-operates with mariadb-operator.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Agent Agent `json:"agent,omitempty"`
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
}

var (
	tenSeconds = metav1.Duration{Duration: 10 * time.Second}

	// DefaultReplicationSpec provides sensible defaults for the ReplicationSpec.
	DefaultReplicationSpec = ReplicationSpec{
		Primary: &PrimaryReplication{
			PodIndex:               ptr.To(0),
			AutomaticFailover:      ptr.To(true),
			AutomaticFailoverDelay: ptr.To(metav1.Duration{}),
		},
		Replica: &ReplicaReplication{
			WaitPoint:         ptr.To(WaitPointAfterCommit),
			Gtid:              ptr.To(GtidCurrentPos),
			ConnectionTimeout: ptr.To(tenSeconds),
			ConnectionRetries: ptr.To(10),
			SyncTimeout:       ptr.To(tenSeconds),
		},
		SyncBinlog: ptr.To(1),
	}
)

// GetAutomaticFailoverDelay returns the duration of the automatic failover delay.
func (m *MariaDB) GetAutomaticFailoverDelay() time.Duration {
	primary := ptr.Deref(m.Replication().Primary, PrimaryReplication{})
	automaticFailoverDelay := ptr.Deref(primary.AutomaticFailoverDelay, *DefaultReplicationSpec.Primary.AutomaticFailoverDelay)

	return automaticFailoverDelay.Duration
}

// HasConfiguredReplica indicates whether the cluster has a configured replica.
func (m *MariaDB) HasConfiguredReplica() bool {
	return m.Status.Replication.HasConfiguredReplica()
}

// IsConfiguredReplica indicates whether the given pod is a configured replica.
func (m *MariaDB) IsConfiguredReplica(podName string) bool {
	return m.Status.Replication.IsConfiguredReplica(podName)
}

// IsSwitchingPrimary indicates whether a primary swichover operation is in progress.
func (m *MariaDB) IsSwitchingPrimary() bool {
	return meta.IsStatusConditionFalse(m.Status.Conditions, ConditionTypePrimarySwitched)
}

// IsSwitchoverRequired indicates that a primary switchover operation is required.
func (m *MariaDB) IsSwitchoverRequired() bool {
	if m.Status.CurrentPrimaryPodIndex == nil {
		return false
	}
	currentPodIndex := ptr.Deref(m.Status.CurrentPrimaryPodIndex, 0)
	desiredPodIndex := ptr.Deref(m.Replication().Primary.PodIndex, 0)
	return currentPodIndex != desiredPodIndex
}

// ReplicationState represents the observed replication states.
type ReplicationState string

const (
	ReplicationStatePrimary       ReplicationState = "Primary"
	ReplicationStateReplica       ReplicationState = "Replica"
	ReplicationStateNotConfigured ReplicationState = "NotConfigured"
)

// ReplicationStatus is the replication current status per each Pod.
type ReplicationStatus map[string]ReplicationState

// HasConfiguredReplica determines whether at least one replica has been configured.
func (r ReplicationStatus) HasConfiguredReplica() bool {
	for _, state := range r {
		if state == ReplicationStateReplica {
			return true
		}
	}
	return false
}

// HasConfiguredReplica determines whether if a specific replica has been configured.
func (r ReplicationStatus) IsConfiguredReplica(podName string) bool {
	for pod, state := range r {
		if pod == podName && state == ReplicationStateReplica {
			return true
		}
	}
	return false
}
