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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	exist, err := r.DiscoveryClient.ServiceMonitorExist()
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
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	if err := r.reconcileMetricsPassword(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileMetricsUser(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	if result, err := r.reconcileMetricsGrant(ctx, mariadb); !result.IsZero() || err != nil {
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
	key := types.NamespacedName{
		Name:      secretKeyRef.Name,
		Namespace: mariadb.Namespace,
	}
	_, err := r.SecretReconciler.ReconcileRandomPassword(ctx, key, secretKeyRef.Key, mariadb)
	return err
}

func (r *MariaDBReconciler) reconcileMetricsUser(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	key := mariadb.MetricsKey()
	var existingUser mariadbv1alpha1.User
	if err := r.Get(ctx, key, &existingUser); err == nil {
		return nil
	}

	opts := builder.UserOpts{
		Key:                  key,
		PasswordSecretKeyRef: mariadb.Spec.Metrics.PasswordSecretKeyRef,
		MaxUserConnections:   3,
		Name:                 mariadb.Spec.Metrics.Username,
	}
	user, err := r.Builder.BuildUser(mariadb, opts)
	if err != nil {
		return fmt.Errorf("error building User: %v", err)
	}
	return r.Create(ctx, user)
}

func (r *MariaDBReconciler) reconcileMetricsGrant(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	key := mariadb.MetricsKey()

	var user mariadbv1alpha1.User
	if err := r.Get(ctx, key, &user); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
		return ctrl.Result{}, fmt.Errorf("error getting metrics User: %v", err)
	}
	if !user.IsReady() {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	var existingGrant mariadbv1alpha1.Grant
	if err := r.Get(ctx, key, &existingGrant); err == nil {
		return ctrl.Result{}, nil
	}

	opts := builder.GrantOpts{
		Key:         key,
		Privileges:  exporterPrivileges,
		Database:    "*",
		Table:       "*",
		Username:    mariadb.Spec.Metrics.Username,
		GrantOption: false,
	}
	grant, err := r.Builder.BuildGrant(mariadb, opts)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building metrics Grant: %v", err)
	}
	return ctrl.Result{}, r.Create(ctx, grant)
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

	passwordSecretKeyRef := mariadb.Spec.Metrics.PasswordSecretKeyRef
	passwordSecretKey := types.NamespacedName{
		Name:      passwordSecretKeyRef.Name,
		Namespace: mariadb.Namespace,
	}
	var passwordSecret corev1.Secret
	if err := r.Get(ctx, passwordSecretKey, &passwordSecret); err != nil {
		return fmt.Errorf("error getting metrics password Secret: %v", err)
	}

	type tplOpts struct {
		User     string
		Password string
	}
	tpl := createTpl(secretKeyRef.Key, `[client]
user = {{ .User }}
password = {{ .Password }}`)
	buf := new(bytes.Buffer)
	err := tpl.Execute(buf, tplOpts{
		User:     mariadb.Spec.Metrics.Username,
		Password: string(passwordSecret.Data[passwordSecretKeyRef.Key]),
	})
	if err != nil {
		return fmt.Errorf("error rendering exporter config: %v", err)
	}

	secretOpts := builder.SecretOpts{
		MariaDB: mariadb,
		Key:     key,
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
	key := mariadb.MetricsKey()
	desiredDeploy, err := r.Builder.BuildExporterDeployment(mariadb, key)
	if err != nil {
		return fmt.Errorf("error building exporter Deployment: %v", err)
	}
	return r.DeploymentReconciler.Reconcile(ctx, desiredDeploy)
}

func (r *MariaDBReconciler) reconcileExporterService(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	key := mariadb.MetricsKey()
	metricsSelectorLabels :=
		labels.NewLabelsBuilder().
			WithMetricsSelectorLabels(mariadb).
			Build()
	opts := builder.ServiceOpts{
		ServiceTemplate: mariadbv1alpha1.ServiceTemplate{
			Labels: metricsSelectorLabels,
		},
		Selectorlabels: metricsSelectorLabels,
		Ports: []corev1.ServicePort{
			{
				Name: builder.MetricsPortName,
				Port: mariadb.Spec.Metrics.Exporter.Port,
			},
		},
	}
	desiredSvc, err := r.Builder.BuildService(mariadb, key, opts)
	if err != nil {
		return fmt.Errorf("error building exporter Service: %v", err)
	}
	return r.ServiceReconciler.Reconcile(ctx, desiredSvc)
}

func (r *MariaDBReconciler) reconcileServiceMonitor(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	key := mariadb.MetricsKey()
	desiredSvcMonitor, err := r.Builder.BuildServiceMonitor(mariadb, key)
	if err != nil {
		return fmt.Errorf("error building Service Monitor: %v", err)
	}
	return r.ServiceMonitorReconciler.Reconcile(ctx, desiredSvcMonitor)
}

func createTpl(name, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}
