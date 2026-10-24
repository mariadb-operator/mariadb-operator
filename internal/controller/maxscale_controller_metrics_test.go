package controller

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/deployment"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/servicemonitor"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/discovery"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileMaxScaleMetrics(t *testing.T) {
	serviceMonitorCRD := &metav1.APIResourceList{
		GroupVersion: "monitoring.coreos.com/v1",
		APIResources: []metav1.APIResource{
			{
				Name: "servicemonitors",
			},
		},
	}

	tests := []struct {
		name                  string
		serviceMonitorEnabled bool
		serviceMonitorCRD     bool
		wantRequeue           bool
		wantDeployment        bool
		wantServiceMonitor    bool
	}{
		{
			name:                  "ServiceMonitor enabled and CRD available",
			serviceMonitorEnabled: true,
			serviceMonitorCRD:     true,
			wantDeployment:        true,
			wantServiceMonitor:    true,
		},
		{
			name:                  "ServiceMonitor enabled and CRD not available",
			serviceMonitorEnabled: true,
			serviceMonitorCRD:     false,
			wantRequeue:           true,
			wantDeployment:        true,
			wantServiceMonitor:    false,
		},
		{
			name:                  "ServiceMonitor disabled",
			serviceMonitorEnabled: false,
			serviceMonitorCRD:     true,
			wantDeployment:        true,
			wantServiceMonitor:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resources []*metav1.APIResourceList
			if tt.serviceMonitorCRD {
				resources = append(resources, serviceMonitorCRD)
			}
			mxs := fakeMetricsMaxScale(tt.serviceMonitorEnabled)
			r, c := fakeMaxScaleMetricsReconciler(t, resources, mxs, fakeMaxScaleMetricsPassword(mxs))

			result, err := r.reconcileMetrics(testCtx, &requestMaxScale{mxs: mxs})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if requeue := !result.IsZero(); requeue != tt.wantRequeue {
				t.Errorf("requeue = %v, want %v", requeue, tt.wantRequeue)
			}

			err = c.Get(testCtx, mxs.MetricsKey(), &appsv1.Deployment{})
			if tt.wantDeployment && err != nil {
				t.Errorf("expected exporter Deployment to be created, got: %v", err)
			}
			if !tt.wantDeployment && !apierrors.IsNotFound(err) {
				t.Errorf("expected exporter Deployment not to be created, got: %v", err)
			}

			err = c.Get(testCtx, mxs.MetricsKey(), &monitoringv1.ServiceMonitor{})
			if tt.wantServiceMonitor && err != nil {
				t.Errorf("expected ServiceMonitor to be created, got: %v", err)
			}
			if !tt.wantServiceMonitor && !apierrors.IsNotFound(err) {
				t.Errorf("expected ServiceMonitor not to be created, got: %v", err)
			}
		})
	}
}

func fakeMetricsMaxScale(serviceMonitorEnabled bool) *mariadbv1alpha1.MaxScale {
	mxs := &mariadbv1alpha1.MaxScale{
		ObjectMeta: metav1.ObjectMeta{Name: "maxscale", Namespace: "test"},
		Spec: mariadbv1alpha1.MaxScaleSpec{
			Metrics: &mariadbv1alpha1.MaxScaleMetrics{
				Enabled: true,
				ServiceMonitor: mariadbv1alpha1.ServiceMonitor{
					Enabled: ptr.To(serviceMonitorEnabled),
				},
			},
		},
	}
	mxs.SetDefaults(fakeMetricsEnv, nil)
	condition.SetReadyHealthy(&mxs.Status)
	return mxs
}

func fakeMaxScaleMetricsPassword(mxs *mariadbv1alpha1.MaxScale) *corev1.Secret {
	secretKeyRef := mxs.Spec.Auth.MetricsPasswordSecretKeyRef
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretKeyRef.Name, Namespace: mxs.Namespace},
		Data: map[string][]byte{
			secretKeyRef.Key: []byte("MaxScale11!"),
		},
	}
}

func fakeMaxScaleMetricsReconciler(t *testing.T, resources []*metav1.APIResourceList,
	objs ...client.Object) (*MaxScaleReconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(mariadbv1alpha1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	disc, err := discovery.NewFakeDiscovery(resources...)
	if err != nil {
		t.Fatal(err)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	builder := builder.NewBuilder(scheme, fakeMetricsEnv, disc)
	secretReconciler, err := secret.NewSecretReconciler(c, builder)
	if err != nil {
		t.Fatal(err)
	}
	return &MaxScaleReconciler{
		Client:                   c,
		Scheme:                   scheme,
		Recorder:                 events.NewFakeRecorder(10),
		Builder:                  builder,
		RefResolver:              refresolver.New(c),
		Discovery:                disc,
		SecretReconciler:         secretReconciler,
		ServiceReconciler:        service.NewServiceReconciler(c),
		DeploymentReconciler:     deployment.NewDeploymentReconciler(c),
		ServiceMonitorReconciler: servicemonitor.NewServiceMonitorReconciler(c),
	}, c
}
