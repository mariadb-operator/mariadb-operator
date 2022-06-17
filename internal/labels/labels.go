package labels

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	App = "mariadb"
)

func GetLabels(meta metav1.ObjectMeta, extraLabels map[string]string) map[string]string {
	labels := map[string]string{
		"app.kubernetes.io/name":     App,
		"app.kubernetes.io/instance": meta.Name,
	}
	for k, v := range extraLabels {
		labels[k] = v
	}
	return labels
}
