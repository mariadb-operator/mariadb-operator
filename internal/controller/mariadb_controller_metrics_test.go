package controller

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/auth"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/deployment"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/servicemonitor"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/discovery"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileMetrics(t *testing.T) {
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
			mdb := fakeMetricsMariaDB(t, tt.serviceMonitorEnabled)
			r, c := fakeMetricsReconciler(t, resources, mdb, fakeReadyMetricsUser(mdb))

			result, err := r.reconcileMetrics(testCtx, mdb)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if requeue := !result.IsZero(); requeue != tt.wantRequeue {
				t.Errorf("requeue = %v, want %v", requeue, tt.wantRequeue)
			}

			err = c.Get(testCtx, mdb.MetricsKey(), &appsv1.Deployment{})
			if tt.wantDeployment && err != nil {
				t.Errorf("expected exporter Deployment to be created, got: %v", err)
			}
			if !tt.wantDeployment && !apierrors.IsNotFound(err) {
				t.Errorf("expected exporter Deployment not to be created, got: %v", err)
			}

			err = c.Get(testCtx, mdb.MetricsKey(), &monitoringv1.ServiceMonitor{})
			if tt.wantServiceMonitor && err != nil {
				t.Errorf("expected ServiceMonitor to be created, got: %v", err)
			}
			if !tt.wantServiceMonitor && !apierrors.IsNotFound(err) {
				t.Errorf("expected ServiceMonitor not to be created, got: %v", err)
			}
		})
	}
}

func fakeMetricsMariaDB(t *testing.T, serviceMonitorEnabled bool) *mariadbv1alpha1.MariaDB {
	t.Helper()
	mdb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{Name: "mariadb", Namespace: "test"},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Storage: mariadbv1alpha1.Storage{Size: ptr.To(resource.MustParse("1Gi"))},
			TLS:     &mariadbv1alpha1.TLS{Enabled: false},
			Metrics: &mariadbv1alpha1.MariadbMetrics{
				Enabled: true,
				ServiceMonitor: mariadbv1alpha1.ServiceMonitor{
					Enabled: ptr.To(serviceMonitorEnabled),
				},
			},
		},
	}
	if err := mdb.SetDefaults(fakeMetricsEnv); err != nil {
		t.Fatal(err)
	}
	condition.SetReadyHealthy(&mdb.Status)
	return mdb
}

func fakeReadyMetricsUser(mdb *mariadbv1alpha1.MariaDB) *mariadbv1alpha1.User {
	user := &mariadbv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: mdb.MetricsKey().Name, Namespace: mdb.Namespace},
	}
	condition.SetReadyCreated(&user.Status)
	return user
}

var fakeMetricsEnv = &environment.OperatorEnv{
	MariadbOperatorImage:  "mariadb-operator:test",
	RelatedMariadbImage:   "mariadb:test",
	RelatedExporterImage:  "exporter:test",
	MariadbDefaultVersion: "test",
}

func fakeMetricsReconciler(t *testing.T, resources []*metav1.APIResourceList,
	objs ...client.Object) (*MariaDBReconciler, client.Client) {
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
	return &MariaDBReconciler{
		Client:                   c,
		Scheme:                   scheme,
		Recorder:                 events.NewFakeRecorder(10),
		Builder:                  builder,
		RefResolver:              refresolver.New(c),
		Discovery:                disc,
		SecretReconciler:         secretReconciler,
		ServiceReconciler:        service.NewServiceReconciler(c),
		AuthReconciler:           auth.NewAuthReconciler(c, builder),
		DeploymentReconciler:     deployment.NewDeploymentReconciler(c),
		ServiceMonitorReconciler: servicemonitor.NewServiceMonitorReconciler(c),
	}, c
}
