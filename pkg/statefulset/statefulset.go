package statefulset

import (
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ServiceFQDN(meta metav1.ObjectMeta) string {
	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		clusterName = "cluster.local"
	}
	return fmt.Sprintf("%s.%s.svc.%s", meta.Name, meta.Namespace, clusterName)
}

func PodFQDN(meta metav1.ObjectMeta, podIndex int) string {
	return fmt.Sprintf("%s-%d.%s", meta.Name, podIndex, ServiceFQDN(meta))
}
