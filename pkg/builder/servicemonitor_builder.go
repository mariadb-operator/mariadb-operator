package builder

import (
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (b *Builder) BuildServiceMonitor(mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName) (*monitoringv1.ServiceMonitor, error) {
	if !mariadb.AreMetricsEnabled() {
		return nil, errors.New("MariaDB instance does not specify Metrics")
	}
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMetricsSelectorLabels(mariadb).
			Build()
	serviceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: serviceMonitorObjectMeta(mariadb, key),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{mariadb.Namespace},
			},
			Endpoints: serviceMonitorEndpoints(mariadb),
		},
	}
	if err := controllerutil.SetControllerReference(mariadb, serviceMonitor, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to ServiceMonitor: %v", err)
	}
	return serviceMonitor, nil
}

func serviceMonitorObjectMeta(mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName) metav1.ObjectMeta {
	metaBuilder :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb)
	if mariadb.Spec.Metrics.ServiceMonitor.PrometheusRelease != "" {
		metaBuilder =
			metaBuilder.WithLabels(map[string]string{
				"release": mariadb.Spec.Metrics.ServiceMonitor.PrometheusRelease,
			})
	}
	return metaBuilder.Build()
}

func serviceMonitorEndpoints(mariadb *mariadbv1alpha1.MariaDB) []monitoringv1.Endpoint {
	endpoints := make([]monitoringv1.Endpoint, mariadb.Spec.Replicas)
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		podName := statefulset.PodName(mariadb.ObjectMeta, i)
		podFQDN := statefulset.PodFQDNWithService(mariadb.ObjectMeta, i, mariadb.InternalServiceKey().Name)
		endpoints = append(endpoints, monitoringv1.Endpoint{
			Path:          "/probe",
			Port:          MetricsPortName,
			Scheme:        "http",
			Interval:      monitoringv1.Duration(mariadb.Spec.Metrics.ServiceMonitor.Interval),
			ScrapeTimeout: monitoringv1.Duration(mariadb.Spec.Metrics.ServiceMonitor.ScrapeTimeout),
			MetricRelabelConfigs: []*monitoringv1.RelabelConfig{
				{
					Action:      "replace",
					Replacement: podFQDN,
					SourceLabels: []monitoringv1.LabelName{
						monitoringv1.LabelName("instance"),
					},
					TargetLabel: "instance",
				},
				{
					Action:      "replace",
					Replacement: podName,
					SourceLabels: []monitoringv1.LabelName{
						monitoringv1.LabelName("target"),
					},
					TargetLabel: "target",
				},
			},
			Params: map[string][]string{
				"target": {
					fmt.Sprintf("%s:%d", podFQDN, mariadb.Spec.Port),
				},
			},
		})
	}
	return endpoints
}
