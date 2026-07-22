package controller

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	stsobj "github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	"golang.org/x/mod/semver"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcileExternalReplInit handles the initialization of external replication for a MariaDB cluster.
// It ensures that a backup of the external MariaDB is taken and restored to the replica pods
// before marking the external replication as initialized.
func (r *MariaDBReconciler) reconcileExternalReplInit(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("external-repl-init")

	if !mariadb.IsReplicationEnabled() {
		logger.Info("replication is not enabled")
		return ctrl.Result{}, nil
	}
	replication := mariadb.Replication()

	if !replication.IsExternalReplication() {
		logger.Info("replication is enabled but is not external")
		return ctrl.Result{}, nil
	}

	if mariadb.HasConfiguredReplication() {
		logger.Info("replication is enabled and external but replication it is already configured")
		return ctrl.Result{}, nil
	}

	if mariadb.IsExternalReplInitialized() {
		logger.Info("external replication already initialized")
		return ctrl.Result{}, nil
	}

	logger.Info("reconciling external replication init")
	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetExternalReplInitializing(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}

	// Ensure External MariaDB is ready before proceeding with the backup and restore

	emdb, err := r.RefResolver.ExternalMariaDB(ctx, &replication.ReplicaFromExternal.MariaDBRef.ObjectReference, mariadb.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting external MariaDB object: %v", err)
	}

	if !emdb.IsReady() {
		logger.Info("external MariaDB is not ready")
		return ctrl.Result{RequeueAfter: time.Minute * 1}, nil
	}

	logger.Info("reconciling logical backup")
	if result, err := r.reconcileLogicalBackup(ctx, mariadb, replication, logger); err != nil || !result.IsZero() {
		return result, err
	}

	logger.Info("reconciling logical restore on each pod")
	total_pods := 0
	total_restored_pods := 0
	for _, i := range r.replicationPodIndexes(mariadb) {
		total_pods++
		if _, err := r.reconcileRestoreInPod(ctx, mariadb, i, logger, false); err == nil {
			total_restored_pods++
		}
	}

	if total_pods != total_restored_pods {
		logger.Info("restore in pod in-progress")
		return ctrl.Result{RequeueAfter: time.Minute * 1}, nil
	}

	//cleanup the restore
	logger.Info("cleaning up the restore on each pod")
	for _, i := range r.replicationPodIndexes(mariadb) {
		_ = r.cleanupRestoreInPod(ctx, mariadb, i, logger)
	}

	logger.Info("reconciling restore finished")

	logger.Info("setting ExternalReplInitialized status to true")
	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetExternalReplInitialized(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}

	logger.Info("reconciling external replication init finished")
	return ctrl.Result{}, nil
}

