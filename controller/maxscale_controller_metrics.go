package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MaxScaleReconciler) reconcileMetrics(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	if !req.mxs.AreMetricsEnabled() {
		return ctrl.Result{}, nil
	}

	exist, err := r.DiscoveryClient.ServiceMonitorExist()
	if err != nil {
		return ctrl.Result{}, err
	}
	if !exist {
		r.Recorder.Event(req.mxs, corev1.EventTypeWarning, mariadbv1alpha1.ReasonCRDNotFound,
			"Unable to reconcile metrics: ServiceMonitor CRD not installed in the cluster")
		log.FromContext(ctx).Error(errors.New("ServiceMonitor CRD not installed in the cluster"), "Unable to reconcile metrics")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if !req.mxs.IsReady() {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	mdb, err := r.RefResolver.MariaDB(ctx, req.mxs.Spec.MariaDBRef, req.mxs.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mdb)
	}

	if err := r.reconcileExporterConfig(ctx, req); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileExporterDeployment(ctx, req.mxs, mdb); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileExporterService(ctx, req.mxs); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileServiceMonitor(ctx, req.mxs, mdb); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *MaxScaleReconciler) reconcileExporterConfig(ctx context.Context, req *requestMaxScale) error {
	secretKeyRef := req.mxs.MetricsConfigSecretKeyRef()
	key := types.NamespacedName{
		Name:      secretKeyRef.Name,
		Namespace: req.mxs.Namespace,
	}
	var existingSecret corev1.Secret
	if err := r.Get(ctx, key, &existingSecret); err == nil {
		return nil
	}

	password, err := r.RefResolver.SecretKeyRef(ctx, req.mxs.Spec.Auth.MetricsPasswordSecretKeyRef, req.mxs.Namespace)
	if err != nil {
		return fmt.Errorf("error getting metrics password Secret: %v", err)
	}

	type tplOpts struct {
		User     string
		Password string
	}
	tpl := createTpl(secretKeyRef.Key, `[maxscale_exporter]
maxscale_username={{ .User }}
maxscale_password={{ .Password }}`)
	buf := new(bytes.Buffer)
	err = tpl.Execute(buf, tplOpts{
		User:     req.mxs.Spec.Auth.MetricsUsername,
		Password: password,
	})
	if err != nil {
		return fmt.Errorf("error rendering exporter config: %v", err)
	}

	secretOpts := builder.SecretOpts{
		Key: key,
		Data: map[string][]byte{
			secretKeyRef.Key: buf.Bytes(),
		},
	}
	secret, err := r.Builder.BuildSecret(secretOpts, req.mxs)
	if err != nil {
		return fmt.Errorf("error building exporter config Secret: %v", err)
	}
	return r.Create(ctx, secret)
}

func (r *MaxScaleReconciler) reconcileExporterDeployment(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	mariadb *mariadbv1alpha1.MariaDB) error {
	desiredDeploy, err := r.Builder.BuildMaxScaleExporterDeployment(mxs, mariadb)
	if err != nil {
		return fmt.Errorf("error building exporter Deployment: %v", err)
	}
	return r.DeploymentReconciler.Reconcile(ctx, desiredDeploy)
}

func (r *MaxScaleReconciler) reconcileExporterService(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) error {
	key := mxs.MetricsKey()
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
		Ports: []corev1.ServicePort{
			{
				Name: builder.MetricsPortName,
				Port: mxs.Spec.Metrics.Exporter.Port,
			},
		},
		SelectorLabels: selectorLabels,
		ExtraMeta:      nil,
	}

	desiredSvc, err := r.Builder.BuildService(key, mxs, opts)
	if err != nil {
		return fmt.Errorf("error building exporter Service: %v", err)
	}
	return r.ServiceReconciler.Reconcile(ctx, desiredSvc)
}

func (r *MaxScaleReconciler) reconcileServiceMonitor(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	mariadb *mariadbv1alpha1.MariaDB) error {
	desiredSvcMonitor, err := r.Builder.BuildMaxScaleServiceMonitor(mxs, mariadb)
	if err != nil {
		return fmt.Errorf("error building Service Monitor: %v", err)
	}
	return r.ServiceMonitorReconciler.Reconcile(ctx, desiredSvcMonitor)
}
