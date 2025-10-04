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

type reconcileRequest struct {
	mariadb        *mariadbv1alpha1.MariaDB
	key            types.NamespacedName
	replClientSet  *ReplicationClientSet
	agentClientSet *agentclient.ClientSet
	replicasSynced bool
}

func (r *reconcileRequest) close() error {
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

	req, err := r.newReconcileRequest(ctx, mdb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating reconcile request: %v", err)
	}
	defer req.close()

	if !mdb.IsMaxScaleEnabled() && mdb.IsSwitchoverRequired() {
		return ctrl.Result{}, r.reconcileSwitchover(ctx, req, switchoverLogger)
	}
	if result, err := r.reconcileReplication(ctx, req, logger); !result.IsZero() || err != nil {
		return result, err
	}
	return ctrl.Result{}, r.reconcileSwitchover(ctx, req, switchoverLogger)
}

func (r *ReplicationReconciler) newReconcileRequest(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (*reconcileRequest, error) {
	replClientSet, err := NewReplicationClientSet(mdb, r.refResolver)
	if err != nil {
		return nil, fmt.Errorf("error creating mariadb clientset: %v", err)
	}
	agentClientSet, err := agentclient.NewClientSet(ctx, mdb, r.env, r.refResolver)
	if err != nil {
		return nil, fmt.Errorf("error getting agent clientset: %v", err)
	}
	return &reconcileRequest{
		mariadb:        mdb,
		key:            client.ObjectKeyFromObject(mdb),
		replClientSet:  replClientSet,
		agentClientSet: agentClientSet,
		replicasSynced: false,
	}, nil
}

func (r *ReplicationReconciler) reconcileReplication(ctx context.Context, req *reconcileRequest, logger logr.Logger) (ctrl.Result, error) {
	if result, err := r.shouldReconcileReplication(ctx, req, logger); !result.IsZero() || err != nil {
		return result, err
	}
	for _, i := range r.replicationPodIndexes(ctx, req) {
		if result, err := r.reconcileReplicationInPod(ctx, req, logger, i); !result.IsZero() || err != nil {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *ReplicationReconciler) shouldReconcileReplication(ctx context.Context, req *reconcileRequest,
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

func (r *ReplicationReconciler) replicationPodIndexes(ctx context.Context, req *reconcileRequest) []int {
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

func (r *ReplicationReconciler) reconcileReplicationInPod(ctx context.Context, req *reconcileRequest, logger logr.Logger,
	index int) (ctrl.Result, error) {
	primaryPodIndex := *req.mariadb.Status.CurrentPrimaryPodIndex
	replStatus := ptr.Deref(req.mariadb.Status.Replication, mariadbv1alpha1.ReplicationStatus{})
	replState := replStatus.State
	pod := statefulset.PodName(req.mariadb.ObjectMeta, index)

	if primaryPodIndex == index {
		if rs, ok := replState[pod]; ok && rs == mariadbv1alpha1.ReplicationStatePrimary {
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

	rs, ok := replState[pod]
	if ok && rs == mariadbv1alpha1.ReplicationStateReplica && !req.mariadb.ReplicaNeedsConfiguration(pod) {
		return ctrl.Result{}, nil
	}
	client, err := req.replClientSet.clientForIndex(ctx, index)
	if err != nil {
		logger.V(1).Info("error getting replica client", "err", err, "pod", pod)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	logger.Info("Configuring replica", "pod", pod)

	replicaOpts, err := r.getReplicaOpts(ctx, req, pod, index, logger)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting replica opts: %v", err)
	}
	if err := r.replConfigClient.ConfigureReplica(ctx, req.mariadb, client, primaryPodIndex, replicaOpts...); err != nil {
		return ctrl.Result{}, fmt.Errorf("error configuring replica: %v", err)
	}

	if err := r.markReplicaAsConfigured(ctx, req, pod, logger); err != nil {
		return ctrl.Result{}, fmt.Errorf("error marking replica as configured: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *ReplicationReconciler) getReplicaOpts(ctx context.Context, req *reconcileRequest, pod string, index int,
	logger logr.Logger) ([]ConfigureReplicaOpt, error) {
	if !req.mariadb.ReplicaNeedsConfiguration(pod) {
		return nil, nil
	}
	bootstrapFrom := ptr.Deref(req.mariadb.Spec.BootstrapFrom, mariadbv1alpha1.BootstrapFrom{})

	var gtid string
	if bootstrapFrom.VolumeSnapshotRef != nil {
		snapshotKey := types.NamespacedName{
			Name:      bootstrapFrom.VolumeSnapshotRef.Name,
			Namespace: req.mariadb.Namespace,
		}
		var snapshot volumesnapshotv1.VolumeSnapshot
		if err := r.Get(ctx, snapshotKey, &snapshot); err != nil {
			return nil, fmt.Errorf("error getting bootstrap VolumeSnapshot: %v", err)
		}
		snapshotGtid, ok := snapshot.Annotations[metadata.GtidAnnotation]
		if !ok {
			return nil, fmt.Errorf("could not find GTID annotation \"%s\" in VolumeSnapshot", metadata.GtidAnnotation)
		}

		gtid = snapshotGtid
		logger.Info("Got replica GTID from VolumeSnapshot", "gtid", gtid)
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
		logger.Info("Got replica GTID from agent", "gtid", gtid)
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

func (r *ReplicationReconciler) markReplicaAsConfigured(ctx context.Context, req *reconcileRequest, pod string, logger logr.Logger) error {
	if !req.mariadb.ReplicaNeedsConfiguration(pod) {
		return nil
	}
	logger.Info("Marking replica as configured", "replica", pod)
	return r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		req.mariadb.MarkReplicaAsConfigured(pod)
	})
}

func (r *ReplicationReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return r.Status().Patch(ctx, mariadb, patch)
}