// reconcileRestoreInPod ensures that the given replica pod index is restored from the backup taken from the external MariaDB.
func (r *MariaDBReconciler) reconcileRestoreInPod(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	replicaPodIndex int, logger logr.Logger, removeCurrentPod bool) (ctrl.Result, error) {

	logger.Info("reconciling restore in pod", "pod", replicaPodIndex)

	replClientSet, err := replication.NewReplicationClientSet(mariadb, r.RefResolver)
	if err != nil {
		logger.Error(err, "error getting replica clientset", "err", err, "pod", replicaPodIndex)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	client, err := replClientSet.ClientForIndex(ctx, replicaPodIndex)
	if err != nil {
		logger.Error(err, "error getting replica client", "err", err, "pod", replicaPodIndex)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}
	defer client.Close()

	if err := client.ResetMaster(ctx); err != nil {
		logger.Error(err, "error resetting master")
		return ctrl.Result{}, fmt.Errorf("error resetting master: %v", err)
	}

	var existingRestore mariadbv1alpha1.Restore
	err = r.Get(ctx, mariadb.RestoreKeyInPod(replicaPodIndex), &existingRestore)

	if err == nil && !existingRestore.IsComplete() {
		logger.Info("restore exists, but not complete", "pod", replicaPodIndex)
		return ctrl.Result{RequeueAfter: time.Second * 10}, fmt.Errorf("restore is not complete")
	}

	podKey := types.NamespacedName{
		Name:      stsobj.PodName(*mariadb.GetObjectMeta(), replicaPodIndex),
		Namespace: mariadb.Namespace,
	}

	if !existingRestore.IsComplete() {
		// Restore/Bootstrap node from backup
		logger.Info("restore does not exists, create a new restore", "pod", replicaPodIndex)

		if removeCurrentPod {
			logger.Info("Recreating Pod")

			pvcKey := mariadb.PVCKey(builder.StorageVolume, replicaPodIndex)
			var pvc corev1.PersistentVolumeClaim
			if err := r.Get(ctx, pvcKey, &pvc); err != nil {
				return ctrl.Result{}, fmt.Errorf("error getting pvc from Pod '%v': %v", replicaPodIndex, err)
			}
			if err := r.Delete(ctx, &pvc); err != nil {
				return ctrl.Result{}, fmt.Errorf("error deleting pvc from Pod '%v': %v", replicaPodIndex, err)
			}

			var existingPod corev1.Pod
			if err := r.Get(ctx, podKey, &existingPod); err != nil {
				return ctrl.Result{}, fmt.Errorf("error getting Pod '%v': %v", replicaPodIndex, err)
			}
			if err := r.Delete(ctx, &existingPod); err != nil {
				return ctrl.Result{}, fmt.Errorf("error deleting Pod '%v': %v", replicaPodIndex, err)
			}
		}
		err := newRestore(mariadb, *r, ctx, replicaPodIndex)
		return ctrl.Result{}, fmt.Errorf("new restore attempt%v", err)
	}

	if err != nil {
		logger.Error(err, "error creating new restore", "pod", replicaPodIndex)
		return ctrl.Result{}, fmt.Errorf("error creating new restore: %v", err)
	}

	logger.Info("restore complete", "pod", replicaPodIndex)

	return ctrl.Result{}, nil
}

// cleanupRestoreInPod deletes the Restore object for the given replica pod index.
// This is used to clean up the Restore object after the restore is complete.
func (r *MariaDBReconciler) cleanupRestoreInPod(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	replicaPodIndex int, logger logr.Logger) error {

	logger.Info("cleaning up restore in pod", "pod", replicaPodIndex)

	var existingRestore mariadbv1alpha1.Restore
	err := r.Get(ctx, mariadb.RestoreKeyInPod(replicaPodIndex), &existingRestore)

	if err == nil {
		if err := r.Delete(ctx, &existingRestore); err != nil {
			logger.Info("failed to delete restore", "pod", replicaPodIndex)
			return fmt.Errorf("error deleting Restore: %v", err)
		}
	}
	return nil
}

// handleInitialBackup ensures that a valid backup exists for the external MariaDB. If a backup does not exist, it creates a new one.
func (r *MariaDBReconciler) reconcileLogicalBackup(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	replication mariadbv1alpha1.Replication, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Reconciling initial logical backup for external replication")

	emdb, err := r.RefResolver.ExternalMariaDB(ctx, &replication.ReplicaFromExternal.MariaDBRef.ObjectReference, mariadb.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting external MariaDB object: %v", err)
	}
	key := types.NamespacedName{
		Name:      mariadb.ExternalReplLogicalBackupName(),
		Namespace: emdb.Namespace,
	}

	logger.Info("Checking if viable backup already exists")
	var isBackupInvalid = false
	var binlogExpireLogsDuration time.Duration
	var existingBackup mariadbv1alpha1.Backup

	logger.Info("Getting the binlog_expire_logs_seconds on the external MariaDB")
	if binlogExpireLogsDuration, err = getBinlogExpireLogsDuration(emdb, ctx, r.RefResolver, logger); err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to get binlog_expire_logs_seconds: %v", err)
	}

	logger.Info("Trying to get the current backup")
	err = r.Get(ctx, key, &existingBackup)

	if err == nil {
		logger.Info("Backup exists, check if it is expired. binlogExpireLogsDuration", "duration", binlogExpireLogsDuration.String())
		isBackupInvalid = removeBackupIfExpired(existingBackup, ctx, binlogExpireLogsDuration, *r, logger)
	}

	// Create a new backup if required
	if err != nil || isBackupInvalid {
		logger.Info("Take a new backup")
		template, err := r.getLogicalBackupTemplate(ctx, mariadb, replication)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting logical backup template: %v", err)
		}
		backup_error := newBackup(emdb, *r, ctx, binlogExpireLogsDuration, mariadb.GetImagePullSecrets(), mariadb.Spec.Storage.Size,
			key, replication.ReplicaFromExternal.FilteredReplicaTables, template)
		return ctrl.Result{RequeueAfter: time.Minute * 1}, backup_error
	}

	if !existingBackup.IsComplete() {
		logger.Info("Backup is running")
		return ctrl.Result{RequeueAfter: time.Minute * 1}, nil
	}

	if existingBackup.IsFailed() {
		logger.Info("Backup has failed, deleting and retrying")
		if err := r.Delete(ctx, &existingBackup); err != nil {
			return ctrl.Result{}, fmt.Errorf("error deleting failed Backup: %v", err)
		}
		return ctrl.Result{}, fmt.Errorf("backup failed, retrying")
	}

	return ctrl.Result{}, nil
}

