package builder

import (
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	labels "github.com/mmontes11/mariadb-operator/pkg/builder/labels"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (b *Builder) BuildServiceMonitor(mariaDb *mariadbv1alpha1.MariaDB, key types.NamespacedName) (*monitoringv1.ServiceMonitor, error) {
	if mariaDb.Spec.Metrics == nil {
		return nil, errors.New("MariaDB instance does not specify Metrics")
	}
	serviceMonitorLabels :=
		labels.NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariaDb.Name).
			WithRelease(mariaDb.Spec.Metrics.ServiceMonitor.PrometheusRelease).
			Build()
	serviceLabels :=
		labels.NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariaDb.Name).
			Build()
	serviceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    serviceMonitorLabels,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: serviceLabels,
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
	}
	if err := controllerutil.SetControllerReference(mariaDb, serviceMonitor, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to ServiceMonitor: %v", err)
	}
	return serviceMonitor, nil
}
