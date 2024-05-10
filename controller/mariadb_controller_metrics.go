package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"text/template"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/auth"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var exporterPrivileges = []string{
	"SELECT",
	"PROCESS",
	"REPLICATION CLIENT",
	"REPLICA MONITOR",
	"SLAVE MONITOR",
}

func (r *MariaDBReconciler) reconcileMetrics(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !mariadb.AreMetricsEnabled() {
		return ctrl.Result{}, nil
	}

	exist, err := r.Discovery.ServiceMonitorExist()
	if err != nil {
		return ctrl.Result{}, err
	}
	if !exist {
		r.Recorder.Event(mariadb, corev1.EventTypeWarning, mariadbv1alpha1.ReasonCRDNotFound,
			"Unable to reconcile metrics: ServiceMonitor CRD not installed in the cluster")
		log.FromContext(ctx).Error(errors.New("ServiceMonitor CRD not installed in the cluster"), "Unable to reconcile metrics")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if !mariadb.IsReady() {
		log.FromContext(ctx).V(1).Info("MariaDB not ready. Requeuing metrics")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	if err := r.reconcileMetricsPassword(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	if result, err := r.reconcileAuth(ctx, mariadb); !result.IsZero() || err != nil {
		return result, err
	}
	if err := r.reconcileExporterConfig(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileExporterDeployment(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileExporterService(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileServiceMonitor(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcileMetricsPassword(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	secretKeyRef := mariadb.Spec.Metrics.PasswordSecretKeyRef
	req := secret.PasswordRequest{
		Owner:    mariadb,
		Metadata: mariadb.Spec.InheritMetadata,
		Key: types.NamespacedName{
			Name:      secretKeyRef.Name,
			Namespace: mariadb.Namespace,
		},
		SecretKey: secretKeyRef.Key,
		Generate:  secretKeyRef.Generate,
	}
	_, err := r.SecretReconciler.ReconcilePassword(ctx, req)
	return err
}

func (r *MariaDBReconciler) reconcileAuth(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	key := mariadb.MetricsKey()
	ref := mariadbv1alpha1.MariaDBRef{
		ObjectReference: corev1.ObjectReference{
			Name:      mariadb.Name,
			Namespace: mariadb.Namespace,
		},
	}
	userOpts := builder.UserOpts{
		Name:                 mariadb.Spec.Metrics.Username,
		PasswordSecretKeyRef: &mariadb.Spec.Metrics.PasswordSecretKeyRef.SecretKeySelector,
		MaxUserConnections:   3,
		Metadata:             mariadb.Spec.InheritMetadata,
		MariaDBRef:           ref,
	}
	grantOpts := auth.GrantOpts{
		GrantOpts: builder.GrantOpts{
			Privileges:  exporterPrivileges,
			Database:    "*",
			Table:       "*",
			Username:    mariadb.Spec.Metrics.Username,
			GrantOption: false,
			Metadata:    mariadb.Spec.InheritMetadata,
			MariaDBRef:  ref,
		},
		Key: key,
	}
	return r.AuthReconciler.ReconcileUserGrant(ctx, key, mariadb, userOpts, grantOpts)
}

func (r *MariaDBReconciler) reconcileExporterConfig(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	secretKeyRef := mariadb.MetricsConfigSecretKeyRef()
	key := types.NamespacedName{
		Name:      secretKeyRef.Name,
		Namespace: mariadb.Namespace,
	}
	var existingSecret corev1.Secret
	if err := r.Get(ctx, key, &existingSecret); err == nil {
		return nil
	}

	password, err := r.RefResolver.SecretKeyRef(ctx, mariadb.Spec.Metrics.PasswordSecretKeyRef.SecretKeySelector, mariadb.Namespace)
	if err != nil {
		return fmt.Errorf("error getting metrics password Secret: %v", err)
	}

	type tplOpts struct {
		User     string
		Password string
	}
	tpl := createTpl(secretKeyRef.Key, `[client]
user={{ .User }}
password={{ .Password }}`)
	buf := new(bytes.Buffer)
	err = tpl.Execute(buf, tplOpts{
		User:     mariadb.Spec.Metrics.Username,
		Password: password,
	})
	if err != nil {
		return fmt.Errorf("error rendering exporter config: %v", err)
	}

	secretOpts := builder.SecretOpts{
		Metadata: []*mariadbv1alpha1.Metadata{mariadb.Spec.InheritMetadata},
		Key:      key,
		Data: map[string][]byte{
			secretKeyRef.Key: buf.Bytes(),
		},
	}
	secret, err := r.Builder.BuildSecret(secretOpts, mariadb)
	if err != nil {
		return fmt.Errorf("error building exporter config Secret: %v", err)
	}
	return r.Create(ctx, secret)
}

func (r *MariaDBReconciler) reconcileExporterDeployment(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	desiredDeploy, err := r.Builder.BuildExporterDeployment(mariadb)
	if err != nil {
		return fmt.Errorf("error building exporter Deployment: %v", err)
	}
	return r.DeploymentReconciler.Reconcile(ctx, desiredDeploy)
}

func (r *MariaDBReconciler) reconcileExporterService(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	key := mariadb.MetricsKey()
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMetricsSelectorLabels(key).
			Build()
	opts := builder.ServiceOpts{
		ServiceTemplate: mariadbv1alpha1.ServiceTemplate{
			Metadata: &mariadbv1alpha1.Metadata{
				Labels: selectorLabels,
			},
		},
		ExtraMeta: mariadb.Spec.InheritMetadata,
		Ports: []corev1.ServicePort{
			{
				Name: builder.MetricsPortName,
				Port: mariadb.Spec.Metrics.Exporter.Port,
			},
		},
		SelectorLabels: selectorLabels,
	}

	desiredSvc, err := r.Builder.BuildService(key, mariadb, opts)
	if err != nil {
		return fmt.Errorf("error building exporter Service: %v", err)
	}
	return r.ServiceReconciler.Reconcile(ctx, desiredSvc)
}

func (r *MariaDBReconciler) reconcileServiceMonitor(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	desiredSvcMonitor, err := r.Builder.BuildServiceMonitor(mariadb)
	if err != nil {
		return fmt.Errorf("error building Service Monitor: %v", err)
	}
	return r.ServiceMonitorReconciler.Reconcile(ctx, desiredSvcMonitor)
}

func createTpl(name, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}
