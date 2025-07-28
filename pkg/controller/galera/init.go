package galera

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/configmap"
	mdbembed "github.com/mariadb-operator/mariadb-operator/v25/pkg/embed"
	jobpkg "github.com/mariadb-operator/mariadb-operator/v25/pkg/job"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/pvc"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *GaleraReconciler) ReconcileInit(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if mariadb.HasGaleraConfiguredCondition() || mariadb.IsGaleraInitialized() || mariadb.IsInitialized() {
		return ctrl.Result{}, nil
	}

	if !mariadb.IsGaleraInitializing() {
		pvcs, err := pvc.ListStoragePVCs(ctx, r.Client, mariadb)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error listing PVCs: %v", err)
		}
		if len(pvcs) > 0 {
			for _, p := range pvcs {
				if p.Status.Phase != corev1.ClaimBound {
					r.recorder.Eventf(mariadb, corev1.EventTypeWarning, mariadbv1alpha1.ReasonGaleraPVCNotBound,
						"Unable to init Galera cluster: PVC \"%s\" in non Bound phase", p.Name)

					log.FromContext(ctx).Info("Unable to init Galera cluster: PVC in non Bound phase. Requeuing...",
						"pvc", p.Name, "phase", p.Status.Phase)
					return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
				}
			}

			if err = r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
				condition.SetGaleraConfigured(status)
				status.GaleraRecovery = nil
				condition.SetGaleraNotReady(status)
			}); err != nil {
				return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
			}
			return ctrl.Result{}, nil
		}
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		condition.SetGaleraInitializing(status)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
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

	if !jobpkg.IsJobComplete(&initJob) {
		log.FromContext(ctx).V(1).Info("Galera init job not completed. Requeuing")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	if err := r.initCleanup(ctx, mariadb); err != nil {
		return ctrl.Result{}, fmt.Errorf("error cleaning up init resources: %v", err)
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		condition.SetGaleraInitialized(status)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *GaleraReconciler) createJob(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	job, err := r.builder.BuildGaleraInitJob(mariadb.InitKey(), mariadb)
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

	entrypointBytes, err := mdbembed.ReadEntrypoint(ctx, mariadb, r.env)
	if err != nil {
		return fmt.Errorf("error reading entrypoint: %v", err)
	}

	tpl, err := template.New("init").Parse(`#!/bin/bash

set -eo pipefail

CURDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
source "${CURDIR}/{{ .LibKey }}"

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
docker_create_db_directories

# Check if MariaDB is already initialized.
if [ -n "$DATABASE_ALREADY_EXISTS" ]; then
	mysql_note "MariaDB Server already initialized"
	exit 0
fi

run_as_mysql

docker_verify_minimum_env
docker_mariadb_init "mariadbd"
`)
	if err != nil {
		return fmt.Errorf("error building template: %v", err)
	}

	buf := new(bytes.Buffer)
	err = tpl.Execute(buf, struct {
		LibKey string
	}{
		LibKey: builder.InitLibKey,
	})
	if err != nil {
		return fmt.Errorf("error rendering template: %v", err)
	}

	req := configmap.ReconcileRequest{
		Metadata: mariadb.Spec.InheritMetadata,
		Owner:    mariadb,
		Key:      mariadb.InitKey(),
		Data: map[string]string{
			builder.InitLibKey:        string(entrypointBytes),
			builder.InitEntrypointKey: buf.String(),
		},
	}
	return r.configMapReconciler.Reconcile(ctx, &req)
}

func (r *GaleraReconciler) reconcilePVC(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	key := mariadb.PVCKey(builder.StorageVolume, 0)
	pvc, err := r.builder.BuildStoragePVC(key, mariadb.Spec.Storage.VolumeClaimTemplate, mariadb)
	if err != nil {
		return err
	}
	return r.pvcReconciler.Reconcile(ctx, key, pvc)
}

func (r *GaleraReconciler) initCleanup(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	var job batchv1.Job
	if err := r.Get(ctx, mariadb.InitKey(), &job); err == nil {
		if err := r.Delete(ctx, &job, &client.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationBackground)}); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
	}
	var cm corev1.ConfigMap
	if err := r.Get(ctx, mariadb.InitKey(), &cm); err == nil {
		if err := r.Delete(ctx, &cm); err != nil {
			return client.IgnoreNotFound(err)
		}
	}
	return nil
}
