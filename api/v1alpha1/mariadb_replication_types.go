package v1alpha1

import (
	"errors"
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

// Gtid indicates which Global Transaction ID (GTID) position mode should be used when connecting a replica to the master.
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

// PrimaryReplication is the replication configuration and operation parameters for the primary.
type PrimaryReplication struct {
	// PodIndex is the StatefulSet index of the primary node. The user may change this field to perform a manual switchover.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PodIndex *int `json:"podIndex,omitempty"`
	// AutoFailover indicates whether the operator should automatically update PodIndex to perform an automatic primary failover.
	// It is enabled by default.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	AutoFailover *bool `json:"autoFailover,omitempty"`
	// AutoFailoverDelay indicates the duration before performing an automatic primary failover.
	// By default, no extra delay is added.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	AutoFailoverDelay *metav1.Duration `json:"autoFailoverDelay,omitempty"`
}

// SetDefaults fills the current PrimaryReplication object with DefaultReplicationSpec.
// This enables having minimal PrimaryReplication objects and provides sensible defaults.
func (r *PrimaryReplication) SetDefaults() {
	if r.PodIndex == nil {
		r.PodIndex = ptr.To(0)
	}
	if r.AutoFailover == nil {
		r.AutoFailover = ptr.To(true)
	}
	if r.AutoFailoverDelay == nil {
		r.AutoFailoverDelay = ptr.To(metav1.Duration{})
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

// ReplicaRecovery defines how the replicas should be recovered after they enter an error state.
type ReplicaRecovery struct {
	// Enabled is a flag to enable replica recovery.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Enabled bool `json:"enabled"`
	// ErrorDurationThreshold defines the time duration after which, if a replica continues to report errors,
	// the operator will initiate the recovery process for that replica.
	// This threshold applies only to error codes not identified as recoverable by the operator.
	// Errors identified as recoverable will trigger the recovery process immediately.
	// It defaults to 5 minutes.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ErrorDurationThreshold *metav1.Duration `json:"errorDurationThreshold,omitempty"`
}

// ReplicaReplication is the replication configuration and operation parameters for the replicas.
type ReplicaReplication struct {
	// ReplPasswordSecretKeyRef provides a reference to the Secret to use as password for the replication user.
	// By default, a random password will be generated.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReplPasswordSecretKeyRef *GeneratedSecretKeyRef `json:"replPasswordSecretKeyRef,omitempty"`
	// Gtid indicates which Global Transaction ID (GTID) position mode should be used when connecting a replica to the master.
	// By default, CurrentPos is used.
	// See: https://mariadb.com/docs/server/reference/sql-statements/administrative-sql-statements/replication-statements/change-master-to#master_use_gtid.
	// +optional
	// +kubebuilder:validation:Enum=CurrentPos;SlavePos
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Gtid *Gtid `json:"gtid,omitempty"`
	// ConnectionRetrySeconds is the number of seconds that the replica will wait between connection retries.
	// See: https://mariadb.com/docs/server/reference/sql-statements/administrative-sql-statements/replication-statements/change-master-to#master_connect_retry.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	ConnectionRetrySeconds *int `json:"connectionRetrySeconds,omitempty"`
	// MaxLagSeconds is the maximum number of seconds that replicas are allowed to lag behind the primary.
	// If a replica exceeds this threshold, it is marked as not ready and read queries will no longer be forwarded to it.
	// If not provided, it defaults to 0, which means that replicas are not allowed to lag behind the primary (recommended).
	// Lagged replicas will not be taken into account as candidates for the new primary during failover,
	// and they will block other operations, such as switchover and upgrade.
	// This field is not taken into account by MaxScale, you can define the maximum lag as router parameters.
	// See: https://mariadb.com/docs/maxscale/reference/maxscale-routers/maxscale-readwritesplit#max_replication_lag.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	MaxLagSeconds *int `json:"maxLagSeconds,omitempty"`
	// SyncTimeout defines the timeout for the synchronization phase during switchover and failover operations.
	// During switchover, all replicas must be synced with the current primary before promoting the new primary.
	// During failover, the new primary must be synced before being promoted as primary. This implies processing all the events in the relay log.
	// When the timeout is reached, the operator restarts the operation from the beginning.
	// It defaults to 10s.
	// See: https://mariadb.com/docs/server/reference/sql-functions/secondary-functions/miscellaneous-functions/master_gtid_wait
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SyncTimeout *metav1.Duration `json:"syncTimeout,omitempty"`
	// ReplicaBootstrapFrom defines the data sources used to bootstrap new replicas.
	// This will be used as part of the scaling out and recovery operations, when new replicas are created.
	// If not provided, scale out and recovery operations will return an error.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReplicaBootstrapFrom *ReplicaBootstrapFrom `json:"bootstrapFrom,omitempty"`
	// ReplicaRecovery defines how the replicas should be recovered after they enter an error state.
	// This process deletes data from faulty replicas and recreates them using the source defined in the bootstrapFrom field.
	// It is disabled by default, and it requires the bootstrapFrom field to be set.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReplicaRecovery *ReplicaRecovery `json:"recovery,omitempty"`
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
	recoveryEnabled := ptr.Deref(r.ReplicaRecovery, ReplicaRecovery{}).Enabled
	if recoveryEnabled && r.ReplicaBootstrapFrom == nil {
		return errors.New("'bootstrapFrom' must be set when 'recovery` is enabled")
	}
	return nil
}

// Replication defines replication configuration for a MariaDB cluster.
type Replication struct {
	// ReplicationSpec is the Replication desired state specification.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReplicationSpec `json:",inline"`
	// Enabled is a flag to enable replication.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled,omitempty"`
}

// ReplicationSpec is the replication desired state.
type ReplicationSpec struct {
	// Primary is the replication configuration for the primary node.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Primary PrimaryReplication `json:"primary,omitempty"`
	// ReplicaReplication is the replication configuration for the replica nodes.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Replica ReplicaReplication `json:"replica,omitempty"`
	// GtidStrictMode determines whether the GTID strict mode is enabled.
	// See: https://mariadb.com/docs/server/ha-and-performance/standard-replication/gtid#gtid_strict_mode.
	// It is enabled by default.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	GtidStrictMode *bool `json:"gtidStrictMode,omitempty"`
	// SemiSyncEnabled determines whether semi-synchronous replication is enabled.
	// Semi-synchronous replication requires that at least one replica should have sent an ACK to the primary node
	// before committing the transaction back to the client.
	// See: https://mariadb.com/docs/server/ha-and-performance/standard-replication/semisynchronous-replication
	// It is enabled by default
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SemiSyncEnabled *bool `json:"semiSyncEnabled,omitempty"`
	// SemiSyncAckTimeout for the replica to acknowledge transactions to the primary.
	// It requires semi-synchronous replication to be enabled.
	// See: https://mariadb.com/docs/server/ha-and-performance/standard-replication/semisynchronous-replication#rpl_semi_sync_master_timeout
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SemiSyncAckTimeout *metav1.Duration `json:"semiSyncAckTimeout,omitempty"`
	// SemiSyncWaitPoint determines whether the transaction should wait for an ACK after having synced the binlog (AfterSync)
	// or after having committed to the storage engine (AfterCommit, the default).
	// It requires semi-synchronous replication to be enabled.
	// See: https://mariadb.com/kb/en/semisynchronous-replication/#rpl_semi_sync_master_wait_point.
	// +optional
	// +kubebuilder:validation:Enum=AfterSync;AfterCommit
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SemiSyncWaitPoint *WaitPoint `json:"semiSyncWaitPoint,omitempty"`
	// SyncBinlog indicates after how many events the binary log is synchronized to the disk.
	// See: https://mariadb.com/docs/server/ha-and-performance/standard-replication/replication-and-binary-log-system-variables#sync_binlog
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	SyncBinlog *int `json:"syncBinlog,omitempty"`
	// InitContainer is an init container that runs in the MariaDB Pod and co-operates with mariadb-operator.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	InitContainer InitContainer `json:"initContainer,omitempty"`
	// Agent is a sidecar agent that runs in the MariaDB Pod and co-operates with mariadb-operator.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced"}
	Agent Agent `json:"agent,omitempty"`
	// StandaloneProbes indicates whether to use the default non-HA startup and liveness probes.
	// It is disabled by default
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	StandaloneProbes *bool `json:"standaloneProbes,omitempty"`
}

// IsGtidStrictModeEnabled determines whether GTID strict mode is enabled.
func (r *Replication) IsGtidStrictModeEnabled() bool {
	return ptr.Deref(r.GtidStrictMode, true)
}

// IsSemiSyncEnabled determines whether semi-synchronous replication is enabled.
func (r *Replication) IsSemiSyncEnabled() bool {
	return ptr.Deref(r.SemiSyncEnabled, true)
}

// Validate determines whether replication config is valid.
func (r *Replication) Validate() error {
	if r.IsSemiSyncEnabled() {
		if r.SemiSyncWaitPoint != nil {
			if err := r.SemiSyncWaitPoint.Validate(); err != nil {
				return fmt.Errorf("invalid WaitPoint: %v", err)
			}
		}
	}
	return nil
}

// SetDefaults sets reasonable defaults for replication.
func (r *Replication) SetDefaults(mdb *MariaDB, env *environment.OperatorEnv) error {
	r.Primary.SetDefaults()
	r.Replica.SetDefaults(mdb)

	if r.GtidStrictMode == nil {
		r.GtidStrictMode = ptr.To(true)
	}
	if r.SemiSyncEnabled == nil {
		r.SemiSyncEnabled = ptr.To(true)
	}
	if r.StandaloneProbes == nil {
		r.StandaloneProbes = ptr.To(false)
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

// HasConfiguredReplication indicates whether the MariaDB object has a ConditionTypeReplicationConfigured status condition.
// This means that replication has been successfully configured for the first time.
func (m *MariaDB) HasConfiguredReplication() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeReplicationConfigured)
}

// HasConfiguredReplica indicates whether the cluster has a configured replica.
func (m *MariaDB) HasConfiguredReplica() bool {
	if m.Status.Replication == nil {
		return false
	}
	for _, role := range m.Status.Replication.Roles {
		if role == ReplicationRoleReplica {
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
	for pod, role := range m.Status.Replication.Roles {
		if pod == podName && role == ReplicationRoleReplica {
			return true
		}
	}
	return false
}

// IsReplicaRecoveryEnabled indicates if the replica recovery is enabled
func (m *MariaDB) IsReplicaRecoveryEnabled() bool {
	if !m.IsReplicationEnabled() {
		return false
	}
	replication := ptr.Deref(m.Spec.Replication, Replication{})
	recovery := ptr.Deref(replication.Replica.ReplicaRecovery, ReplicaRecovery{})
	return recovery.Enabled
}

// IsRecoveringReplicas indicates that a replica is being recovered.
func (m *MariaDB) IsRecoveringReplicas() bool {
	return meta.IsStatusConditionFalse(m.Status.Conditions, ConditionTypeReplicaRecovered)
}

// ReplicaRecoveryError indicates that the MariaDB instance has a replica recovery error.
func (m *MariaDB) ReplicaRecoveryError() error {
	c := meta.FindStatusCondition(m.Status.Conditions, ConditionTypeReplicaRecovered)
	if c == nil {
		return nil
	}
	if c.Status == metav1.ConditionFalse && c.Reason == ConditionReasonReplicaRecoverError {
		return errors.New(c.Message)
	}
	return nil
}

// SetReplicaToRecover sets the replica to be recovered
func (m *MariaDB) SetReplicaToRecover(replica *string) {
	if m.Status.Replication == nil {
		m.Status.Replication = &ReplicationStatus{}
	}
	m.Status.Replication.ReplicaToRecover = replica
}

// IsReplicaBeingRecovered indicates whether a replica is being recovered
func (m *MariaDB) IsReplicaBeingRecovered(replica string) bool {
	if !m.IsRecoveringReplicas() {
		return false
	}
	replication := ptr.Deref(m.Status.Replication, ReplicationStatus{})
	return replication.ReplicaToRecover != nil && *replication.ReplicaToRecover == replica
}

// GetAutomaticFailoverDelay returns the duration of the automatic failover delay.
func (m *MariaDB) GetAutomaticFailoverDelay() time.Duration {
	primary := ptr.Deref(m.Spec.Replication, Replication{}).Primary
	autoFailoverDelay := ptr.Deref(primary.AutoFailoverDelay, metav1.Duration{})

	return autoFailoverDelay.Duration
}

// IsSwitchingPrimary indicates whether a primary swichover operation is in progress.
func (m *MariaDB) IsSwitchingPrimary() bool {
	return meta.IsStatusConditionFalse(m.Status.Conditions, ConditionTypePrimarySwitched)
}

// IsReplicationSwitchoverRequired indicates that a primary switchover operation is required.
func (m *MariaDB) IsReplicationSwitchoverRequired() bool {
	if m.IsMaxScaleEnabled() || !m.IsReplicationEnabled() {
		return false
	}
	if m.Status.CurrentPrimaryPodIndex == nil || m.Spec.Replication == nil || m.Spec.Replication.Primary.PodIndex == nil {
		return false
	}
	return *m.Status.CurrentPrimaryPodIndex != *m.Spec.Replication.Primary.PodIndex
}

// ReplicationRole represents the observed replication roles.
type ReplicationRole string

const (
	ReplicationRolePrimary ReplicationRole = "Primary"
	ReplicationRoleReplica ReplicationRole = "Replica"
	ReplicationRoleUnknown ReplicationRole = "Unknown"
)

// ReplicaStatusVars is the observed replica status variables.
type ReplicaStatusVars struct {
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
	// SlaveIORunning indicates whether the slave IO thread is running.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	SlaveIORunning *bool `json:"slaveIORunning,omitempty"`
	// SlaveSQLRunning indicates whether the slave SQL thread is running.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	SlaveSQLRunning *bool `json:"slaveSQLRunning,omitempty"`
	// SecondsBehindMaster measures the replication lag with the primary.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	SecondsBehindMaster *int `json:"secondsBehindMaster,omitempty"`
	// GtidIOPos is the last GTID position received by the IO thread and written to the relay log.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	GtidIOPos *string `json:"gtidIOPos,omitempty"`
	// GtidCurrentPos is the last GTID position executed by the SQL thread.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	GtidCurrentPos *string `json:"gtidCurrentPos,omitempty"`
}

// EqualErrors determines equality of error codes.
func (r *ReplicaStatusVars) EqualErrors(o *ReplicaStatusVars) bool {
	if r == nil && o == nil {
		return true
	}
	if r == nil || o == nil {
		return false
	}
	return ptr.Equal(r.LastIOErrno, o.LastIOErrno) &&
		ptr.Equal(r.LastSQLErrno, o.LastSQLErrno)
}

// ReplicaStatus is the observed replica status.
type ReplicaStatus struct {
	// ReplicaStatusVars is the observed replica status variables.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReplicaStatusVars `json:",inline"`
	// LastErrorTransitionTime is the last time the replica transitioned to an error state.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	LastErrorTransitionTime metav1.Time `json:"lastErrorTransitionTime,omitempty"`
}

// ReplicationStatus is the replication current state.
type ReplicationStatus struct {
	// Roles is the observed replication roles for each Pod.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Roles map[string]ReplicationRole `json:"roles,omitempty"`
	// Replicas is the observed replication status for each replica.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Replicas map[string]ReplicaStatus `json:"replicas,omitempty"`
	// ReplicaToRecover is the replica that is being recovered by the operator.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	ReplicaToRecover *string `json:"replicaToRecover,omitempty"`
}

// UseStandaloneProbes indicates whether to use the default non-HA startup and liveness probes.
func (m *MariaDB) UseStandaloneProbes() bool {
	replication := ptr.Deref(m.Spec.Replication, Replication{})
	standaloneProbes := ptr.Deref(replication.StandaloneProbes, false)

	return standaloneProbes
}
