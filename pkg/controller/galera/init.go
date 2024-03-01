package galera

import (
	"context"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *GaleraReconciler) ReconcileInit(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if mariadb.HasGaleraConfiguredCondition() {
		return ctrl.Result{}, nil
	}

	if err := r.reconcilePVC(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileConfigMap(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}

	var initJob batchv1.Job
	if err := r.Get(ctx, mariadb.InitKey(), &initJob); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.createJob(ctx, mariadb); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	if !isJobComplete(&initJob) {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func (r *GaleraReconciler) createJob(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	job, err := r.builder.BuilInitJob(mariadb.InitKey(), mariadb)
	if err != nil {
		return err
	}
	return r.Create(ctx, job)
}

func (r *GaleraReconciler) reconcileConfigMap(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	key := mariadb.InitKey()

	var cm corev1.ConfigMap
	err := r.Get(ctx, key, &cm)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	req := configmap.ReconcileRequest{
		Mariadb: mariadb,
		Owner:   mariadb,
		Key:     mariadb.InitKey(),
		Data: map[string]string{
			// Source: https://github.com/MariaDB/mariadb-docker/blob/master/docker-entrypoint.sh
			builder.InitConfigKey: `#!/bin/bash

source /usr/local/bin/docker-entrypoint.sh

# If container is started as root user, restart as dedicated mysql user
function run_as_mysql() {
	if [ "$(id -u)" = "0" ]; then
		mysql_note "Switching to dedicated user 'mysql'"
		exec gosu mysql "$0" "$@"
	fi
}

mysql_note "Entrypoint script for MariaDB Server ${MARIADB_VERSION} started"
mysql_check_config "mariadbd"

# Load various environment variables
docker_setup_env "mariadbd"

# Check if MariaDB is already initialized.
if [ -n "$DATABASE_ALREADY_EXISTS" ]; then
	run_as_mysql

	mysql_note "Starting temporary server"
	docker_temp_server_start "mariadbd"
	mysql_note "Temporary server started."

	if mariadb -u root -p"${MARIADB_ROOT_PASSWORD}" -e "SELECT 1;" &> /dev/null; then
		mysql_note "MariaDB is already initialized. Skipping initialization."
		
		mysql_note "Stopping temporary server"
		docker_temp_server_stop
		mysql_note "Temporary server stopped"
		exit 0
	fi

	mysql_warn "This MariaDB instance has already been initialized but the root password Secret does not match the existing state."
	mysql_warn "Please either update the root password Secret or delete the PVC."
	mysql_note "Stopping temporary server"
	docker_temp_server_stop
	mysql_note "Temporary server stopped"
	exit 1
fi

docker_create_db_directories

run_as_mysql

docker_verify_minimum_env
docker_mariadb_init "mariadbd"`,
		},
	}
	return r.configMapReconciler.Reconcile(ctx, &req)
}

func (r *GaleraReconciler) reconcilePVC(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	key := mariadb.PVCKey(builder.StorageVolume, 0)

	var existingPVC corev1.PersistentVolumeClaim
	err := r.Get(ctx, key, &existingPVC)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	pvc, err := r.builder.BuildStoragePVC(key, mariadb.Spec.Storage.VolumeClaimTemplate, mariadb)
	if err != nil {
		return err
	}
	return r.Create(ctx, pvc)
}

func (r *GaleraReconciler) deleteInitJob(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	var job batchv1.Job
	if err := r.Get(ctx, mariadb.InitKey(), &job); err != nil {
		return client.IgnoreNotFound(err)
	}
	if err := r.Delete(ctx, &job, &client.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationBackground)}); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
}

func isJobComplete(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
