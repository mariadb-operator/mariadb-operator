package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/replication"
	jobpkg "github.com/mariadb-operator/mariadb-operator/v25/pkg/job"
	podpkg "github.com/mariadb-operator/mariadb-operator/v25/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/pvc"
	stsobj "github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	mdbsnapshot "github.com/mariadb-operator/mariadb-operator/v25/pkg/volumesnapshot"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/wait"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MariaDBReconciler) reconcileInit(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("init")

	if mariadb.Spec.BootstrapFrom != nil && mariadb.Spec.BootstrapFrom.BackupContentType == mariadbv1alpha1.BackupContentTypePhysical {
		return r.reconcilePhysicalBackupInit(ctx, mariadb, logger)
	} else if mariadb.IsGaleraEnabled() {
		if result, err := r.GaleraReconciler.ReconcileInit(ctx, mariadb); !result.IsZero() || err != nil {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcilePhysicalBackupInit(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (ctrl.Result, error) {
	if mariadb.IsInitialized() {
		return ctrl.Result{}, nil
	}

	if !mariadb.IsInitializing() || mariadb.InitError() != nil {
		pvcs, err := pvc.ListStoragePVCs(ctx, r.Client, mariadb)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error listing PVCs: %v", err)
		}
		if len(pvcs) > 0 {
			r.Recorder.Eventf(mariadb, corev1.EventTypeWarning, mariadbv1alpha1.ReasonMariaDBInitError,
				"Unable to init MariaDB: storage PVCs already exist")

			if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
				condition.SetInitError(status, "storage PVCs already exist")
				return nil
			}); err != nil {
				return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
			}

			logger.Info("Unable to init MariaDB: storage PVCs already exist. Requeuing...")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetInitializing(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}

	bootstrapFrom := ptr.Deref(mariadb.Spec.BootstrapFrom, mariadbv1alpha1.BootstrapFrom{})
	fromIndex := 0 // init process reconciles all Pods

	var snapshotKey *types.NamespacedName
	if bootstrapFrom.VolumeSnapshotRef != nil {
		snapshotKey = &types.NamespacedName{
			Name:      bootstrapFrom.VolumeSnapshotRef.Name,
			Namespace: mariadb.Namespace,
		}
	}
	if result, err := r.reconcilePVCs(ctx, mariadb, fromIndex, snapshotKey); !result.IsZero() || err != nil {
		return result, err
	}
	if result, err := r.reconcileStagingPVC(ctx, mariadb); !result.IsZero() || err != nil {
		return result, err
	}

	if bootstrapFrom.VolumeSnapshotRef == nil {
		if result, err := r.reconcileRollingInitJobs(
			ctx,
			mariadb,
			fromIndex,
			logger,
			withRestoreOpts(
				builder.WithBootstrapFrom(mariadb.Spec.BootstrapFrom),
			),
		); !result.IsZero() || err != nil {
			return result, err
		}
		if err := r.cleanupInitJobs(ctx, mariadb); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.cleanupStagingPVC(ctx, mariadb); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.ensureReplicasConfigured(ctx, fromIndex, mariadb, snapshotKey, logger); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetInitialized(status)
		condition.SetRestoredPhysicalBackup(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}

	// Requeue to track replication status
	if mariadb.IsReplicationEnabled() {
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcilePVCs(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, fromIndex int,
	snapshotKey *types.NamespacedName) (ctrl.Result, error) {
	var pvcOpts []builder.PVCOption
	if snapshotKey != nil {
		if result, err := r.waitForReadyVolumeSnapshot(ctx, *snapshotKey); !result.IsZero() || err != nil {
			return result, err
		}
		pvcOpts = append(
			pvcOpts,
			builder.WithVolumeSnapshotDataSource(snapshotKey.Name),
		)
	}

	for i := fromIndex; i < int(mariadb.Spec.Replicas); i++ {
		key := mariadb.PVCKey(builder.StorageVolume, i)
		pvc, err := r.Builder.BuildStoragePVC(key, mariadb.Spec.Storage.VolumeClaimTemplate, mariadb, pvcOpts...)
		if err != nil {
			return ctrl.Result{}, err
		}
		if err := r.PVCReconciler.Reconcile(ctx, key, pvc); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcileStagingPVC(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if shouldProvisionPhysicalBackupStagingPVC(mariadb) {
		key := mariadb.PhysicalBackupStagingPVCKey()
		pvc, err := r.Builder.BuildBackupStagingPVC(
			key,
			mariadb.Spec.BootstrapFrom.StagingStorage.PersistentVolumeClaim,
			mariadb.Spec.InheritMetadata,
			mariadb,
		)
		if err != nil {
			return ctrl.Result{}, err
		}
		if err := r.PVCReconciler.Reconcile(ctx, key, pvc); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) waitForReadyVolumeSnapshot(ctx context.Context, key types.NamespacedName) (ctrl.Result, error) {
	var snapshot volumesnapshotv1.VolumeSnapshot
	if err := r.Get(ctx, key, &snapshot); err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting VolumeSnapshot: %v", err)
	}
	if !mdbsnapshot.IsVolumeSnapshotReady(&snapshot) {
		log.FromContext(ctx).Info("VolumeSnapshot not ready. Requeuing...", "snapshot", snapshot.Name, "status", snapshot.Status)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

type rollingInitOpts struct {
	restoreOpts []builder.PhysicalBackupRestoreOpt
}

type rollingInitOpt func(*rollingInitOpts)

func withRestoreOpts(opts ...builder.PhysicalBackupRestoreOpt) rollingInitOpt {
	return func(rio *rollingInitOpts) {
		rio.restoreOpts = opts
	}
}

func (r *MariaDBReconciler) reconcileRollingInitJobs(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	fromIndex int, logger logr.Logger, riopts ...rollingInitOpt) (ctrl.Result, error) {
	opts := rollingInitOpts{}
	for _, setOpt := range riopts {
		setOpt(&opts)
	}

	return r.forEachMariaDBPod(mariadb, fromIndex, func(podIndex int) (ctrl.Result, error) {
		physicalBackupKey := mariadb.PhysicalBackupInitJobKey(podIndex)
		pod := stsobj.PodName(mariadb.ObjectMeta, podIndex)

		logger.V(1).Info("Reconciling init Job", "pod-index", podIndex)
		if result, err := r.reconcileAndWaitForInitJob(
			ctx,
			mariadb,
			physicalBackupKey,
			podIndex,
			opts.restoreOpts...,
		); !result.IsZero() || err != nil {
			return result, err
		}

		newReplicas := int32(podIndex + 1)
		logger.V(1).Info("Upscaling StatefulSet", "replicas", newReplicas)
		if err := r.upscaleStatefulSet(ctx, mariadb, newReplicas); err != nil {
			return ctrl.Result{}, fmt.Errorf("error upscaling StatefulSet: %v", err)
		}
		if result, err := r.waitForPodScheduled(ctx, mariadb, podIndex); !result.IsZero() || err != nil {
			return result, err
		}
		logger.V(1).Info("Pod successfully initialized", "pod", pod)

		return ctrl.Result{}, nil
	})
}

func (r *MariaDBReconciler) reconcileAndWaitForInitJob(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	key types.NamespacedName, podIndex int, restoreOpts ...builder.PhysicalBackupRestoreOpt) (ctrl.Result, error) {
	var job batchv1.Job
	if err := r.Get(ctx, key, &job); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.createInitJob(ctx, mariadb, key, podIndex, restoreOpts...); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
	}
	if !jobpkg.IsJobComplete(&job) {
		log.FromContext(ctx).V(1).Info("PhysicalBackup init job not completed. Requeuing")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) createInitJob(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	key types.NamespacedName, podIndex int, restoreOpts ...builder.PhysicalBackupRestoreOpt) error {
	job, err := r.Builder.BuildPhysicalBackupRestoreJob(
		key,
		mariadb,
		&podIndex,
		restoreOpts...,
	)
	if err != nil {
		return fmt.Errorf("error building PhysicalBackup init Job: %v", err)
	}
	return r.Create(ctx, job)
}

func (r *MariaDBReconciler) upscaleStatefulSet(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, replicas int32) error {
	key := client.ObjectKeyFromObject(mariadb)

	var sts appsv1.StatefulSet
	if err := r.Get(ctx, key, &sts); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting StatefulSet: %v", err)
		}

		updateAnnotations, err := r.getUpdateAnnotations(ctx, mariadb)
		if err != nil {
			return fmt.Errorf("error getting Pod annotations: %v", err)
		}
		desiredSts, err := r.Builder.BuildMariadbStatefulSet(mariadb, key, updateAnnotations)
		if err != nil {
			return fmt.Errorf("error building StatefulSet: %v", err)
		}
		sts = *desiredSts
	}
	if sts.Status.Replicas >= replicas {
		return nil
	}
	sts.Spec.Replicas = &replicas

	if err := r.StatefulSetReconciler.Reconcile(ctx, &sts); err != nil {
		return fmt.Errorf("error reconciling StatefulSet with %d replicas : %v", replicas, err)
	}
	return nil
}

func (r *MariaDBReconciler) waitForPodScheduled(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, podIndex int) (ctrl.Result, error) {
	podKey := types.NamespacedName{
		Name:      stsobj.PodName(mariadb.ObjectMeta, podIndex),
		Namespace: mariadb.Namespace,
	}
	var pod corev1.Pod
	if err := r.Get(ctx, podKey, &pod); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
		return ctrl.Result{}, fmt.Errorf("error getting Pod: %v", err)
	}

	if !podpkg.PodScheduled(&pod) {
		log.FromContext(ctx).V(1).Info("Pod has not been scheduled. Requeuing...", "pod", pod.Name)
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) ensureReplicasConfigured(ctx context.Context, fromIndex int, mariadb *mariadbv1alpha1.MariaDB,
	snapshotKey *types.NamespacedName, logger logr.Logger) error {
	if !mariadb.IsReplicationEnabled() {
		return nil
	}
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return errors.New("'status.currentPrimaryPodIndex' must be set")
	}

	_, err := r.forEachMariaDBPod(mariadb, fromIndex, func(podIndex int) (ctrl.Result, error) {
		pod := stsobj.PodName(mariadb.ObjectMeta, podIndex)

		if *mariadb.Status.CurrentPrimaryPodIndex != podIndex && !mariadb.IsConfiguredReplica(pod) {
			if err := r.ensureReplicaConfigured(
				ctx,
				pod,
				mariadb,
				snapshotKey,
				logger,
			); err != nil {
				return ctrl.Result{}, fmt.Errorf("error configuring Pod %s: %v", pod, err)
			}
		}
		return ctrl.Result{}, nil
	})
	return err
}

func (r *MariaDBReconciler) ensureReplicaConfigured(ctx context.Context, replica string, mariadb *mariadbv1alpha1.MariaDB,
	snapshotKey *types.NamespacedName, logger logr.Logger) error {
	if !mariadb.IsReplicationEnabled() {
		return nil
	}
	podIndex, err := stsobj.PodIndex(replica)
	if err != nil {
		return fmt.Errorf("error getting replica pod index: %v", err)
	}
	req, err := r.ReplicationReconciler.NewReconcileRequest(ctx, mariadb)
	if err != nil {
		return fmt.Errorf("error creating replication reconcile request: %v", err)
	}
	defer req.Close()

	pollCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	return wait.PollUntilSuccessOrContextCancel(pollCtx, logger, func(ctx context.Context) error {
		if result, err := r.ReplicationReconciler.ReconcileReplicationInPod(
			ctx,
			req,
			*podIndex,
			logger,
			replication.WithForceReplicaConfiguration(true),
			replication.WithVolumeSnapshotKey(snapshotKey),
		); !result.IsZero() || err != nil {
			return errors.New("replication not configured")
		}
		return nil
	})
}

func (r *MariaDBReconciler) cleanupInitJobs(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	_, err := r.forEachMariaDBPod(mariadb, 0, func(podIndex int) (ctrl.Result, error) {
		key := mariadb.PhysicalBackupInitJobKey(podIndex)
		var job batchv1.Job
		if err := r.Get(ctx, key, &job); err == nil {
			if err := r.Delete(ctx, &job, &client.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationBackground)}); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
			}
		}
		return ctrl.Result{}, nil
	})
	return err
}

func (r *MariaDBReconciler) cleanupStagingPVC(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if !shouldProvisionPhysicalBackupStagingPVC(mariadb) {
		return nil
	}
	key := mariadb.PhysicalBackupStagingPVCKey()
	var pvc corev1.PersistentVolumeClaim
	if err := r.Get(ctx, key, &pvc); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return r.Delete(ctx, &pvc)
}

func (r *MariaDBReconciler) forEachMariaDBPod(mariadb *mariadbv1alpha1.MariaDB, fromIndex int,
	fn func(podIndex int) (ctrl.Result, error)) (ctrl.Result, error) {
	for i := fromIndex; i < int(mariadb.Spec.Replicas); i++ {
		if result, err := fn(i); !result.IsZero() || err != nil {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

func shouldProvisionPhysicalBackupStagingPVC(mariadb *mariadbv1alpha1.MariaDB) bool {
	b := mariadb.Spec.BootstrapFrom
	if b == nil {
		return false
	}
	return b.BackupContentType == mariadbv1alpha1.BackupContentTypePhysical &&
		b.S3 != nil && b.StagingStorage != nil && b.StagingStorage.PersistentVolumeClaim != nil
}
