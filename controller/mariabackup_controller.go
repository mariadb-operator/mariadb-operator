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

package controller

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/batch"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MariaBackupReconciler reconciles a MariaBackup object
type MariaBackupReconciler struct {
	client.Client
	RESTClient        rest.Interface
	RESTConfig        *rest.Config
	Scheme            *runtime.Scheme
	Builder           *builder.Builder
	RefResolver       *refresolver.RefResolver
	ConditionComplete *condition.Complete
	BatchReconciler   *batch.BatchReconciler
}

//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=mariabackups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=mariabackups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=mariabackups/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *MariaBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var backup mariadbv1alpha1.MariaBackup
	if err := r.Get(ctx, req.NamespacedName, &backup); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	mariaDb, err := r.RefResolver.MariaDB(ctx, &backup.Spec.MariaDBRef, backup.Namespace)
	if err != nil {
		var mariaDbErr *multierror.Error
		mariaDbErr = multierror.Append(mariaDbErr, err)

		err = r.patchStatus(ctx, &backup, r.ConditionComplete.PatcherRefResolver(err, mariaDb))
		mariaDbErr = multierror.Append(mariaDbErr, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mariaDbErr)
	}

	if backup.Spec.MariaDBRef.WaitForIt && !mariaDb.IsReady() {
		if err := r.patchStatus(ctx, &backup, r.ConditionComplete.PatcherFailed("MariaDB not ready")); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching Backup: %v", err)
		}
		return ctrl.Result{}, errors.New("MariaDB not ready")
	}

	var batchErr *multierror.Error
	err = r.BatchReconciler.Reconcile(ctx, &backup, mariaDb)
	batchErr = multierror.Append(batchErr, err)

	patcher, err := r.patcher(ctx, err, req.NamespacedName, &backup)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		return ctrl.Result{}, fmt.Errorf("error getting patcher for Backup: %v", err)
	}

	err = r.patchStatus(ctx, &backup, patcher)
	batchErr = multierror.Append(batchErr, err)

	if err := batchErr.ErrorOrNil(); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating Job: %v", err)
	}

	podList := &corev1.PodList{}

	// Create a label selector to filter pods with a specific label
	labelSelector := labels.SelectorFromSet(labels.Set(map[string]string{
		"backup-ref": backup.Name,
	}))

	// Create a ListOptions with the label selector
	listOptions := &client.ListOptions{
		LabelSelector: labelSelector,
	}

	max_attempts := 0
	for len(podList.Items) == 0 && max_attempts < 10 {
		// List pods in the specified namespace with the label selector
		err = r.Client.List(ctx, podList, listOptions)
		if err != nil {
			// Handle the error
			return ctrl.Result{}, fmt.Errorf("error getting job: %v", err)
		}
		time.Sleep(3 * time.Second)
		max_attempts++
	}

	var podIP string = ""
	// Iterate through the list of pods

	for _, pod := range podList.Items {

		if pod.Status.Phase == "Running" {
			podIP = pod.Status.PodIP
			break
		} else if pod.Status.Phase == "Succeeded" {
			podIP = ""
			break
		}
	}

	if podIP != "" {
		// Start mariadb-backup on the primary pod
		// TODO: make the backup source configurable primary or replicas
		// TODO: add s3 configuration parameters to get backup to s3 directly
		cmd := []string{
			"/bin/sh",
			"-c",
			fmt.Sprintf("mariadb-backup --backup --user=root --password=${MARIADB_ROOT_PASSWORD} --compress --stream=mbstream --parallel 1 --target-dir=/tmp | socat -u stdio TCP:%s:4444 &", podIP),
		}

		execReq := r.RESTClient.
			Post().
			Namespace(mariaDb.Namespace).
			Resource("pods").
			Name(*mariaDb.Status.CurrentPrimary).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Container: "mariadb",
				Command:   cmd,
				Stdin:     true,
				Stdout:    true,
				Stderr:    true,
			}, runtime.NewParameterCodec(r.Scheme))

		exec, err := remotecommand.NewSPDYExecutor(r.RESTConfig, "POST", execReq.URL())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error while creating remote command executor: %v", err)
		}

		err = exec.Stream(remotecommand.StreamOptions{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
			Tty:    false,
		})

		if err != nil {
			// Handle the error
			return ctrl.Result{}, fmt.Errorf("error executing script in pod: %v", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *MariaBackupReconciler) patcher(ctx context.Context, err error,
	key types.NamespacedName, backup *mariadbv1alpha1.MariaBackup) (condition.Patcher, error) {

	if backup.Spec.Schedule != nil {
		return r.ConditionComplete.PatcherWithCronJob(ctx, err, key)
	}
	return r.ConditionComplete.PatcherWithJob(ctx, err, key)
}

func (r *MariaBackupReconciler) patchStatus(ctx context.Context, backup *mariadbv1alpha1.MariaBackup,
	patcher condition.Patcher) error {
	patch := client.MergeFrom(backup.DeepCopy())
	patcher(&backup.Status)

	if err := r.Client.Status().Patch(ctx, backup, patch); err != nil {
		return fmt.Errorf("error patching Backup status: %v", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MariaBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.MariaBackup{}).
		Owns(&batchv1.CronJob{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
