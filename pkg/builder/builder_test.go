package builder

import (
	"reflect"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/discovery"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func newTestBuilder(discovery *discovery.Discovery) *Builder {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(mariadbv1alpha1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))

	env := &environment.OperatorEnv{
		MariadbOperatorName:      "mariadb-operator",
		MariadbOperatorNamespace: "test",
		MariadbOperatorSAPath:    "/var/run/secrets/kubernetes.io/serviceaccount/token",
		MariadbOperatorImage:     "mariadb-operator:test",
		RelatedMariadbImage:      "mariadb:11.2.2:test",
		RelatedMaxscaleImage:     "maxscale:test",
		RelatedExporterImage:     "mysql-exporter:test",
		MariadbGaleraInitImage:   "mariadb-operator:test",
		MariadbGaleraAgentImage:  "mariadb-operator:test",
		MariadbGaleraLibPath:     "/usr/lib/galera/libgalera_smm.so",
		WatchNamespace:           "",
	}
	builder := NewBuilder(scheme, env, discovery)

	return builder
}

func newDefaultTestBuilder(t *testing.T) *Builder {
	discovery, err := discovery.NewDiscovery()
	if err != nil {
		t.Fatalf("unexpected error creating discovery: %v", err)
	}
	return newTestBuilder(discovery)
}

func assertObjectMeta(t *testing.T, objMeta *metav1.ObjectMeta, wantLabels, wantAnnotations map[string]string) {
	if objMeta == nil {
		t.Fatal("expecting object metadata to not be nil")
	}
	if !reflect.DeepEqual(wantLabels, objMeta.Labels) {
		t.Errorf("unexpected labels, want: %v  got: %v", wantLabels, objMeta.Labels)
	}
	if !reflect.DeepEqual(wantAnnotations, objMeta.Annotations) {
		t.Errorf("unexpected annotations, want: %v  got: %v", wantAnnotations, objMeta.Annotations)
	}
}

func assertMeta(t *testing.T, meta *mariadbv1alpha1.Metadata, wantLabels, wantAnnotations map[string]string) {
	if meta == nil {
		t.Fatal("expecting metadata to not be nil")
	}
	if !reflect.DeepEqual(wantLabels, meta.Labels) {
		t.Errorf("unexpected labels, want: %v  got: %v", wantLabels, meta.Labels)
	}
	if !reflect.DeepEqual(wantAnnotations, meta.Annotations) {
		t.Errorf("unexpected annotations, want: %v  got: %v", wantAnnotations, meta.Annotations)
	}
}
