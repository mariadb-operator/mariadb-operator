package replication

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	agentclient "github.com/mariadb-operator/mariadb-operator/v26/pkg/agent/client"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	conditions "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Option func(*ReplicationReconciler)

func WithRefResolver(rr *refresolver.RefResolver) Option {
	return func(r *ReplicationReconciler) {
		r.refResolver = rr
	}
}

func WithSecretReconciler(sr *secret.SecretReconciler) Option {
	return func(rr *ReplicationReconciler) {
		rr.secretReconciler = sr
	}
}

func WithServiceReconciler(sr *service.ServiceReconciler) Option {
	return func(rr *ReplicationReconciler) {
		rr.serviceReconciler = sr
	}
}

type ReplicationReconciler struct {
	client.Client
	recorder            events.EventRecorder
	builder             *builder.Builder
	env                 *environment.OperatorEnv
	topologyManager     *TopologyManager
	refResolver         *refresolver.RefResolver
	secretReconciler    *secret.SecretReconciler
	configMapreconciler *configmap.ConfigMapReconciler
	serviceReconciler   *service.ServiceReconciler
}

func NewReplicationReconciler(client client.Client, recorder events.EventRecorder, builder *builder.Builder, env *environment.OperatorEnv,
	topologyManager *TopologyManager, opts ...Option) (*ReplicationReconciler, error) {
	r := &ReplicationReconciler{
		Client:          client,
		recorder:        recorder,
		builder:         builder,
		env:             env,
		topologyManager: topologyManager,
	}
	for _, setOpt := range opts {
		setOpt(r)
	}
	if r.refResolver == nil {
		r.refResolver = refresolver.New(client)
	}
	if r.secretReconciler == nil {
		reconciler, err := secret.NewSecretReconciler(client, builder)
		if err != nil {
			return nil, err
		}
		r.secretReconciler = reconciler
	}
	if r.configMapreconciler == nil {
		r.configMapreconciler = configmap.NewConfigMapReconciler(client, builder)
	}
	if r.serviceReconciler == nil {
		r.serviceReconciler = service.NewServiceReconciler(client)
	}
	return r, nil
}

type ReconcileRequest struct {
	mariadb             *mariadbv1alpha1.MariaDB
	key                 types.NamespacedName
	replClientSet       *ReplicationClientSet
	agentClientSet      *agentclient.ClientSet
	currentPrimaryReady bool
	replicasSynced      bool
}

func (r *ReconcileRequest) Close() error {
	if r.replClientSet != nil {
		r.replClientSet.close()
	}
	return nil
}

