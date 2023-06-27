package statefulset

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ServiceFQDNWithService(meta metav1.ObjectMeta, service string) string {
	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		clusterName = "cluster.local"
	}
	return fmt.Sprintf("%s.%s.svc.%s", service, meta.Namespace, clusterName)
}

func ServiceFQDN(meta metav1.ObjectMeta) string {
	return ServiceFQDNWithService(meta, meta.Name)
}

func PodName(meta metav1.ObjectMeta, podIndex int) string {
	return fmt.Sprintf("%s-%d", meta.Name, podIndex)
}

func PodFQDNWithService(meta metav1.ObjectMeta, podIndex int, service string) string {
	return fmt.Sprintf("%s.%s", PodName(meta, podIndex), ServiceFQDNWithService(meta, service))
}

func PodIndex(podName string) (*int, error) {
	parts := strings.Split(podName, "-")
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid Pod name: %v", podName)
	}
	index, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return nil, fmt.Errorf("invalid Pod name: %v, error: %v", podName, err)
	}
	return &index, nil
}
