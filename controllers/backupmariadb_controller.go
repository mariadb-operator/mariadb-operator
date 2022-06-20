/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/builders"
)

// BackupMariaDBReconciler reconciles a BackupMariaDB object
type BackupMariaDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=database.mmontes.io,resources=backupmariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database.mmontes.io,resources=backupmariadbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database.mmontes.io,resources=backupmariadbs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *BackupMariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var backup databasev1alpha1.BackupMariaDB
	if err := r.Get(ctx, req.NamespacedName, &backup); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var pvc v1.PersistentVolumeClaim
	if err := r.Get(ctx, req.NamespacedName, &pvc); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("error getting PVC: %v", err)
		}

		if err := r.createPVC(ctx, &backup); err != nil {
			return ctrl.Result{}, fmt.Errorf("error creating PVC: %v", err)
		}
	}

	var job batchv1.Job
	if err := r.Get(ctx, req.NamespacedName, &job); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("error getting Job: %v", err)
		}

		mariadb, err := r.getMariaDB(ctx, &backup)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", err)
		}

		if err := r.createJob(ctx, &backup, mariadb); err != nil {
			return ctrl.Result{}, fmt.Errorf("error creating PVC: %v", err)
		}
	}

	if err := r.patchBackupStatus(ctx, &backup); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching BackupMariaDB status: %v", err)
	}

	return ctrl.Result{}, nil
}

func (r *BackupMariaDBReconciler) createPVC(ctx context.Context, backup *databasev1alpha1.BackupMariaDB) error {
	pvcMeta := metav1.ObjectMeta{
		Name:      backup.Name,
		Namespace: backup.Namespace,
	}
	pvc := builders.BuildPVC(pvcMeta, &backup.Spec.Storage)

	if err := controllerutil.SetControllerReference(backup, pvc, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to PVC: %v", err)
	}

	if err := r.Create(ctx, pvc); err != nil {
		return fmt.Errorf("error creating PVC on API server: %v", err)
	}
	return nil
}

func (r *BackupMariaDBReconciler) createJob(ctx context.Context, backup *databasev1alpha1.BackupMariaDB,
	mariadb *databasev1alpha1.MariaDB) error {
	job := builders.BuildJob(backup, mariadb)

	if err := controllerutil.SetControllerReference(backup, job, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to Job: %v", err)
	}

	if err := r.Create(ctx, job); err != nil {
		return fmt.Errorf("error creating PVC on API server: %v", err)
	}
	return nil
}

func (r *BackupMariaDBReconciler) patchBackupStatus(ctx context.Context, backup *databasev1alpha1.BackupMariaDB) error {
	return nil
}

func (r *BackupMariaDBReconciler) getMariaDB(ctx context.Context,
	backup *databasev1alpha1.BackupMariaDB) (*databasev1alpha1.MariaDB, error) {
	var mariadb databasev1alpha1.MariaDB
	nn := types.NamespacedName{
		Name:      backup.Spec.MariaDBRef.Name,
		Namespace: backup.Namespace,
	}
	if err := r.Get(ctx, nn, &mariadb); err != nil {
		return nil, fmt.Errorf("error getting MariaDB on API server:: %v", err)
	}
	return &mariadb, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupMariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.BackupMariaDB{}).
		Owns(&batchv1.Job{}).
		Owns(&v1.PersistentVolumeClaim{}).
		Complete(r)
}