func (r *ReplicationReconciler) NewReconcileRequest(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (*ReconcileRequest, error) {
	replClientSet, err := NewReplicationClientSet(mdb, r.refResolver)
	if err != nil {
		return nil, fmt.Errorf("error creating mariadb clientset: %v", err)
	}
	agentClientSet, err := agentclient.NewClientSet(ctx, mdb, r.env, r.refResolver)
	if err != nil {
		return nil, fmt.Errorf("error getting agent clientset: %v", err)
	}
	return &ReconcileRequest{
		mariadb:             mdb,
		key:                 client.ObjectKeyFromObject(mdb),
		replClientSet:       replClientSet,
		agentClientSet:      agentClientSet,
		currentPrimaryReady: false,
		replicasSynced:      false,
	}, nil
}

func (r *ReplicationReconciler) Reconcile(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !mdb.IsReplicationEnabled() {
		return ctrl.Result{}, nil
	}
	logger := log.FromContext(ctx).WithName("replication")
	switchoverLogger := log.FromContext(ctx).WithName("switchover")

	req, err := r.NewReconcileRequest(ctx, mdb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating reconcile request: %v", err)
	}
	defer req.Close()

	if mdb.IsReplicationSwitchoverRequired() {
		return ctrl.Result{}, r.reconcileSwitchover(ctx, req, switchoverLogger)
	}
	if result, err := r.reconcileReplication(ctx, req, logger); !result.IsZero() || err != nil {
		return result, err
	}
	return ctrl.Result{}, r.reconcileSwitchover(ctx, req, switchoverLogger)
}

func (r *ReplicationReconciler) reconcileReplication(ctx context.Context, req *ReconcileRequest, logger logr.Logger) (ctrl.Result, error) {
	if result, err := r.shouldReconcileReplication(ctx, req, logger); !result.IsZero() || err != nil {
		return result, err
	}

	for _, i := range r.replicationPodIndexes(req) {
		if result, err := r.ReconcileReplicationInPod(ctx, req, i, logger); !result.IsZero() || err != nil {
			return result, err
		}
	}
	if !req.mariadb.HasConfiguredReplication() {
		if err := r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
			conditions.SetReplicationConfigured(status)
		}); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *ReplicationReconciler) shouldReconcileReplication(ctx context.Context, req *ReconcileRequest,
	logger logr.Logger) (ctrl.Result, error) {
	replication := req.mariadb.Replication()
	isExternalReplication := replication.IsExternalReplication()

	if req.mariadb.Status.CurrentPrimaryPodIndex == nil && !isExternalReplication {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	if isExternalReplication && !req.mariadb.IsExternalReplInitialized() {
		logger.Info("external replication no initialized, trying again in 5s")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if req.mariadb.IsSwitchingPrimary() {
		return ctrl.Result{}, nil
	}

	if req.mariadb.IsMaxScaleEnabled() {
		mxs, err := r.refResolver.MaxScale(ctx, req.mariadb.Spec.MaxScaleRef, req.mariadb.Namespace)
		if err != nil {
			// MaxScale is not present, so no conflict can occur. Safe to proceed with replication reconciliation.
			if apierrors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, fmt.Errorf("error getting MaxScale: %v", err)
		}
		if mxs.IsSwitchingPrimary() {
			logger.Info("MaxScale is switching primary. Requeuing..")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}
	return ctrl.Result{}, nil
}

func (r *ReplicationReconciler) replicationPodIndexes(req *ReconcileRequest) []int {
	podIndexes := []int{
		*req.mariadb.Status.CurrentPrimaryPodIndex,
	}
	for i := 0; i < int(req.mariadb.Spec.Replicas); i++ {
		if i != *req.mariadb.Status.CurrentPrimaryPodIndex {
			podIndexes = append(podIndexes, i)
		}
	}
	return podIndexes
}

type ReconcilePodOpts struct {
	forceReplicaConfiguration bool
	volumeSnapshotKey         *types.NamespacedName
}

type ReconcilePodOpt func(*ReconcilePodOpts)

func WithForceReplicaConfiguration(reconcile bool) ReconcilePodOpt {
	return func(rpo *ReconcilePodOpts) {
		rpo.forceReplicaConfiguration = reconcile
	}
}

func WithVolumeSnapshotKey(key *types.NamespacedName) ReconcilePodOpt {
	return func(rpo *ReconcilePodOpts) {
		rpo.volumeSnapshotKey = key
	}
}

func (r *ReplicationReconciler) ReconcileReplicationInPod(ctx context.Context, req *ReconcileRequest, podIndex int,
	logger logr.Logger, reconcilePodOpts ...ReconcilePodOpt) (ctrl.Result, error) {
	opts := ReconcilePodOpts{}
	for _, setOpt := range reconcilePodOpts {
		setOpt(&opts)
	}

	primaryPodIndex := *req.mariadb.Status.CurrentPrimaryPodIndex
	replStatus := ptr.Deref(req.mariadb.Status.Replication, mariadbv1alpha1.ReplicationStatus{})
	replRoles := replStatus.Roles
	pod := statefulset.PodName(req.mariadb.ObjectMeta, podIndex)
	topology := r.topologyManager.TopologyForMariaDB(req.mariadb, logger.WithValues("pod", pod))
	replication := req.mariadb.Replication()
	isExternalReplication := replication.IsExternalReplication()

	if primaryPodIndex == podIndex && !isExternalReplication {
		if shouldSkipPrimaryReconciliation(req.mariadb, replRoles, pod, logger) {
			return ctrl.Result{}, nil
		}
		client, err := req.replClientSet.currentPrimaryClient(ctx)
		if err != nil {
			logger.V(1).Info("error getting current primary client", "err", err, "pod", pod)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		if err := topology.ConfigurePrimary(ctx, client); err != nil {
			return ctrl.Result{}, fmt.Errorf("error configuring primary: %v", err)
		}
		return ctrl.Result{}, nil
	}
	if !opts.forceReplicaConfiguration {
		role, ok := replRoles[pod]
		if ok && role == mariadbv1alpha1.ReplicationRoleReplica {

			// If not external or is in recovery, we can skip configuration drift checks
			if !isExternalReplication || req.mariadb.IsRecoveringReplicas() {
				return ctrl.Result{}, nil
			}
			// For external replication the master connection details live in the ExternalMariaDB
			// resource and may change over time. Detect drift and re-point the replica without
			// resetting the master/GTID position.
			client, err := req.replClientSet.clientForIndex(ctx, podIndex)
			if err != nil {
				logger.V(1).Info("error getting replica client", "err", err, "pod", pod)
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			defer client.Close()
			// convert topology to externalReplicationTopology to access external-replication specific methods
			externalReplicationTopology, ok := topology.(*externalReplicationTopology)
			if !ok {
				logger.Error(nil, "error converting topology to externalReplicationTopology", "pod", pod)
				return ctrl.Result{}, fmt.Errorf("error converting topology to externalReplicationTopology: %v", err)
			}

			if _, err := r.ReconcileExternalReplicaDrift(ctx, externalReplicationTopology,
				req.mariadb, client, primaryPodIndex, logger); err != nil {
				logger.Error(err, "error reconciling external replica drift", "pod", pod)
				return ctrl.Result{}, fmt.Errorf("error reconciling external replica drift: %v", err)
			}
			return ctrl.Result{}, nil
		}
	}

	client, err := req.replClientSet.clientForIndex(ctx, podIndex)
	if err != nil {
		logger.V(1).Info("error getting replica client", "err", err, "pod", pod)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	defer client.Close()
	logger.Info("Configuring replica", "pod", pod)

	replicaOpts, err := r.getReplicaOpts(ctx, req, pod, podIndex, logger, reconcilePodOpts...)
	if err != nil {
		logger.Error(err, "error getting replica opts", "error", err, "pod", pod)
		return ctrl.Result{}, fmt.Errorf("error getting replica opts: %v", err)
	}
	if err := topology.ConfigureReplica(ctx, client, primaryPodIndex, replicaOpts...); err != nil {
		logger.Error(err, "error configuring replica")
		return ctrl.Result{}, fmt.Errorf("error configuring replica: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *ReplicationReconciler) getReplicaOpts(ctx context.Context, req *ReconcileRequest, pod string, index int,
	logger logr.Logger, reconcilePodOpts ...ReconcilePodOpt) ([]ConfigureReplicaOpt, error) {
	opts := ReconcilePodOpts{}
	for _, setOpt := range reconcilePodOpts {
		setOpt(&opts)
	}
	if !opts.forceReplicaConfiguration {
		return nil, nil
	}

	var gtid string
	if opts.volumeSnapshotKey != nil {
		var snapshot volumesnapshotv1.VolumeSnapshot
		if err := r.Get(ctx, *opts.volumeSnapshotKey, &snapshot); err != nil {
			return nil, fmt.Errorf("error getting %s VolumeSnapshot: %v", (*opts.volumeSnapshotKey).Name, err)
		}
		snapshotGtid, ok := snapshot.Annotations[metadata.GtidAnnotation]
		if !ok {
			return nil, fmt.Errorf("could not find GTID annotation %s in VolumeSnapshot", metadata.GtidAnnotation)
		}

		gtid = snapshotGtid
		logger.Info("Got replica GTID from VolumeSnapshot", "pod", pod, "gtid", gtid, "snapshot", snapshot.Name)
	} else {
		agentClient, err := req.agentClientSet.ClientForIndex(index)
		if err != nil {
			return nil, fmt.Errorf("error getting agent client: %v", err)
		}
		agentGtid, err := agentClient.Replication.GetGtid(ctx)
		if err != nil {
			return nil, fmt.Errorf("error requesting GTID to agent: %v", err)
		}

		gtid = agentGtid
		logger.Info("Got replica GTID from agent", "pod", pod, "gtid", gtid)
	}

	changeMasterGtid, err := mariadbv1alpha1.GtidSlavePos.MariaDBFormat()
	if err != nil {
		return nil, fmt.Errorf("error getting change master GTID: %v", err)
	}
	replicaOpts := []ConfigureReplicaOpt{
		WithGtidSlavePos(gtid),
		WithChangeMasterOpts(
			sql.WithChangeMasterGtid(changeMasterGtid),
		),
	}
	// avoid deleting binary logs during archival to prevent drifting from object storage
	if req.mariadb.IsPointInTimeRecoveryEnabled() {
		replicaOpts = append(replicaOpts, WithResetMaster(false))
	}
	return replicaOpts, nil
}

func (r *ReplicationReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return r.Status().Patch(ctx, mariadb, patch)
}

// ReconcileExternalReplicaDrift re-points a replica that is already configured for external
// replication at the current ExternalMariaDB connection details when it has drifted.
//
// It repairs in two cases:
//   - The configured master host, port or user no longer matches the ExternalMariaDB endpoint.
//   - The replica IO thread is failing with an authentication error. The configured password
//     cannot be read back, so re-issuing CHANGE MASTER (which always re-sends the current secret
//     value) is how a rotated replication password gets applied.
//
// Unlike ConfigureReplica, this performs a minimal, non-destructive repair: it does NOT reset the
// master nor require a GTID position. It only stops the slave threads (when they are running),
// issues CHANGE MASTER with the updated connection details (keeping MASTER_USE_GTID=current_pos)
// and starts the slave again. It returns true when a repair was performed.
func (r *ReplicationReconciler) ReconcileExternalReplicaDrift(ctx context.Context, topology *externalReplicationTopology,
	mariadb *mariadbv1alpha1.MariaDB, client *sql.Client, primaryPodIndex int, logger logr.Logger) (bool, error) {
	desiredHost, desiredPort, desiredUser, err := r.externalMasterEndpoint(ctx, mariadb)
	if err != nil {
		return false, fmt.Errorf("error getting external master endpoint: %v", err)
	}

	// Read SHOW REPLICA STATUS as a column map rather than via a positional scan so the check is
	// resilient to column ordering changes across MariaDB versions.
	status, err := client.QueryColumnMap(ctx, "SHOW REPLICA STATUS")
	if err != nil {
		return false, fmt.Errorf("error getting replica status: %v", err)
	}
	currentHost := status["Master_Host"]
	currentPort := status["Master_Port"]
	currentUser := status["Master_User"]
	ioRunning := status["Slave_IO_Running"]
	sqlRunning := status["Slave_SQL_Running"]

	endpointDrift := currentHost != desiredHost ||
		currentPort != strconv.Itoa(int(desiredPort)) ||
		currentUser != desiredUser
	authError := isReplicaAuthError(ioRunning, status["Last_IO_Errno"], status["Last_IO_Error"])

	if !endpointDrift && !authError {
		return false, nil
	}

	if endpointDrift {
		logger.Info("external replica master drift detected, repairing",
			"current-host", currentHost, "current-port", currentPort, "current-user", currentUser,
			"desired-host", desiredHost, "desired-port", desiredPort, "desired-user", desiredUser)
	}
	if authError {
		logger.Info("external replica authentication error detected, re-applying credentials",
			"last-io-errno", status["Last_IO_Errno"], "last-io-error", status["Last_IO_Error"])
	}

	if ioRunning != "No" || sqlRunning != "No" {
		if err := client.StopAllSlaves(ctx); err != nil {
			return false, fmt.Errorf("error stopping slaves: %v", err)
		}
	}
	if err := topology.changeMaster(ctx, client, primaryPodIndex); err != nil {
		return false, fmt.Errorf("error changing master: %v", err)
	}
	if err := client.StartSlave(ctx); err != nil {
		return false, fmt.Errorf("error starting slave: %v", err)
	}
	return true, nil
}

func shouldSkipPrimaryReconciliation(mariadb *mariadbv1alpha1.MariaDB, replRoles map[string]mariadbv1alpha1.ReplicationRole,
	pod string, logger logr.Logger) bool {
	role, ok := replRoles[pod]
	if !ok {
		logger.V(1).Info("Primary Pod role not yet assigned. Skipping reconciliation...", "pod", pod)
		return true
	}
	if mariadb.IsMultiClusterReplica() {
		return role == mariadbv1alpha1.ReplicationRolePrimaryReplica
	}
	return role == mariadbv1alpha1.ReplicationRolePrimary
}

// isReplicaAuthError reports whether the replica IO thread is failing to authenticate against
// the master. The password configured on the replica cannot be read back, so an access-denied
// error is our only signal that a rotated password needs to be re-applied.
func isReplicaAuthError(ioRunning, lastIOErrno, lastIOError string) bool {
	if ioRunning == "Yes" {
		return false
	}
	if errno, err := strconv.Atoi(lastIOErrno); err == nil {
		if _, ok := replicaAuthErrnos[int32(errno)]; ok {
			return true
		}
	}
	return strings.Contains(strings.ToLower(lastIOError), "access denied")
}

// externalMasterEndpoint resolves the master connection details (host, port and user) that an
// external replica should currently be replicating from, reading them from the referenced
// ExternalMariaDB resource.
func (r *ReplicationReconciler) externalMasterEndpoint(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB) (host string, port int32, user string, err error) {
	replication := mariadb.Replication()
	emdbRef := replication.GetExternalReplicationRef()
	emdb, err := r.refResolver.ExternalMariaDB(ctx, &emdbRef, mariadb.Namespace)
	if err != nil {
		return "", 0, "", fmt.Errorf("error getting ExternalMariaDB: %v", err)
	}
	port = emdb.GetPort()
	if emdb.GetBinlogProxyPort() != nil {
		port = *emdb.GetBinlogProxyPort()
	}
	return emdb.GetHost(), port, emdb.GetSUName(), nil
}

// replicaAuthErrnos are the IO thread error codes MariaDB reports when the replica cannot
// authenticate against the master, e.g. after the replication password has been rotated.
var replicaAuthErrnos = map[int32]struct{}{
	1045: {}, // ER_ACCESS_DENIED_ERROR
	1698: {}, // ER_ACCESS_DENIED_NO_PASSWORD_ERROR
}
