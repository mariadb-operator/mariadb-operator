package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MariaDBReconciler) reconcileScaleOut(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("scale-out")

	isScalingOut, err := r.isScalingOut(ctx, mariadb, logger)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !isScalingOut {
		return ctrl.Result{}, nil
	}

	if !mariadb.IsScalingOut() || mariadb.ScalingOutError() != nil {
		replication := ptr.Deref(mariadb.Spec.Replication, mariadbv1alpha1.Replication{})

		if replication.Replica.ReplicaBootstrapFrom == nil {
			r.Recorder.Eventf(mariadb, corev1.EventTypeWarning, mariadbv1alpha1.ReasonMariaDBScaleOutError,
				"Unable to scale out MariaDB: replica datasource not found (replication.replica.bootstrapFrom is nil)")

			if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
				condition.SetScaleOutError(status, "replica datasource not found (replication.replica.bootstrapFrom is nil)")
				return nil
			}); err != nil {
				return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
			}

			logger.Info("Unable to scale out MariaDB: replica datasource not found (replication.replica.bootstrapFrom is nil). Requeuing...")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetScalingOut(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}

	if result, err := r.reconcilePhysicalBackup(ctx, mariadb, logger); !result.IsZero() || err != nil {
		return result, err
	}

	if err := r.cleanupPhysicalBackup(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetScaledOut(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) isScalingOut(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, logger logr.Logger) (bool, error) {
	if !mariadb.IsReplicationEnabled() {
		return false, nil
	}
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(mariadb), &sts); err != nil {
		return false, err
	}
	return sts.Status.Replicas > 0 &&
		sts.Status.Replicas == sts.Status.ReadyReplicas &&
		sts.Status.Replicas < mariadb.Spec.Replicas, nil
}

func (r *MariaDBReconciler) reconcilePhysicalBackup(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (ctrl.Result, error) {
	key := mariadb.PhysicalBackupScaleOutKey()
	var physicalBackup mariadbv1alpha1.PhysicalBackup
	if err := r.Get(ctx, key, &physicalBackup); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.createPhysicalBackup(ctx, mariadb); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	if !physicalBackup.IsComplete() {
		logger.V(1).Info("PhysicalBackup init job not completed. Requeuing")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) createPhysicalBackup(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	replication := ptr.Deref(mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
	if replication.Replica.ReplicaBootstrapFrom == nil {
		return errors.New("replica datasource not found")
	}

	templateKey := types.NamespacedName{
		Name:      replication.Replica.ReplicaBootstrapFrom.PhysicalBackupTemplateRef.Name,
		Namespace: mariadb.Namespace,
	}
	var physicalBackupTpl mariadbv1alpha1.PhysicalBackup
	if err := r.Get(ctx, templateKey, &physicalBackupTpl); err != nil {
		return fmt.Errorf("error getting PhysicalBackup template: %v", err)
	}

	physicalBackup := mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PhysicalBackupScaleOutKey().Name,
			Namespace: mariadb.Namespace,
		},
		Spec: physicalBackupTpl.Spec,
	}
	physicalBackup.Spec.MariaDBRef = mariadbv1alpha1.MariaDBRef{
		ObjectReference: mariadbv1alpha1.ObjectReference{
			Name: mariadb.Name,
		},
	}
	physicalBackup.Spec.Schedule = &mariadbv1alpha1.PhysicalBackupSchedule{
		Immediate: ptr.To(true),
	}
	if err := controllerutil.SetControllerReference(mariadb, &physicalBackup, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to PhysicalBackup: %v", err)
	}
	return r.Create(ctx, &physicalBackup)
}

func (r *MariaDBReconciler) cleanupPhysicalBackup(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	key := mariadb.PhysicalBackupScaleOutKey()
	var physicalBackup mariadbv1alpha1.PhysicalBackup
	if err := r.Get(ctx, key, &physicalBackup); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return r.Delete(ctx, &physicalBackup)
}
