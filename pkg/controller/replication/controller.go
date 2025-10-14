package replication

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	agentclient "github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/client"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	conditions "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
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
	recorder            record.EventRecorder
	builder             *builder.Builder
	env                 *environment.OperatorEnv
	replConfigClient    *ReplicationConfigClient
	refResolver         *refresolver.RefResolver
	secretReconciler    *secret.SecretReconciler
	configMapreconciler *configmap.ConfigMapReconciler
	serviceReconciler   *service.ServiceReconciler
}

func NewReplicationReconciler(client client.Client, recorder record.EventRecorder, builder *builder.Builder, env *environment.OperatorEnv,
	replConfigClient *ReplicationConfigClient, opts ...Option) (*ReplicationReconciler, error) {
	r := &ReplicationReconciler{
		Client:           client,
		recorder:         recorder,
		builder:          builder,
		env:              env,
		replConfigClient: replConfigClient,
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
	mariadb        *mariadbv1alpha1.MariaDB
	key            types.NamespacedName
	replClientSet  *ReplicationClientSet
	agentClientSet *agentclient.ClientSet
	replicasSynced bool
}

func (r *ReconcileRequest) Close() error {
	if r.replClientSet != nil {
		r.replClientSet.close()
	}
	return nil
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

	if !mdb.IsMaxScaleEnabled() && mdb.IsSwitchoverRequired() {
		return ctrl.Result{}, r.reconcileSwitchover(ctx, req, switchoverLogger)
	}
	if result, err := r.reconcileReplication(ctx, req, logger); !result.IsZero() || err != nil {
		return result, err
	}
	return ctrl.Result{}, r.reconcileSwitchover(ctx, req, switchoverLogger)
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
		mariadb:        mdb,
		key:            client.ObjectKeyFromObject(mdb),
		replClientSet:  replClientSet,
		agentClientSet: agentClientSet,
		replicasSynced: false,
	}, nil
}

func (r *ReplicationReconciler) reconcileReplication(ctx context.Context, req *ReconcileRequest, logger logr.Logger) (ctrl.Result, error) {
	if result, err := r.shouldReconcileReplication(ctx, req, logger); !result.IsZero() || err != nil {
		return result, err
	}
	for _, i := range r.replicationPodIndexes(ctx, req) {
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
	if req.mariadb.Status.CurrentPrimaryPodIndex == nil {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
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

func (r *ReplicationReconciler) replicationPodIndexes(ctx context.Context, req *ReconcileRequest) []int {
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

	if primaryPodIndex == podIndex {
		if role, ok := replRoles[pod]; ok && role == mariadbv1alpha1.ReplicationRolePrimary {
			return ctrl.Result{}, nil
		}
		client, err := req.replClientSet.currentPrimaryClient(ctx)
		if err != nil {
			logger.V(1).Info("error getting current primary client", "err", err, "pod", pod)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		logger.Info("Configuring primary", "pod", pod)
		if err := r.replConfigClient.ConfigurePrimary(ctx, req.mariadb, client); err != nil {
			return ctrl.Result{}, fmt.Errorf("error configuring replica: %v", err)
		}
		return ctrl.Result{}, nil
	}

	if !opts.forceReplicaConfiguration {
		role, ok := replRoles[pod]
		if ok && role == mariadbv1alpha1.ReplicationRoleReplica {
			return ctrl.Result{}, nil
		}
	}

	client, err := req.replClientSet.clientForIndex(ctx, podIndex)
	if err != nil {
		logger.V(1).Info("error getting replica client", "err", err, "pod", pod)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	logger.Info("Configuring replica", "pod", pod)

	replicaOpts, err := r.getReplicaOpts(ctx, req, pod, podIndex, logger, reconcilePodOpts...)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting replica opts: %v", err)
	}
	if err := r.replConfigClient.ConfigureReplica(ctx, req.mariadb, client, primaryPodIndex, replicaOpts...); err != nil {
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
	return []ConfigureReplicaOpt{
		WithGtidSlavePos(gtid),
		WithChangeMasterOpts(
			sql.WithChangeMasterGtid(changeMasterGtid),
		),
	}, nil
}

func (r *ReplicationReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return r.Status().Patch(ctx, mariadb, patch)
}
