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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (b *Builder) BuildServiceMonitor(mariadb *mariadbv1alpha1.MariaDB) (*monitoringv1.ServiceMonitor, error) {
	if !mariadb.AreMetricsEnabled() {
		return nil, errors.New("MariaDB instance does not specify Metrics")
	}
	key := mariadb.MetricsKey()
	metrics := ptr.Deref(mariadb.Spec.Metrics, mariadbv1alpha1.MariadbMetrics{})
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(mariadb.Spec.InheritMetadata).
			WithReleaseLabel(metrics.ServiceMonitor.PrometheusRelease).
			Build()
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMetricsSelectorLabels(key).
			Build()
	endpoints := serviceMonitorEndpoints(
		mariadb.ObjectMeta,
		int(mariadb.Spec.Replicas),
		mariadb.InternalServiceKey().Name,
		mariadb.Spec.Port,
		withEndpointInterval(metrics.ServiceMonitor.Interval),
		withEndpointScrapeTimeout(metrics.ServiceMonitor.ScrapeTimeout),
	)

	serviceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: objMeta,
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{mariadb.Namespace},
			},
			Endpoints: endpoints,
		},
	}
	if metrics.ServiceMonitor.JobLabel != "" {
		serviceMonitor.Spec.JobLabel = metrics.ServiceMonitor.JobLabel
	}
	if err := controllerutil.SetControllerReference(mariadb, serviceMonitor, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to ServiceMonitor: %v", err)
	}
	return serviceMonitor, nil
}

func (b *Builder) BuildMaxScaleServiceMonitor(mxs *mariadbv1alpha1.MaxScale) (*monitoringv1.ServiceMonitor, error) {
	if !mxs.AreMetricsEnabled() {
		return nil, errors.New("MaxScale instance does not specify Metrics")
	}
	key := mxs.MetricsKey()
	metrics := ptr.Deref(mxs.Spec.Metrics, mariadbv1alpha1.MaxScaleMetrics{})
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(mxs.Spec.InheritMetadata).
			WithReleaseLabel(metrics.ServiceMonitor.PrometheusRelease).
			Build()
	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMetricsSelectorLabels(key).
			Build()
	endpoints := serviceMonitorEndpoints(
		mxs.ObjectMeta,
		int(mxs.Spec.Replicas),
		mxs.InternalServiceKey().Name,
		mxs.Spec.Admin.Port,
		withEndpointInterval(metrics.ServiceMonitor.Interval),
		withEndpointScrapeTimeout(metrics.ServiceMonitor.ScrapeTimeout),
	)

	serviceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: objMeta,
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{mxs.Namespace},
			},
			Endpoints: endpoints,
		},
	}
	if metrics.ServiceMonitor.JobLabel != "" {
		serviceMonitor.Spec.JobLabel = metrics.ServiceMonitor.JobLabel
	}
	if err := controllerutil.SetControllerReference(mxs, serviceMonitor, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to ServiceMonitor: %v", err)
	}
	return serviceMonitor, nil
}

type endpointOpt func(e *monitoringv1.Endpoint)

func withEndpointInterval(interval string) endpointOpt {
	return func(e *monitoringv1.Endpoint) {
		e.Interval = monitoringv1.Duration(interval)
	}
}

func withEndpointScrapeTimeout(scrapeTimeout string) endpointOpt {
	return func(e *monitoringv1.Endpoint) {
		e.ScrapeTimeout = monitoringv1.Duration(scrapeTimeout)
	}
}

func serviceMonitorEndpoints(objMeta metav1.ObjectMeta, replicas int, serviceName string, port int32,
	opts ...endpointOpt) []monitoringv1.Endpoint {
	endpoints := make([]monitoringv1.Endpoint, replicas)

	for i := 0; i < replicas; i++ {
		podName := statefulset.PodName(objMeta, i)
		podFQDN := statefulset.PodFQDNWithService(objMeta, i, serviceName)
		endpoint := monitoringv1.Endpoint{
			Path:   "/probe",
			Port:   MetricsPortName,
			Scheme: "http",
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
					fmt.Sprintf("%s:%d", podFQDN, port),
				},
			},
		}
		for _, setOpt := range opts {
			setOpt(&endpoint)
		}

		endpoints[i] = endpoint
	}
	return endpoints
}
