package builders

import (
	"errors"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func BuildServiceMonitor(mariaDb *databasev1alpha1.MariaDB, key types.NamespacedName) (*monitoringv1.ServiceMonitor, error) {
	if mariaDb.Spec.Metrics == nil {
		return nil, errors.New("MariaDB instance does not specify Metrics")
	}
	labels :=
		NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariaDb.Name).
			WithRelease(mariaDb.Spec.Metrics.ServiceMonitor.PrometheusRelease).
			Build()
	serviceMonitorLabels := getServiceMonitorLabels(mariaDb)

	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    labels,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: serviceMonitorLabels,
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{mariaDb.Namespace},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port:          metricsPortName,
					Path:          "/metrics",
					Scheme:        "http",
					Interval:      monitoringv1.Duration(mariaDb.Spec.Metrics.ServiceMonitor.Interval),
					ScrapeTimeout: monitoringv1.Duration(mariaDb.Spec.Metrics.ServiceMonitor.ScrapeTimeout),
				},
			},
		},
	}, nil
}

func getServiceMonitorLabels(mariadb *databasev1alpha1.MariaDB) map[string]string {
	return NewLabelsBuilder().
		WithApp(appMariaDb).
		WithInstance(mariadb.Name).
		Build()
}