// replicationPodIndexes returns the list of pod indexes that are part of the replication setup.
func (r *MariaDBReconciler) replicationPodIndexes(mariadb *mariadbv1alpha1.MariaDB) []int {
	podIndexes := []int{
		*mariadb.Status.CurrentPrimaryPodIndex,
	}
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		if i != *mariadb.Status.CurrentPrimaryPodIndex {
			podIndexes = append(podIndexes, i)
		}
	}
	return podIndexes
}

// removeBackupIfExpired checks if the existing backup is expired based on the binlog_expire_logs_seconds value
// from the external MariaDB.
// If the backup is expired, it deletes the backup and returns true. Otherwise, it returns false.
func removeBackupIfExpired(existingBackup mariadbv1alpha1.Backup, ctx context.Context,
	binlogExpireLogsDuration time.Duration, r MariaDBReconciler, logger logr.Logger) bool {
	if time.Since(existingBackup.CreationTimestamp.Time) > binlogExpireLogsDuration {
		logger.Info("Backup is expired, deleting it", "backup", existingBackup.Name)
		if err := r.Delete(ctx, &existingBackup); err == nil {
			return true
		}
	}
	return false
}

// newBackup creates a Backup object to take a backup of the external MariaDB.
// The backup will be used to restore the replica pods.
func newBackup(emdb *mariadbv1alpha1.ExternalMariaDB, r MariaDBReconciler, ctx context.Context,
	binlogExpireLogsDuration time.Duration, imagePullSecrets []mariadbv1alpha1.LocalObjectReference,
	size *resource.Quantity, key types.NamespacedName, filteredTables []string,
	template *mariadbv1alpha1.Backup) error {

	args := []string{
		"--master-data=1",
		"--gtid",
		"--verbose",
		"--single-transaction",
		"--ignore-table=mysql.global_priv",
	}

	backupOps := builder.BackupOpts{
		Metadata: []*mariadbv1alpha1.Metadata{emdb.Spec.InheritMetadata},
		Key:      key,
		MariaDBRef: mariadbv1alpha1.MariaDBRef{
			ObjectReference: mariadbv1alpha1.ObjectReference{
				Name: emdb.Name,
			},
			Kind: mariadbv1alpha1.ExternalMariaDBKind,
		},
		Args:        args,
		Tables:      filteredTables,
		Compression: mariadbv1alpha1.CompressGzip,
		Storage: mariadbv1alpha1.BackupStorage{
			PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						"storage": *size,
					},
				},
			},
		},
		MaxRetention:     binlogExpireLogsDuration,
		ImagePullSecrets: imagePullSecrets,
		Template:         template,
	}

	backup, err := r.Builder.BuildBackup(backupOps, emdb)
	if err != nil {
		return fmt.Errorf("error building Backup object: %v", err)
	}
	if err := r.Create(ctx, backup); err != nil {
		return fmt.Errorf("error creating base Backup: %v", err)
	}
	return nil
}

