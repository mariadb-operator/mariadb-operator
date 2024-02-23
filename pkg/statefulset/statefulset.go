package statefulset

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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

func GetStorageSize(sts *appsv1.StatefulSet, storageVolume string) *resource.Quantity {
	for _, vctpl := range sts.Spec.VolumeClaimTemplates {
		if vctpl.Name == storageVolume {
			return ptr.To(vctpl.Spec.Resources.Requests[corev1.ResourceStorage])
		}
	}
	return nil
}
