package builder

import (
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (b *Builder) BuildServiceMonitor(mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName) (*monitoringv1.ServiceMonitor, error) {
	if mariadb.Spec.Metrics == nil {
		return nil, errors.New("MariaDB instance does not specify Metrics")
	}
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			WithLabels(map[string]string{
				"release": mariadb.Spec.Metrics.ServiceMonitor.PrometheusRelease,
			}).
			Build()
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMariaDBSelectorLabels(mariadb).
			Build()
	serviceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: objMeta,
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{mariadb.Namespace},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port:          metricsPortName,
					Path:          "/metrics",
					Scheme:        "http",
					Interval:      monitoringv1.Duration(mariadb.Spec.Metrics.ServiceMonitor.Interval),
					ScrapeTimeout: monitoringv1.Duration(mariadb.Spec.Metrics.ServiceMonitor.ScrapeTimeout),
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(mariadb, serviceMonitor, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to ServiceMonitor: %v", err)
	}
	return serviceMonitor, nil
}