// getLogicalBackupTemplate loads the optional Backup template referenced from
// replica.bootstrapFrom.logicalBackupTemplateRef. Returns nil when the user has not configured a template.
func (r *MariaDBReconciler) getLogicalBackupTemplate(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	replication mariadbv1alpha1.Replication) (*mariadbv1alpha1.Backup, error) {
	if replication.Replica.ReplicaBootstrapFrom == nil ||
		replication.Replica.ReplicaBootstrapFrom.LogicalBackupTemplateRef == nil {
		return nil, nil
	}
	tplKey := types.NamespacedName{
		Name:      replication.Replica.ReplicaBootstrapFrom.LogicalBackupTemplateRef.Name,
		Namespace: mariadb.Namespace,
	}
	var tpl mariadbv1alpha1.Backup
	if err := r.Get(ctx, tplKey, &tpl); err != nil {
		return nil, fmt.Errorf("error getting Backup template '%s': %v", tplKey.Name, err)
	}
	return &tpl, nil
}

// getBinlogExpireLogsDuration gets the binlog_expire_logs_seconds value from
// the external MariaDB and returns it as a time.Duration.
func getBinlogExpireLogsDuration(emdb *mariadbv1alpha1.ExternalMariaDB, ctx context.Context,
	refResolver *refresolver.RefResolver, logger logr.Logger) (time.Duration, error) {
	var external_client *sql.Client
	var err error
	if external_client, err = sql.NewClientWithMariaDB(ctx, emdb, refResolver); err != nil {
		return time.Duration(0), fmt.Errorf("error getting external MariaDB client: %v", err)
	}
	defer external_client.Close()

	var binlogExpireLogsSecondsStr string
	var binlogExpireLogsSeconds int

	if semver.Compare("v"+emdb.Status.Version, "v10.6.1") >= 0 {
		logger.Info("Using binlog_expire_logs_seconds", "version", emdb.Status.Version)
		binlogExpireLogsSecondsStr, err = external_client.SystemVariable(ctx, "binlog_expire_logs_seconds")
		if err != nil {
			return time.Duration(0), fmt.Errorf("unable to get binlog_expire_logs_seconds: %v", err)
		}
		binlogExpireLogsSeconds, _ = strconv.Atoi(binlogExpireLogsSecondsStr)
	} else {
		logger.Info("Using expire_logs_days", "version", emdb.Status.Version)
		binlogExpireLogsDaysStr, err := external_client.SystemVariable(ctx, "expire_logs_days")
		if err != nil {
			return time.Duration(0), fmt.Errorf("unable to get expire_logs_days: %v", err)
		}
		binlogExpireLogsDays, _ := strconv.Atoi(binlogExpireLogsDaysStr)
		binlogExpireLogsSeconds = binlogExpireLogsDays * 86400
	}
	logger.Info("binlog expire logs duration", "seconds", binlogExpireLogsSeconds)
	return time.Duration(binlogExpireLogsSeconds) * time.Second, nil
}

// newRestore creates a Restore object for the given replica pod index. The Restore will be responsible for restoring
// the backup taken from the external MariaDB to the replica pod.
func newRestore(mariadb *mariadbv1alpha1.MariaDB, r MariaDBReconciler, ctx context.Context, replicaPodIndex int) error {
	restoreOpts := builder.LogicalRestoreOpts{
		PodIndex: &replicaPodIndex,
	}
	restore, err := r.Builder.BuildRestore(mariadb, mariadb.RestoreKeyInPod(replicaPodIndex), restoreOpts)
	if err != nil {
		return fmt.Errorf("error building Restore object: %v", err)
	}
	if err := r.Create(ctx, restore); err != nil {
		return fmt.Errorf("error creating Restore object: %v", err)
	}
	return nil
}
