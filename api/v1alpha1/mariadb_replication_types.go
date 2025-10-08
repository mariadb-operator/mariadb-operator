package v1alpha1

import (
	"fmt"
	"reflect"
	"time"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/datastructures"
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

// SetDefaults fills the current PrimaryReplication object with DefaultReplicationSpec.
// This enables having minimal PrimaryReplication objects and provides sensible defaults.
func (r *PrimaryReplication) SetDefaults() {
	if r.PodIndex == nil {
		r.PodIndex = ptr.To(0)
	}
	if r.AutomaticFailover == nil {
		r.AutomaticFailover = ptr.To(true)
	}
	if r.AutomaticFailoverDelay == nil {
		r.AutomaticFailoverDelay = ptr.To(metav1.Duration{})
	}
}

// ReplicaBootstrapFrom defines the sources for bootstrapping new relicas.
type ReplicaBootstrapFrom struct {
	// PhysicalBackupTemplateRef is a reference to a PhysicalBackup object that will be used as template to create a new PhysicalBackup object
	// used synchronize the data from an up to date replica to the new replica to be bootstrapped.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PhysicalBackupTemplateRef LocalObjectReference `json:"physicalBackupTemplateRef"`
	// RestoreJob defines additional properties for the Job used to perform the restoration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	RestoreJob *Job `json:"restoreJob,omitempty"`
}

// ReplicaReplication is the replication configuration for the replica nodes.
type ReplicaReplication struct {
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
	// See: https://mariadb.com/docs/server/reference/sql-statements/administrative-sql-statements/replication-statements/change-master-to#master_connect_retry
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	ConnectionRetries *int `json:"connectionRetries,omitempty"`
	// SyncTimeout defines the timeout for a replica to be synced with the primary when performing a primary switchover.
	// During a switchover, all replicas must be synced with the primary before promoting the new primary.
	// During a failover, the primary will be down, therefore this sync step will be skipped.
	// See: https://mariadb.com/docs/server/reference/sql-functions/secondary-functions/miscellaneous-functions/master_gtid_wait
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SyncTimeout *metav1.Duration `json:"syncTimeout,omitempty"`
	// MaxLagSeconds is the maximum number of seconds that replicas are allowed to lag behind the primary.
	// If a replica exceeds this threshold, it is marked as not ready and queries will no longer be forwarded to it.
	// Replicas in non ready state will block operations such as primary switchover and upgrades.
	// If not provided, it defaults to 0, which means replicas are not allowed to lag behind the primary.
	// This field is not taken into account by MaxScale, you can define the maximum lag as router parameters. See: https://mariadb.com/docs/maxscale/reference/maxscale-routers/maxscale-readwritesplit#max_replication_lag.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	MaxLagSeconds *int `json:"maxLagSeconds,omitempty"`
	// ReplicaBootstrapFrom defines the datasources for bootstrapping new relicas.
	// This will be used as part of the scaling out operations, when increasing the number of replicas.
	// If not provided, scale out operations will return an error.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReplicaBootstrapFrom *ReplicaBootstrapFrom `json:"bootstrapFrom,omitempty"`
}

// SetDefaults fills the current ReplicaReplication object with DefaultReplicationSpec.
// This enables having minimal ReplicaReplication objects and provides sensible defaults.
func (r *ReplicaReplication) SetDefaults(mdb *MariaDB) {
	if r.ReplPasswordSecretKeyRef == nil {
		r.ReplPasswordSecretKeyRef = ptr.To(mdb.ReplPasswordSecretKeyRef())
	}
	if r.Gtid == nil {
		r.Gtid = ptr.To(GtidCurrentPos)
	}
	if r.ConnectionTimeout == nil {
		r.ConnectionTimeout = ptr.To(metav1.Duration{Duration: 10 * time.Second})
	}
	if r.SyncTimeout == nil {
		r.SyncTimeout = ptr.To(metav1.Duration{Duration: 10 * time.Second})
	}
}

// Validate returns an error if the ReplicaReplication is not valid.
func (r *ReplicaReplication) Validate() error {
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

// ReplicationSpec is the Replication desired state specification.
type ReplicationSpec struct {
	// Primary is the replication configuration for the primary node.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Primary PrimaryReplication `json:"primary,omitempty"`
	// ReplicaReplication is the replication configuration for the replica nodes.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Replica ReplicaReplication `json:"replica,omitempty"`

	// WaitPoint defines whether the transaction should wait for ACK before committing to the storage engine.
	// More info: https://mariadb.com/kb/en/semisynchronous-replication/#rpl_semi_sync_master_wait_point.
	// +optional
	// +kubebuilder:validation:Enum=AfterSync;AfterCommit
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	WaitPoint *WaitPoint `json:"waitPoint,omitempty"`

	// GtidStrictMode determines whether the GTID strict mode is enabled. See: https://mariadb.com/docs/server/ha-and-performance/standard-replication/gtid#gtid_strict_mode.
	// It is enabled by default.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	GtidStrictMode *bool `json:"gtidStrictMode,omitempty"`
	// SyncBinlog indicates after how many events the binary log is synchronized to the disk.
	// See: https://mariadb.com/docs/server/ha-and-performance/standard-replication/replication-and-binary-log-system-variables#sync_binlog
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

func (r *Replication) Validate() error {
	if r.WaitPoint != nil {
		if err := r.WaitPoint.Validate(); err != nil {
			return fmt.Errorf("invalid WaitPoint: %v", err)
		}
	}

	return nil
}

// SetDefaults fills the current Replication object with DefaultReplicationSpec.
// This enables having minimal Replication objects and provides sensible defaults.
func (r *Replication) SetDefaults(mdb *MariaDB, env *environment.OperatorEnv) error {
	r.Primary.SetDefaults()
	r.Replica.SetDefaults(mdb)

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

// GetAutomaticFailoverDelay returns the duration of the automatic failover delay.
func (m *MariaDB) GetAutomaticFailoverDelay() time.Duration {
	primary := ptr.Deref(m.Spec.Replication, Replication{}).Primary
	automaticFailoverDelay := ptr.Deref(primary.AutomaticFailoverDelay, metav1.Duration{})

	return automaticFailoverDelay.Duration
}

// HasConfiguredReplica indicates whether the cluster has a configured replica.
func (m *MariaDB) HasConfiguredReplica() bool {
	if m.Status.Replication == nil {
		return false
	}
	for _, state := range m.Status.Replication.State {
		if state == ReplicationStateReplica {
			return true
		}
	}
	return false
}

// IsConfiguredReplica determines whether a specific replica has been configured.
func (m *MariaDB) IsConfiguredReplica(podName string) bool {
	if m.Status.Replication == nil {
		return false
	}
	for pod, state := range m.Status.Replication.State {
		if pod == podName && state == ReplicationStateReplica {
			return true
		}
	}
	return false
}

// SetReplicasToConfigure sets replicas to be configured.
func (m *MariaDB) SetReplicasToConfigure(replicas ...ReplicaToConfigure) {
	if m.Status.Replication == nil {
		m.Status.Replication = &ReplicationStatus{}
	}
	m.Status.Replication.ReplicasToConfigure = replicas
}

// ReplicaNeedsConfiguration indicates whether a replica needs to be configured.
func (m *MariaDB) ReplicaNeedsConfiguration(replicaName string) bool {
	return m.GetReplicaToConfigure(replicaName) != nil
}

// ReplicaConfigurationSource returns the replica to be configured
func (m *MariaDB) GetReplicaToConfigure(replicaName string) *ReplicaToConfigure {
	if m.Status.Replication == nil {
		return nil
	}
	return datastructures.Find(m.Status.Replication.ReplicasToConfigure, func(r ReplicaToConfigure) bool {
		return r.Name == replicaName
	})
}

// MarkReplicaAsConfigured marks a replica as configured.
func (m *MariaDB) MarkReplicaAsConfigured(replicaName string) {
	if m.Status.Replication == nil {
		return
	}
	m.Status.Replication.ReplicasToConfigure =
		datastructures.Remove(m.Status.Replication.ReplicasToConfigure, func(r ReplicaToConfigure) bool {
			return r.Name == replicaName
		})
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
	desiredPodIndex := ptr.Deref(ptr.Deref(m.Spec.Replication, Replication{}).Primary.PodIndex, 0)
	return currentPodIndex != desiredPodIndex
}

// ReplicationState represents the observed replication states.
type ReplicationState string

const (
	ReplicationStatePrimary       ReplicationState = "Primary"
	ReplicationStateReplica       ReplicationState = "Replica"
	ReplicationStateConfiguring   ReplicationState = "Configuring"
	ReplicationStateNotConfigured ReplicationState = "NotConfigured"
)

// ReplicaToConfigure is a replica that the operator will configure replication on.
type ReplicaToConfigure struct {
	// Name of the replica.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Name string `json:"name"`
	// VolumeSnapshotRef is a reference to the VolumeSnapshot to be used to configure the replica.
	// If not provided, configuration from mariadb-backup is assumed.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	VolumeSnapshotRef *LocalObjectReference `json:"volumeSnapshotRef,omitempty"`
}

// ReplicaErrors is the current error state of the threads available in a replica.
type ReplicaErrors struct {
	// LastIOErrno is the error code returned by the IO thread.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastIOErrno *int `json:"lastIOErrno,omitempty"`
	// LastIOErrno is the error message returned by the IO thread.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastIOError *string `json:"lastIOError,omitempty"`
	// LastSQLErrno is the error code returned by the SQL thread.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastSQLErrno *int `json:"lastSQLErrno,omitempty"`
	// LastSQLError is the error message returned by the SQL thread.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastSQLError *string `json:"lastSQLError,omitempty"`
}

// Equal determines equality based on error codes.
func (r *ReplicaErrors) Equal(o *ReplicaErrors) bool {
	if r == nil && o == nil {
		return true
	}
	if r == nil || o == nil {
		return false
	}
	return ptr.Equal(r.LastIOErrno, o.LastIOErrno) &&
		ptr.Equal(r.LastSQLErrno, o.LastSQLErrno)
}

// ReplicaErrorStatus represents the current error state of a replica.
type ReplicaErrorStatus struct {
	// ReplicaErrors is the current error state of the threads available in a replica.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReplicaErrors `json:",inline"`
	// LastTransitionTime is the last time the replica transitioned to an error state.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// ReplicationStatus is the replication current state.
type ReplicationStatus struct {
	// State is the observed replication state for each Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	State map[string]ReplicationState `json:"state,omitempty"`
	// Errors is the observed state of replication errors for each replica.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Errors map[string]ReplicaErrorStatus `json:"errors,omitempty"`
	// ReplicasToConfigure are the replicas that the operator will configure replication on.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	ReplicasToConfigure []ReplicaToConfigure `json:"replicasToConfigure,omitempty"`
}
